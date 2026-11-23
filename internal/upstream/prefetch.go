package upstream

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	"github.com/invisi-labs/invisi-explorer/internal/wire"
)

type protocolStatus struct {
	IndexedHead *Block `json:"indexedHead"`
	Finality    struct {
		Latest    Head `json:"latest"`
		Safe      Head `json:"safe"`
		Finalized Head `json:"finalized"`
	} `json:"finality"`
}
type historicalWindow struct {
	from, to   uint64
	anchor     model.Anchor
	operations []model.Operation
	complete   bool
	err        error
}

// CatchUp overlaps four complete finalized windows. Near-head scans and saved
// dense-page cursors retain Tick's durable sequential path. Only this goroutine
// writes staged/canonical data; fetch workers never publish independently.
func (s *Syncer) CatchUp(ctx context.Context, c config.Chain) error {
	return s.catchUp(ctx, c, 30*time.Second)
}

func (s *Syncer) catchUp(ctx context.Context, c config.Chain, timeout time.Duration) (syncErr error) {
	scope := Scope(c)
	observation := map[string]any{}
	var previous string
	if s.Store.DB.QueryRowContext(ctx, "SELECT payload FROM source_status WHERE scope=? AND source='protocol'", scope.Key()).Scan(&previous) == nil {
		_ = json.Unmarshal([]byte(previous), &observation)
	}
	record := true
	defer func() {
		if record {
			s.recordSyncResult(ctx, scope, observation, syncErr)
		}
	}()
	sequential := func() error { record = false; return s.Tick(ctx, c) }
	var number, hash string
	if e := s.Store.DB.QueryRowContext(ctx, "SELECT block_number,block_hash FROM chain_checkpoints WHERE id=?", scope.Key()).Scan(&number, &hash); e != nil {
		if errors.Is(e, sql.ErrNoRows) {
			return sequential()
		}
		return e
	}
	var pending int
	if e := s.Store.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_windows WHERE scope=?", scope.Key()).Scan(&pending); e != nil {
		return e
	}
	if pending != 0 {
		return sequential()
	}
	height, e := strconv.ParseUint(number, 10, 64)
	if e != nil {
		return e
	}
	if _, e = s.Deployment(ctx, c); e != nil {
		return e
	}
	var status protocolStatus
	if e = Call(ctx, s.Client, c.RPCURL, "invisi_getStatus", []any{}, &status); e != nil {
		return e
	}
	fin := status.Finality.Finalized
	if status.IndexedHead == nil || fin.Status != "available" || fin.Block == nil {
		return sequential()
	}
	end := min(status.IndexedHead.Number, fin.Block.Number)
	if end <= height || end-height < 256 {
		return sequential()
	}
	observation["rpcIndexedHead"] = publicAnchor(status.IndexedHead)
	observation["chainHead"] = publicAnchor(status.Finality.Latest.Block)
	observation["finality"] = map[string]any{"latest": publicHead(status.Finality.Latest), "safe": publicHead(status.Finality.Safe), "finalized": publicHead(fin)}
	actual, e := s.Header(ctx, c, height)
	if e != nil {
		return e
	}
	if actual != strings.ToLower(hash) {
		return sequential()
	}
	liveFinalized, e := s.Header(ctx, c, fin.Block.Number)
	if e != nil {
		return e
	}
	if liveFinalized != strings.ToLower(fin.Block.Hash) {
		return errors.New("finalized anchor changed before catch-up")
	}
	count := min(uint64(4), (end-height)/128)
	windows := make([]historicalWindow, int(count))
	fetchCtx, stop := context.WithTimeout(ctx, timeout)
	defer stop()
	var workers sync.WaitGroup
	for i := range windows {
		from := height + 1 + uint64(i)*128
		windows[i] = historicalWindow{from: from, to: from + 127}
		workers.Add(1)
		go func(i int) { defer workers.Done(); s.fetchHistoricalWindow(fetchCtx, c, &windows[i]) }(i)
	}
	workers.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(fetchCtx.Err(), context.DeadlineExceeded) {
		return sequential()
	}
	for _, window := range windows {
		if window.err != nil {
			return window.err
		}
		if !window.complete {
			return sequential()
		}
		var currentNumber, currentHash string
		if e = s.Store.DB.QueryRowContext(ctx, "SELECT block_number,block_hash FROM chain_checkpoints WHERE id=?", scope.Key()).Scan(&currentNumber, &currentHash); e != nil {
			return e
		}
		n, parseErr := strconv.ParseUint(currentNumber, 10, 64)
		if parseErr != nil {
			return parseErr
		}
		if n != height || currentHash != hash {
			return errors.New("checkpoint changed during catch-up")
		}
		previous, e := s.Header(ctx, c, height)
		if e != nil {
			return e
		}
		if previous != strings.ToLower(hash) {
			return errors.New("saved boundary reorganized during catch-up")
		}
		// Validate immediately before the single atomic projection transaction.
		live, e := s.Header(ctx, c, window.to)
		if e != nil {
			return e
		}
		if live != strings.ToLower(window.anchor.BlockHash) {
			return errors.New("prefetched snapshot reorganized")
		}
		expected := model.Anchor{BlockNumber: strconv.FormatUint(height, 10), BlockHash: hash}
		if e = s.Store.PublishWindow(ctx, scope, expected, window.anchor, window.operations); e != nil {
			return e
		}

		height = window.to
		hash = strings.ToLower(window.anchor.BlockHash)
	}
	return nil
}

