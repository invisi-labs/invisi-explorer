package upstream

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/readmodel"
	"github.com/invisi-labs/invisi-explorer/internal/wire"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestRealDeploymentEnvelopePagedSyncAndReorg(t *testing.T) {
	ctx := context.Background()
	store, e := readmodel.Open(filepath.Join(t.TempDir(), "db"))
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	protocol := "0x0000000000000000000000000000000000000001"
	hash := "0x" + strings.Repeat("1", 64)
	newHash := "0x" + strings.Repeat("2", 64)
	page := 0
	reorg := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		json.NewDecoder(r.Body).Decode(&q)
		var result any
		switch q.Method {
		case "invisi_getDeployment":
			result = map[string]any{"deployment": map[string]any{"chainId": "97", "protocol": map[string]any{"address": protocol, "runtimeCodeHash": wire.ZeroHash, "fromBlock": 1}}}
		case "invisi_getStatus":
			result = map[string]any{"indexedHead": map[string]any{"number": 1, "hash": hash}, "finality": map[string]any{"latest": map[string]any{"status": "available", "block": map[string]any{"number": 1, "hash": hash}}, "finalized": map[string]any{"status": "available", "block": map[string]any{"number": 1, "hash": hash}}}}
		case "invisi_readChainState":
			h := hash
			if reorg {
				h = newHash
			}
			result = map[string]any{"header": map[string]any{"number": "0x1"}, "headerHash": h}
		case "invisi_getSettlementOperations":
			page++
			cursor := ""
			ops := []any{}
			if page == 1 {
				cursor = "continue"
			} else {
				ops = append(ops, map[string]any{"chainId": "97", "protocolAddress": protocol, "blockNumber": "1", "blockHash": hash, "parentHash": wire.ZeroHash, "txHash": hash, "operationDigest": wire.ZeroHash, "operationIndex": "0", "logIndex": "0", "timestamp": "1", "kind": "unsupported", "unsupportedReason": "indirect_call_attribution_requires_traces"})
			}
			result = map[string]any{"operations": ops, "snapshot": map[string]any{"blockNumber": "1", "blockHash": hash}, "nextCursor": cursor}
		default:
			t.Errorf("unexpected method %s", q.Method)
		}
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":%s}`, mustJSON(result))
	}))
	defer server.Close()
	c := config.Chain{ChainID: "97", ProtocolAddress: protocol, RPCURL: server.URL, CodeHash: wire.ZeroHash, StartBlock: 1}
	s := &Syncer{Store: store, Client: server.Client()}
	if e = s.Tick(ctx, c); e != nil {
		t.Fatal(e)
	}
	var count, canonical int
	store.DB.QueryRow("SELECT COUNT(*),canonical FROM operations").Scan(&count, &canonical)
	if count != 1 || canonical != 1 || page != 2 {
		t.Fatalf("publication %d/%d pages %d", count, canonical, page)
	}
	diagnostics, diagErr := s.Diagnostics(ctx, []config.Chain{c})
	if diagErr != nil {
		t.Fatal(diagErr)
	}
	data := mustJSON(diagnostics)
	if !strings.Contains(data, `"indexedHead":{"blockHash"`) || !strings.Contains(data, `"blockNumber":"1"`) || !strings.Contains(data, `"finalized":{"block"`) || !strings.Contains(data, `"lastSuccessfulAt"`) {
		t.Fatalf("missing actual observed heads %s", data)
	}
	reorg = true
	if e = s.Tick(ctx, c); e != nil {
		t.Fatal(e)
	}
	store.DB.QueryRow("SELECT canonical FROM operations").Scan(&canonical)
	if canonical != 0 {
		t.Fatal("reorg retained canonical operation")
	}
	c.CodeHash = hash
	if _, e = s.Deployment(ctx, c); e == nil {
		t.Fatal("accepted changed deployment")
	}
}
func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
