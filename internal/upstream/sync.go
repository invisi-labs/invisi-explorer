package upstream

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	"github.com/invisi-labs/invisi-explorer/internal/readmodel"
	"github.com/invisi-labs/invisi-explorer/internal/wire"
	"net/http"
	"strconv"
	"strings"
)

var ErrCaughtUp = errors.New("caught up")

type Syncer struct {
	provenanceReads provenanceQuota
	transactions    transactionCache
	Store           *readmodel.Store
	Client          *http.Client
}
type Block struct {
	Number     uint64 `json:"number"`
	Hash       string `json:"hash"`
	ParentHash string `json:"parentHash"`
	Timestamp  uint64 `json:"timestamp"`
}
type Deployment struct {
	ChainID             string          `json:"chainId"`
	ProtocolAddress     string          `json:"protocolAddress"`
	CodeHash            string          `json:"codeHash"`
	StartBlock          uint64          `json:"startBlock"`
	BroadcasterRegistry json.RawMessage `json:"broadcasterRegistry"`
	Protocol            struct {
		Address         string `json:"address"`
		RuntimeCodeHash string `json:"runtimeCodeHash"`
		FromBlock       uint64 `json:"fromBlock"`
	} `json:"protocol"`
}

func Scope(c config.Chain) wire.Scope {
	return wire.Scope{ChainID: c.ChainID, ProtocolAddress: strings.ToLower(c.ProtocolAddress)}
}
func (s *Syncer) Deployment(ctx context.Context, c config.Chain) (Deployment, error) {
	var r struct {
		Deployment Deployment `json:"deployment"`
	}
	if e := Call(ctx, s.Client, c.RPCURL, "invisi_getDeployment", []any{}, &r); e != nil {
		return r.Deployment, e
	}
	d := r.Deployment
	d.ProtocolAddress = d.Protocol.Address
	d.CodeHash = d.Protocol.RuntimeCodeHash
	d.StartBlock = d.Protocol.FromBlock
	if d.ChainID != c.ChainID || !strings.EqualFold(d.ProtocolAddress, c.ProtocolAddress) || c.CodeHash != "" && !strings.EqualFold(c.CodeHash, d.CodeHash) || c.StartBlock != 0 && c.StartBlock != d.StartBlock {
		return d, errors.New("deployment identity mismatch")
	}
	return d, nil
}
func (s *Syncer) Header(ctx context.Context, c config.Chain, n uint64) (string, error) {
	var r struct {
		HeaderHash string `json:"headerHash"`
		Header     struct {
			Number string `json:"number"`
		} `json:"header"`
	}
	if e := Call(ctx, s.Client, c.RPCURL, "invisi_readChainState", []any{map[string]any{"kind": "header", "block": fmt.Sprintf("0x%x", n)}}, &r); e != nil {
		return "", e
	}
	height, e := strconv.ParseUint(strings.TrimPrefix(r.Header.Number, "0x"), 16, 64)
	if e != nil || height != n {
		return "", errors.New("header binding mismatch")
	}
	if _, e = wire.DecodeHex(r.HeaderHash, 32); e != nil {
		return "", e
	}
	return strings.ToLower(r.HeaderHash), nil
}

