package upstream

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	"github.com/invisi-labs/invisi-explorer/internal/readmodel"
	"github.com/invisi-labs/invisi-explorer/internal/wire"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPublicRecognitionDropsNestedPrivateMaterial(t *testing.T) {
	value := map[string]any{"recognition": map[string]any{"recognitionState": "recognized", "proofJobId": "SECRET_JOB", "trackingId": "SECRET_TRACK", "privateKey": "SECRET_KEY"}, "fundingChecks": []any{map[string]any{"payer": "public", "decision": "allow", "witness": "SECRET_WITNESS", "checks": []any{map[string]any{"status": "allow", "proofJobId": "SECRET_NESTED"}}}}, "unknown": "SECRET_UNKNOWN"}
	b, _ := json.Marshal(PublicRecognition(value))
	if strings.Contains(string(b), "SECRET") || !strings.Contains(string(b), "recognized") || !strings.Contains(string(b), "public") {
		t.Fatalf("bad public filter: %s", b)
	}
}
func TestRecognitionHydrationUsesPositionalQueriesAndBindsScope(t *testing.T) {
	for _, wrongScope := range []bool{false, true} {
		t.Run(fmt.Sprint(wrongScope), func(t *testing.T) {
			ctx := context.Background()
			store, e := readmodel.Open(filepath.Join(t.TempDir(), "db"))
			if e != nil {
				t.Fatal(e)
			}
			defer store.Close()
			scope := wire.Scope{ChainID: "97", ProtocolAddress: "0x0000000000000000000000000000000000000001"}
			op := model.Operation{ChainID: scope.ChainID, ProtocolAddress: scope.ProtocolAddress, BlockNumber: "1", BlockHash: wire.ZeroHash, TxHash: wire.ZeroHash, OperationIndex: "0", Timestamp: "1", Success: true}
			if e = store.Stage(ctx, scope, []model.Operation{op}, "", "1"); e != nil {
				t.Fatal(e)
			}
			if e = store.Publish(ctx, scope, model.Anchor{BlockNumber: "1", BlockHash: wire.ZeroHash}); e != nil {
				t.Fatal(e)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var q struct {
					Method string            `json:"method"`
					Params []json.RawMessage `json:"params"`
				}
				if e := json.NewDecoder(r.Body).Decode(&q); e != nil {
					t.Errorf("nonpositional params %v", e)
					http.Error(w, "bad params", 400)
					return
				}
				result := map[string]any{"scope": scope, "chainGeneration": "1", "checkpointSequence": "0"}
				switch q.Method {
				case "provenance_getStatus":
					if len(q.Params) != 0 {
						t.Error("status params")
					}
				case "provenance_getRoots":
					if len(q.Params) != 1 {
						t.Error("root params")
					}
					result["items"] = []any{}
				case "provenance_getOperationRecognition":
					if len(q.Params) != 1 {
						t.Error("operation params")
					}
					result["operation"] = op
					result["recognitionState"] = "recognized"
					result["recognition"] = map[string]any{"recognitionState": "recognized", "proofJobId": "SECRET"}
					if wrongScope {
						result["scope"] = wire.Scope{ChainID: "1", ProtocolAddress: scope.ProtocolAddress}
					}
				default:
					t.Errorf("unexpected method %s", q.Method)
				}
				fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":%s}`, mustJSON(result))
			}))
			defer server.Close()
			s := &Syncer{Store: store, Client: server.Client()}
			c := config.Chain{ChainID: scope.ChainID, ProtocolAddress: scope.ProtocolAddress, ProvenanceURL: server.URL}
			if e = s.SyncRoots(ctx, c); e != nil {
				t.Fatal(e)
			}
			e = s.HydrateRecognition(ctx, c)
			if wrongScope && e == nil {
				t.Fatal("wrong scope accepted")
			}
			if !wrongScope && e != nil {
				t.Fatal(e)
			}
			value, e := s.Recognition(ctx, c, op)
			if e != nil {
				t.Fatal(e)
			}
			b, _ := json.Marshal(value)
			if strings.Contains(string(b), "SECRET") || wrongScope && strings.Contains(string(b), `"recognized"`) {
				t.Fatalf("unsafe recognition %s", b)
			}
		})
	}
}
func TestUnconfiguredRecognitionNeverCallsNetwork(t *testing.T) {
	store, e := readmodel.Open(filepath.Join(t.TempDir(), "db"))
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	s := &Syncer{Store: store, Client: &http.Client{Transport: failTransport{t}}}
	start := time.Now()
	for range 100 {
		v, e := s.Recognition(context.Background(), config.Chain{}, model.Operation{})
		if e != nil || v == nil {
			t.Fatal(e)
		}
	}
	if time.Since(start) > time.Second {
		t.Fatal("cached history is blocked by optional dependency")
	}
}