func (s *Syncer) fetchHistoricalWindow(ctx context.Context, c config.Chain, w *historicalWindow) {
	cursor := ""
	seen := map[string]bool{}
	bytes := 0
	for range 8 {
		q := map[string]any{"limit": 100}
		if cursor != "" {
			q["cursor"] = cursor
		} else {
			q["fromBlock"] = w.from
			q["toBlock"] = w.to
		}
		var page model.OperationsPage
		if e := Call(ctx, s.Client, c.RPCURL, "invisi_getSettlementOperations", []any{q}, &page); e != nil {
			w.err = e
			return
		}
		if page.Snapshot.BlockNumber != strconv.FormatUint(w.to, 10) {
			w.err = errors.New("prefetch snapshot range mismatch")
			return
		}
		if _, e := wire.DecodeHex(page.Snapshot.BlockHash, 32); e != nil {
			w.err = e
			return
		}
		if w.anchor.BlockHash != "" && !strings.EqualFold(w.anchor.BlockHash, page.Snapshot.BlockHash) {
			w.err = errors.New("prefetch snapshot changed across pages")
			return
		}
		w.anchor = page.Snapshot
		encoded, e := json.Marshal(page)
		if e != nil {
			w.err = e
			return
		}
		bytes += len(encoded)
		if bytes > MaxResponseBytes {
			w.operations = nil
			return
		}
		for _, op := range page.Operations {
			if e := validateOperation(op, Scope(c), w.from, w.to); e != nil {
				w.err = e
				return
			}
			w.operations = append(w.operations, op)
		}
		if page.NextCursor == "" {
			w.complete = true
			return
		}
		if seen[page.NextCursor] {
			w.err = fmt.Errorf("prefetch cursor failed to advance")
			return
		}
		seen[page.NextCursor] = true
		cursor = page.NextCursor
	}
	// Dense ranges resume through Tick with its persisted cursor. An uncommitted
	// memory prefetch is disposable; it never replaces the durable progress.
	w.operations = nil
}

func (s *Syncer) recordSyncResult(ctx context.Context, scope wire.Scope, observation map[string]any, syncErr error) {
	now := strconv.FormatInt(time.Now().Unix(), 10)
	state := "ready"
	if syncErr != nil && !errors.Is(syncErr, ErrCaughtUp) {
		state = "unavailable"
	} else {
		observation["lastSuccessfulAt"] = now
	}
	observation["updatedAt"] = now
	var number, hash string
	if s.Store.DB.QueryRowContext(context.WithoutCancel(ctx), "SELECT block_number,block_hash FROM chain_checkpoints WHERE id=?", scope.Key()).Scan(&number, &hash) == nil {
		n, _ := strconv.ParseUint(number, 10, 64)
		observation["indexedHead"] = map[string]any{"blockNumber": strconv.FormatUint(n, 10), "blockHash": hash}
	}
	_ = s.Store.SourceStatus(context.WithoutCancel(ctx), scope, "protocol", state, observation)
}