// Tick processes at most eight RPC pages and a 128-block window. Incomplete
// windows are durably staged and never become visible to public queries.
func (s *Syncer) Tick(ctx context.Context, c config.Chain) (tickErr error) {
	scope := Scope(c)
	observation := map[string]any{}
	var previous string
	if s.Store.DB.QueryRowContext(ctx, "SELECT payload FROM source_status WHERE scope=? AND source='protocol'", scope.Key()).Scan(&previous) == nil {
		_ = json.Unmarshal([]byte(previous), &observation)
	}
	defer func() { s.recordSyncResult(ctx, scope, observation, tickErr) }()

	d, e := s.Deployment(ctx, c)
	if e != nil {
		return e
	}
	var tip, hash string
	e = s.Store.DB.QueryRowContext(ctx, "SELECT block_number,block_hash FROM chain_checkpoints WHERE id=?", scope.Key()).Scan(&tip, &hash)
	start := d.StartBlock
	if e == nil {
		height, _ := strconv.ParseUint(tip, 10, 64)
		actual, e := s.Header(ctx, c, height)
		if e != nil {
			return e
		}
		if actual != strings.ToLower(hash) {
			return s.reconcile(ctx, c, height)
		}
		start = height + 1
	} else if !errors.Is(e, sql.ErrNoRows) {
		return e
	}
	var status struct {
		IndexedHead *Block `json:"indexedHead"`
		Finality    struct {
			Latest    Head `json:"latest"`
			Safe      Head `json:"safe"`
			Finalized Head `json:"finalized"`
		} `json:"finality"`
	}
	if e = Call(ctx, s.Client, c.RPCURL, "invisi_getStatus", []any{}, &status); e != nil {
		return e
	}
	observation["rpcIndexedHead"] = publicAnchor(status.IndexedHead)
	observation["chainHead"] = publicAnchor(status.Finality.Latest.Block)
	observation["finality"] = map[string]any{"latest": publicHead(status.Finality.Latest), "safe": publicHead(status.Finality.Safe), "finalized": publicHead(status.Finality.Finalized)}
	if status.IndexedHead == nil {
		return errors.New("protocol index unavailable")
	}
	if status.IndexedHead.Number < start {
		return ErrCaughtUp
	}
	end := min(start+127, status.IndexedHead.Number)
	var cursor, target string
	e = s.Store.DB.QueryRowContext(ctx, "SELECT cursor,target FROM sync_windows WHERE scope=?", scope.Key()).Scan(&cursor, &target)
	if e == nil {
		end, _ = strconv.ParseUint(target, 10, 64)
		// Empty cursor denotes a completed-but-unpublished stage. After a
		// restart refetch the range from a clean stage, never upsert a new
		// fork into terminal rows left by the previous attempt.
		if cursor == "" {
			if e = s.Store.ResetStage(ctx, scope); e != nil {
				return e
			}
		}
	} else if !errors.Is(e, sql.ErrNoRows) {
		return e
	}
	for range 8 {
		q := map[string]any{"limit": 100}
		if cursor != "" {
			q["cursor"] = cursor
		} else {
			q["fromBlock"] = start
			q["toBlock"] = end
		}
		var page model.OperationsPage
		if e = Call(ctx, s.Client, c.RPCURL, "invisi_getSettlementOperations", []any{q}, &page); e != nil {
			_ = s.Store.ResetStage(ctx, scope)
			return e
		}
		if page.Snapshot.BlockNumber != strconv.FormatUint(end, 10) {
			_ = s.Store.ResetStage(ctx, scope)
			return errors.New("snapshot range mismatch")
		}
		if _, e = wire.DecodeHex(page.Snapshot.BlockHash, 32); e != nil {
			return e
		}
		for i := range page.Operations {
			op := &page.Operations[i]
			if e = validateOperation(*op, scope, start, end); e != nil {
				return e
			}
		}
		if e = s.Store.Stage(ctx, scope, page.Operations, page.NextCursor, strconv.FormatUint(end, 10)); e != nil {
			return e
		}
		if page.NextCursor == "" {
			actual, e := s.Header(ctx, c, end)
			if e != nil {
				return e
			}
			if actual != strings.ToLower(page.Snapshot.BlockHash) {
				_ = s.Store.ResetStage(ctx, scope)
				return errors.New("chain reorganized")
			}
			return s.Store.Publish(ctx, scope, page.Snapshot)
		}
		if page.NextCursor == cursor {
			return errors.New("upstream cursor failed to advance")
		}
		cursor = page.NextCursor
	}
	return nil
}
func validateOperation(op model.Operation, scope wire.Scope, start, end uint64) error {
	if op.ChainID != scope.ChainID || !strings.EqualFold(op.ProtocolAddress, scope.ProtocolAddress) {
		return errors.New("operation scope mismatch")
	}
	n, e := wire.Uint64(op.BlockNumber)
	if e != nil || n < start || n > end {
		return errors.New("operation outside snapshot")
	}
	for _, v := range []string{op.Timestamp, op.OperationIndex, op.LogIndex} {
		if _, e = wire.Uint64(v); e != nil {
			return e
		}
	}
	for _, v := range []string{op.BlockHash, op.TxHash, op.ParentHash, op.OperationDigest} {
		if _, e = wire.DecodeHex(v, 32); e != nil {
			return e
		}
	}
	return nil
}
func (s *Syncer) reconcile(ctx context.Context, c config.Chain, tip uint64) error {
	scope := Scope(c)
	rows, e := s.Store.DB.QueryContext(ctx, "SELECT block_number,block_hash FROM chain_checkpoints WHERE id LIKE ? AND block_number<? ORDER BY block_number DESC LIMIT 16", scope.Key()+"/block/%", fmt.Sprintf("%020d", tip))
	if e != nil {
		return e
	}
	type anchor struct {
		n    uint64
		hash string
	}
	var anchors []anchor
	for rows.Next() {
		var n, h string
		if e = rows.Scan(&n, &h); e != nil {
			rows.Close()
			return e
		}
		v, _ := strconv.ParseUint(n, 10, 64)
		anchors = append(anchors, anchor{v, h})
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, a := range anchors {
		h, e := s.Header(ctx, c, a.n)
		if e != nil {
			return e
		}
		if h == strings.ToLower(a.hash) {
			return s.Store.Rollback(ctx, scope, a.n)
		}
	}
	if len(anchors) > 0 {
		return s.Store.Rollback(ctx, scope, anchors[len(anchors)-1].n-1)
	}
	return s.Store.Rollback(ctx, scope, 0)
}

// BlockAt obtains the complete Block wire identity required by anchored RPC queries.
func (s *Syncer) BlockAt(ctx context.Context, c config.Chain, n uint64) (Block, error) {
	var r struct {
		HeaderHash string `json:"headerHash"`
		Header     struct {
			Number     string `json:"number"`
			ParentHash string `json:"parentHash"`
			Timestamp  string `json:"timestamp"`
		} `json:"header"`
	}
	if e := Call(ctx, s.Client, c.RPCURL, "invisi_readChainState", []any{map[string]any{"kind": "header", "block": fmt.Sprintf("0x%x", n)}}, &r); e != nil {
		return Block{}, e
	}
	height, e := strconv.ParseUint(strings.TrimPrefix(r.Header.Number, "0x"), 16, 64)
	if e != nil || height != n {
		return Block{}, errors.New("header height mismatch")
	}
	timestamp, e := strconv.ParseUint(strings.TrimPrefix(r.Header.Timestamp, "0x"), 16, 64)
	if e != nil {
		return Block{}, e
	}
	for _, hash := range []string{r.HeaderHash, r.Header.ParentHash} {
		if _, e = wire.DecodeHex(hash, 32); e != nil {
			return Block{}, e
		}
	}
	return Block{Number: n, Hash: r.HeaderHash, ParentHash: r.Header.ParentHash, Timestamp: timestamp}, nil
}

type Head struct {
	Status string `json:"status"`
	Block  *Block `json:"block"`
}

func publicAnchor(b *Block) any {
	if b == nil {
		return nil
	}
	return map[string]any{"blockNumber": strconv.FormatUint(b.Number, 10), "blockHash": b.Hash}
}
func publicHead(h Head) any {
	status := h.Status
	if status == "" {
		status = "unavailable"
	}
	return map[string]any{"status": status, "block": publicAnchor(h.Block)}
}
