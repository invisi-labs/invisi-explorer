package upstream

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	"github.com/invisi-labs/invisi-explorer/internal/readmodel"
	"github.com/invisi-labs/invisi-explorer/internal/wire"
)

func prefetchHash(n uint64) string { return fmt.Sprintf("0x%064x", n+1) }
func prefetchFixture(t *testing.T, mode string) (*Syncer, config.Chain, *atomic.Int64) {
	t.Helper()
	store, e := readmodel.Open(filepath.Join(t.TempDir(), "explorer.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { store.Close() })
	c := config.Chain{ChainID: "97", ProtocolAddress: "0x" + strings.Repeat("1", 40), CodeHash: wire.ZeroHash, StartBlock: 1}
	if e = store.Stage(context.Background(), Scope(c), nil, "", "128"); e != nil {
		t.Fatal(e)
	}
	if e = store.Publish(context.Background(), Scope(c), model.Anchor{BlockNumber: "128", BlockHash: prefetchHash(128)}); e != nil {
		t.Fatal(e)
	}
	active, maxActive := new(atomic.Int64), new(atomic.Int64)
	startedSecond := make(chan struct{})
	var once sync.Once
	var fetching atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			Method string
			Params []map[string]json.RawMessage
		}
		json.NewDecoder(r.Body).Decode(&q)
		var result any
		switch q.Method {
		case "invisi_getDeployment":
			result = map[string]any{"deployment": map[string]any{"chainId": "97", "protocol": map[string]any{"address": c.ProtocolAddress, "runtimeCodeHash": wire.ZeroHash, "fromBlock": 1}}}
		case "invisi_getStatus":
			result = map[string]any{"indexedHead": map[string]any{"number": 640, "hash": prefetchHash(640)}, "finality": map[string]any{"finalized": map[string]any{"status": "available", "block": map[string]any{"number": 640, "hash": prefetchHash(640)}}}}
		case "invisi_readChainState":
			var tag string
			json.Unmarshal(q.Params[0]["block"], &tag)
			n, _ := strconv.ParseUint(strings.TrimPrefix(tag, "0x"), 16, 64)
			hash := prefetchHash(n)
			if mode == "reorg" && fetching.Load() && n == 128 {
				hash = prefetchHash(999)
			}
			result = map[string]any{"header": map[string]any{"number": tag}, "headerHash": hash}
		case "invisi_getSettlementOperations":
			fetching.Store(true)
			current := active.Add(1)
			defer active.Add(-1)
			for old := maxActive.Load(); current > old && !maxActive.CompareAndSwap(old, current); old = maxActive.Load() {
			}
			var from, to uint64
			json.Unmarshal(q.Params[0]["fromBlock"], &from)
			json.Unmarshal(q.Params[0]["toBlock"], &to)
			var cursor string
			json.Unmarshal(q.Params[0]["cursor"], &cursor)
			if from > 129 {
				once.Do(func() { close(startedSecond) })
			}
			if mode == "ordered" && from == 129 {
				select {
				case <-startedSecond:
				case <-r.Context().Done():
					return
				}
			}
			if cursor != "" {
				from = 129
				to = 256
			}
			hash := prefetchHash(to)
			if mode == "bad-second" && from == 257 {
				hash = prefetchHash(999)
			}
			op := map[string]any{"chainId": "97", "protocolAddress": c.ProtocolAddress, "blockNumber": strconv.FormatUint(from, 10), "blockHash": prefetchHash(from), "parentHash": prefetchHash(from - 1), "txHash": prefetchHash(from + 1000), "operationDigest": wire.ZeroHash, "operationIndex": "0", "logIndex": "0", "timestamp": "1", "kind": "unsupported", "success": false}
			next := ""
			ops := []any{op}
			if mode == "empty" {
				ops = []any{}
			}
			if (mode == "dense" || mode == "paged") && from == 129 {
				page := 0
				if cursor != "" {
					page, _ = strconv.Atoi(cursor)
				}
				maxPage := 8
				if mode == "paged" {
					maxPage = 7
				}
				if page < maxPage {
					next = strconv.Itoa(page + 1)
				}
				ops = []any{}
			}
			result = map[string]any{"operations": ops, "snapshot": map[string]any{"blockNumber": strconv.FormatUint(to, 10), "blockHash": hash}, "nextCursor": next}
		default:
			t.Errorf("unexpected method %s", q.Method)
		}
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":%s}`, mustJSON(result))
	}))
	t.Cleanup(server.Close)
	c.RPCURL = server.URL
	return &Syncer{Store: store, Client: server.Client()}, c, maxActive
}
func checkpointNumber(t *testing.T, s *Syncer, c config.Chain) uint64 {
	t.Helper()
	var v string
	if e := s.Store.DB.QueryRow("SELECT block_number FROM chain_checkpoints WHERE id=?", Scope(c).Key()).Scan(&v); e != nil {
		t.Fatal(e)
	}
	n, _ := strconv.ParseUint(v, 10, 64)
	return n
}
func TestCatchUpPrefetchesButPublishesContiguousWindows(t *testing.T) {
	s, c, parallel := prefetchFixture(t, "ordered")
	ctx, stop := context.WithTimeout(context.Background(), 3*time.Second)
	defer stop()
	if e := s.CatchUp(ctx, c); e != nil {
		t.Fatal(e)
	}
	if parallel.Load() < 2 {
		t.Fatal("historical reads still serialized")
	}
	if n := checkpointNumber(t, s, c); n != 640 {
		t.Fatalf("checkpoint %d", n)
	}
	var count int
	s.Store.DB.QueryRow("SELECT COUNT(*) FROM operations WHERE canonical=1").Scan(&count)
	if count != 4 {
		t.Fatalf("missing or duplicated failed transactions: %d", count)
	}
}
func TestCatchUpDoesNotPublishAcrossInvalidWindow(t *testing.T) {
	s, c, _ := prefetchFixture(t, "bad-second")
	if e := s.CatchUp(context.Background(), c); e == nil {
		t.Fatal("stale snapshot accepted")
	}
	if n := checkpointNumber(t, s, c); n != 256 {
		t.Fatalf("published across gap: %d", n)
	}
	var count int
	s.Store.DB.QueryRow("SELECT COUNT(*) FROM operations WHERE canonical=1").Scan(&count)
	if count != 1 {
		t.Fatalf("later windows leaked: %d", count)
	}
}
func TestCatchUpRechecksSavedBoundaryBeforePublication(t *testing.T) {
	s, c, _ := prefetchFixture(t, "reorg")
	if e := s.CatchUp(context.Background(), c); e == nil {
		t.Fatal("reorg accepted")
	}
	if n := checkpointNumber(t, s, c); n != 128 {
		t.Fatalf("stale checkpoint advanced: %d", n)
	}
}
func TestCatchUpRetainsDenseCursorAndSequentialResume(t *testing.T) {
	s, c, _ := prefetchFixture(t, "dense")
	if e := s.CatchUp(context.Background(), c); e != nil {
		t.Fatal(e)
	}
	if n := checkpointNumber(t, s, c); n != 128 {
		t.Fatalf("incomplete dense range published: %d", n)
	}
	var cursor string
	if e := s.Store.DB.QueryRow("SELECT cursor FROM sync_windows WHERE scope=?", Scope(c).Key()).Scan(&cursor); e != nil || cursor != "8" {
		t.Fatalf("cursor lost: %q %v", cursor, e)
	}
	if e := s.CatchUp(context.Background(), c); e != nil {
		t.Fatal(e)
	}
	if n := checkpointNumber(t, s, c); n != 256 {
		t.Fatalf("dense continuation lost: %d", n)
	}
}