type failTransport struct{ t *testing.T }

func (f failTransport) RoundTrip(*http.Request) (*http.Response, error) {
	f.t.Fatal("recognition read attempted network")
	return nil, fmt.Errorf("unexpected")
}

func TestHydrationContinuesAfterOperationAbsenceButTripsOnSourceOutage(t *testing.T) {
	for _, code := range []int{-32021, -32002, 0} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			ctx := context.Background()
			store, e := readmodel.Open(filepath.Join(t.TempDir(), "db"))
			if e != nil {
				t.Fatal(e)
			}
			defer store.Close()
			scope := wire.Scope{ChainID: "97", ProtocolAddress: "0x0000000000000000000000000000000000000001"}
			known := model.Operation{ChainID: scope.ChainID, ProtocolAddress: scope.ProtocolAddress, BlockNumber: "1", BlockHash: wire.ZeroHash, TxHash: wire.ZeroHash, OperationIndex: "0", Timestamp: "1", Kind: "transact", Success: true}
			missing := known
			missing.BlockNumber = "2"
			missing.TxHash = "0x" + strings.Repeat("1", 64)
			if e = store.Stage(ctx, scope, []model.Operation{known, missing}, "", "2"); e != nil {
				t.Fatal(e)
			}
			if e = store.Publish(ctx, scope, model.Anchor{BlockNumber: "2", BlockHash: wire.ZeroHash}); e != nil {
				t.Fatal(e)
			}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var q struct {
					Params []struct {
						TxHash string `json:"txHash"`
					} `json:"params"`
				}
				if e := json.NewDecoder(r.Body).Decode(&q); e != nil {
					t.Fatal(e)
				}
				if q.Params[0].TxHash == missing.TxHash {
					w.WriteHeader(503)
					if code == 0 {
						fmt.Fprint(w, "PRIVATE gateway unavailable")
					} else {
						fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"error":{"code":%d,"message":"PRIVATE operation is absent"}}`, code)
					}
					return
				}
				fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":%s}`, mustJSON(map[string]any{"scope": scope, "operation": known, "recognitionState": "recognized"}))
			}))
			defer server.Close()
			syncer := &Syncer{Store: store, Client: server.Client()}
			chain := config.Chain{ChainID: scope.ChainID, ProtocolAddress: scope.ProtocolAddress, ProvenanceURL: server.URL}
			e = syncer.HydrateRecognition(ctx, chain)
			if code == 0 {
				if e == nil || calls != 1 {
					t.Fatalf("outage breaker failed calls=%d error=%v", calls, e)
				}
				if strings.Contains(e.Error(), "PRIVATE") {
					t.Fatal("outage exposed upstream body")
				}
				return
			}
			if e != nil || calls != 2 {
				t.Fatalf("per-operation absence starved later row calls=%d error=%v", calls, e)
			}
			value, e := syncer.Recognition(ctx, chain, known)
			if e != nil {
				t.Fatal(e)
			}
			if !strings.Contains(mustJSON(value), `"recognized"`) {
				t.Fatalf("later known row not hydrated: %s", mustJSON(value))
			}
		})
	}
}
