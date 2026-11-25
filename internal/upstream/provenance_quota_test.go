package upstream

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	"github.com/invisi-labs/invisi-explorer/internal/readmodel"
	"github.com/invisi-labs/invisi-explorer/internal/wire"
)

// Provenance's default read quota is 120/minute per client IP, shared by all
// methods and chain paths. Pending operation refreshes must leave room for roots.
func TestPendingRecognitionBacklogDoesNotExhaustRootQuota(t *testing.T) {
	ctx := context.Background()
	store, err := readmodel.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	scope := wire.Scope{ChainID: "97", ProtocolAddress: "0x0000000000000000000000000000000000000001"}
	ops := []model.Operation{}
	byHash := map[string]model.Operation{}
	for i := 0; i < 80; i++ {
		op := model.Operation{ChainID: scope.ChainID, ProtocolAddress: scope.ProtocolAddress, BlockNumber: "1", BlockHash: wire.ZeroHash, TxHash: fmt.Sprintf("0x%064x", i+1), OperationIndex: "0", Success: true, NoteOutputs: []model.NoteOutput{{NoteCommitment: wire.UintField(uint64(i + 1)), TreeID: "1", LeafIndex: fmt.Sprint(i)}}}
		ops = append(ops, op)
		byHash[op.TxHash] = op
	}
	if err = store.Stage(ctx, scope, ops, "", "1"); err != nil {
		t.Fatal(err)
	}
	if err = store.Publish(ctx, scope, model.Anchor{BlockNumber: "1", BlockHash: wire.ZeroHash}); err != nil {
		t.Fatal(err)
	}
	var calls, rejected atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			Method string           `json:"method"`
			Params []map[string]any `json:"params"`
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil {
			t.Error("bad request")
			return
		}
		if calls.Add(1) > 120 {
			rejected.Add(1)
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(429)
			fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32013,"message":"rate limited"}}`)
			return
		}
		result := map[string]any{"scope": scope, "providerId": "issuer", "chainGeneration": "1", "checkpointSequence": "1"}
		switch q.Method {
		case "provenance_getStatus":
		case "provenance_getRoots":
			result["items"] = []wire.Certificate{}
			if q.Params[0]["after"] == "0" {
				result["items"] = []wire.Certificate{certificate(scope, "1", "1")}
			}
		case "provenance_getOperationRecognition":
			result["operation"] = byHash[q.Params[0]["txHash"].(string)]
			result["recognitionState"] = "pending_proof"
		default:
			t.Errorf("unexpected method %s", q.Method)
		}
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":%s}`, mustJSON(result))
	}))
	defer server.Close()
	syncer := &Syncer{Store: store, Client: server.Client()}
	chain := config.Chain{ChainID: scope.ChainID, ProtocolAddress: scope.ProtocolAddress, ProvenanceURL: server.URL, IssuerID: "issuer"}
	start := time.Now()
	now := start
	syncer.provenanceReads.now = func() time.Time { return now }
	for pass := 0; pass < 12; pass++ {
		now = now.Add(5 * time.Second)
		if err = syncer.SyncRoots(ctx, chain); err != nil {
			t.Fatalf("root sync starved by pending backlog at pass %d: %v (calls=%d)", pass, err, calls.Load())
		}
		if err = syncer.HydrateRecognition(ctx, chain); err != nil {
			t.Fatalf("pending refresh exhausted shared quota: %v (calls=%d)", err, calls.Load())
		}
		// Make pending rows due without sleeping through the production refresh TTL.
		if _, err = store.DB.Exec("UPDATE remote_recognition SET updated_at=0; UPDATE recognition_attempts SET attempted_at=0"); err != nil {
			t.Fatal(err)
		}
	}
	if time.Since(start) >= time.Minute {
		t.Fatal("test exceeded the upstream quota window")
	}
	if rejected.Load() != 0 {
		t.Fatalf("upstream quota exceeded: %d", rejected.Load())
	}
	var n int
	if err = store.DB.QueryRow("SELECT COUNT(*) FROM remote_recognition").Scan(&n); err != nil || n == 0 {
		t.Fatalf("pending refresh stopped entirely: %d %v", n, err)
	}
}

func TestProvenanceQuotaReservesRootsAcrossChainPaths(t *testing.T) {
	var q provenanceQuota
	now := time.Now()
	reads := 0
	for pass := 0; pass < 12; pass++ {
		for _, method := range []string{"status", "roots"} {
			if delay := q.acquire("http://provenance:18660/v1/chains/97", false, now); delay != 0 {
				t.Fatalf("%s starved at pass %d: %v", method, pass, delay)
			}
			reads++
		}
		for i := 0; i < 20; i++ {
			if q.acquire("http://provenance:18660/v1/chains/56", true, now) != 0 {
				break
			}
			reads++
		}
		now = now.Add(5 * time.Second)
	}
	if reads > 100 || reads <= 24 {
		t.Fatalf("unsafe or idle shared budget: %d", reads)
	}
	// A different chain path cannot escape cooldown; a different origin can.
	q.block("http://provenance:18660/v1/chains/97", now.Add(time.Minute))
	if q.acquire("http://provenance:18660/v1/chains/56", false, now) != time.Minute {
		t.Fatal("chain path bypassed origin cooldown")
	}
	if q.acquire("http://other:18660/v1/chains/97", false, now) != 0 {
		t.Fatal("unrelated origin throttled")
	}
	if q.acquire("http://provenance:18660/v1/chains/56", false, now.Add(time.Minute)) != 0 {
		t.Fatal("cooldown did not expire")
	}
}

func TestProvenanceRateLimitCooldownIsBoundedAndCancelable(t *testing.T) {
	var count atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(429)
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32013,"message":"PRIVATE diagnostics"}}`)
	}))
	defer server.Close()
	s := &Syncer{Client: server.Client()}
	var out any
	if err := s.callProvenance(context.Background(), server.URL, "provenance_getStatus", []any{}, &out); err == nil {
		t.Fatal("rate limit accepted")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := s.callProvenance(ctx, server.URL, "provenance_getStatus", []any{}, &out); err == nil {
		t.Fatal("cooldown ignored")
	}
	if count.Load() != 1 {
		t.Fatalf("retried during cooldown: %d", count.Load())
	}
}
