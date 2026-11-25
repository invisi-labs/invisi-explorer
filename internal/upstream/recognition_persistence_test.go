package upstream

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/explorer"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	"github.com/invisi-labs/invisi-explorer/internal/readmodel"
	"github.com/invisi-labs/invisi-explorer/internal/wire"
)

type recognitionFixture struct {
	t              *testing.T
	path           string
	store          *readmodel.Store
	syncer         *Syncer
	chain          config.Chain
	scope          wire.Scope
	op             model.Operation
	server         *httptest.Server
	generation     atomic.Uint64
	state          atomic.Value
	fail           atomic.Bool
	rootsFail      atomic.Bool
	wrongIdentity  atomic.Bool
	rollbackOnRead atomic.Bool
	calls          atomic.Int64
	pollTime       time.Time
}

func newRecognitionFixture(t *testing.T, state string) *recognitionFixture {
	t.Helper()
	f := &recognitionFixture{t: t, path: filepath.Join(t.TempDir(), "explorer.sqlite"), scope: wire.Scope{ChainID: "97", ProtocolAddress: "0x0000000000000000000000000000000000000001"}}
	f.generation.Store(1)
	f.state.Store(state)
	var err error
	f.store, err = readmodel.Open(f.path)
	if err != nil {
		t.Fatal(err)
	}
	f.op = model.Operation{ChainID: f.scope.ChainID, ProtocolAddress: f.scope.ProtocolAddress, BlockNumber: "100", BlockHash: wire.ZeroHash, TxHash: wire.ZeroHash, OperationIndex: "0", Timestamp: "1", Kind: "shield", Success: true, NoteOutputs: []model.NoteOutput{{NoteCommitment: wire.UintField(1), TreeID: "1", LeafIndex: "0"}}}
	if err = f.store.Stage(context.Background(), f.scope, []model.Operation{f.op}, "", "100"); err != nil {
		t.Fatal(err)
	}
	if err = f.store.Publish(context.Background(), f.scope, model.Anchor{BlockNumber: "100", BlockHash: wire.ZeroHash}); err != nil {
		t.Fatal(err)
	}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			t.Error(err)
			return
		}
		if f.fail.Load() || q.Method == "provenance_getRoots" && f.rootsFail.Load() {
			http.Error(w, "unavailable", 503)
			return
		}
		result := map[string]any{"scope": f.scope, "providerId": "issuer", "chainGeneration": fmt.Sprint(f.generation.Load()), "checkpointSequence": "1"}
		switch q.Method {
		case "provenance_getStatus":
		case "provenance_getRoots":
			result["items"] = []any{}
		case "provenance_getOperationRecognition":
			f.calls.Add(1)
			if f.rollbackOnRead.Swap(false) {
				if err := f.store.Rollback(context.Background(), f.scope, 100); err != nil {
					t.Error(err)
				}
			}
			result["operation"] = f.op
			result["recognitionState"] = f.state.Load()
			if f.wrongIdentity.Load() {
				result["providerId"] = "other-issuer"
			}
			if f.state.Load() == "recognized" {
				result["recognition"] = map[string]any{"recognitionState": "recognized", "chainGeneration": fmt.Sprint(f.generation.Load()), "checkpointSequence": "1"}
			}
		default:
			t.Errorf("unexpected method %s", q.Method)
		}
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":%s}`, mustJSON(result))
	}))
	f.chain = config.Chain{ChainID: f.scope.ChainID, ProtocolAddress: f.scope.ProtocolAddress, ProvenanceURL: f.server.URL, IssuerID: "issuer"}
	f.syncer = &Syncer{Store: f.store, Client: f.server.Client()}
	f.pollTime = time.Now()
	f.syncer.provenanceReads.now = func() time.Time { return f.pollTime }
	t.Cleanup(func() { f.server.Close(); f.store.Close() })
	f.refresh()
	return f
}
func (f *recognitionFixture) refresh() {
	f.pollTime = f.pollTime.Add(5 * time.Second)
	f.t.Helper()
	if err := f.syncer.SyncRoots(context.Background(), f.chain); err != nil {
		f.t.Fatal(err)
	}
	if err := f.syncer.HydrateRecognition(context.Background(), f.chain); err != nil {
		f.t.Fatal(err)
	}
}
func (f *recognitionFixture) age() {
	f.pollTime = f.pollTime.Add(time.Hour)
	f.t.Helper()
	if _, err := f.store.DB.Exec("UPDATE remote_recognition SET updated_at=?", time.Now().Unix()-3600); err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.store.DB.Exec("UPDATE recognition_attempts SET attempted_at=?", time.Now().Unix()-3600); err != nil {
		f.t.Fatal(err)
	}
}
func (f *recognitionFixture) assertState(want string) {
	f.t.Helper()
	v, err := f.syncer.Recognition(context.Background(), f.chain, f.op)
	if err != nil {
		f.t.Fatal(err)
	}
	if !strings.Contains(mustJSON(v), `"status":"`+want+`"`) {
		f.t.Fatalf("want %s got %s", want, mustJSON(v))
	}
}

func TestRecognizedCacheSurvivesExpiryAndRestart(t *testing.T) {
	f := newRecognitionFixture(t, "recognized")
	f.age()
	f.store.Close()
	var err error
	f.store, err = readmodel.Open(f.path)
	if err != nil {
		t.Fatal(err)
	}
	f.syncer.Store = f.store
	f.fail.Store(true)
	f.syncer.Client = &http.Client{Transport: failTransport{t}}
	f.assertState("recognized")
	v, err := f.syncer.NoteRecognition(context.Background(), f.chain, wire.UintField(1))
	if err != nil || !strings.Contains(mustJSON(v), `"status":"recognized"`) {
		t.Fatalf("note lost immutable decision: %s %v", mustJSON(v), err)
	}
	if err = f.syncer.HydrateRecognition(context.Background(), f.chain); err != nil {
		t.Fatal("recognized decision polled during outage:", err)
	}
	if f.calls.Load() != 1 {
		t.Fatal("recognized decision was polled again")
	}
}
func TestPendingCacheSurvivesFailureAndContinuesRefreshing(t *testing.T) {
	for _, identityFailure := range []bool{false, true} {
		t.Run(fmt.Sprint(identityFailure), func(t *testing.T) {
			f := newRecognitionFixture(t, "pending_proof")
			f.age()
			f.fail.Store(!identityFailure)
			f.wrongIdentity.Store(identityFailure)
			if err := f.syncer.HydrateRecognition(context.Background(), f.chain); err == nil {
				t.Fatal("query failure not reported")
			}
			f.assertState("pending_proof")
			f.fail.Store(false)
			f.wrongIdentity.Store(false)
			f.state.Store("recognized")
			f.age()
			if err := f.syncer.HydrateRecognition(context.Background(), f.chain); err != nil {
				t.Fatal(err)
			}
			f.assertState("recognized")
		})
	}
}
func TestGenerationChangeFencesRecognitionEvenWhenRootsFail(t *testing.T) {
	f := newRecognitionFixture(t, "recognized")
	f.generation.Store(2)
	f.rootsFail.Store(true)
	if err := f.syncer.SyncRoots(context.Background(), f.chain); err == nil {
		t.Fatal("root failure not reported")
	}
	f.assertState("provider_unavailable")
	v, _ := f.syncer.NoteRecognition(context.Background(), f.chain, wire.UintField(1))
	if strings.Contains(mustJSON(v), `"status":"recognized"`) {
		t.Fatal("old-generation note is recognized")
	}
	f.rootsFail.Store(false)
	f.refresh()
	f.assertState("recognized")
	var n int
	f.store.DB.QueryRow("SELECT COUNT(*) FROM remote_recognition").Scan(&n)
	if n != 1 {
		t.Fatal("recognition not rehydrated")
	}
}
func TestReorgFencesRetainedAndOrphanedRecognition(t *testing.T) {
	for _, height := range []uint64{99, 100} {
		t.Run(fmt.Sprint(height), func(t *testing.T) {
			f := newRecognitionFixture(t, "recognized")
			if err := f.store.Rollback(context.Background(), f.scope, height); err != nil {
				t.Fatal(err)
			}
			f.assertState("provider_unavailable")
			v, _ := f.syncer.NoteRecognition(context.Background(), f.chain, wire.UintField(1))
			if strings.Contains(mustJSON(v), `"status":"recognized"`) {
				t.Fatal("reorg retained old note recognition")
			}
		})
	}
}
func TestRecognitionWriteCannotCrossReorg(t *testing.T) {
	f := newRecognitionFixture(t, "pending_proof")
	f.age()
	f.state.Store("recognized")
	f.rollbackOnRead.Store(true)
	_ = f.syncer.HydrateRecognition(context.Background(), f.chain)
	f.assertState("provider_unavailable")
}
func TestRecognitionSourceChangeDoesNotReuseCachedIssuer(t *testing.T) {
	for _, field := range []string{"issuer", "url"} {
		t.Run(field, func(t *testing.T) {
			f := newRecognitionFixture(t, "recognized")
			if field == "issuer" {
				f.chain.IssuerID = "new-issuer"
			} else {
				f.chain.ProvenanceURL = f.server.URL + "/other"
			}
			f.assertState("provider_unavailable")
		})
	}
}
func TestOverviewCountsDurableRecognition(t *testing.T) {
	f := newRecognitionFixture(t, "recognized")
	f.age()
	e := explorer.New(f.store.DB, explorer.Options{Dialect: "sqlite", Scopes: []wire.Scope{f.scope}, CursorKey: []byte(strings.Repeat("k", 32))})
	v, err := e.Public(context.Background(), "/overview", url.Values{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if v.(map[string]any)["recognizedNotes"] != "1" {
		t.Fatalf("immutable count expired: %s", mustJSON(v))
	}
	if err = f.store.Rollback(context.Background(), f.scope, 100); err != nil {
		t.Fatal(err)
	}
	v, err = e.Public(context.Background(), "/overview", url.Values{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if v.(map[string]any)["recognizedNotes"] != nil {
		t.Fatal("overview counted reorged recognition")
	}
}

func TestTypedAbsenceDoesNotOverwritePendingEvidence(t *testing.T) {
	f := newRecognitionFixture(t, "pending_confirmation")
	f.age()
	absent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32021,"message":"index pending"}}`)
	}))
	defer absent.Close()
	// Keep the configured identity stable and replace only the transport target.
	f.syncer.Client = &http.Client{Transport: rewriteRecognitionTransport{target: absent.URL, base: absent.Client().Transport}}
	if err := f.syncer.HydrateRecognition(context.Background(), f.chain); err != nil {
		t.Fatal(err)
	}
	f.assertState("pending_confirmation")
}

type rewriteRecognitionTransport struct {
	target string
	base   http.RoundTripper
}

func (t rewriteRecognitionTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	u, err := url.Parse(t.target)
	if err != nil {
		return nil, err
	}
	r.URL = u
	return t.base.RoundTrip(r)
}

func TestUnpinnedProviderChangeInvalidatesRecognition(t *testing.T) {
	f := newRecognitionFixture(t, "recognized")
	f.chain.IssuerID = ""
	f.refresh()
	// A discovered issuer change must invalidate the cache even without an issuer pin.
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":%s}`, mustJSON(map[string]any{"scope": f.scope, "providerId": "other-issuer", "chainGeneration": "1", "checkpointSequence": "1", "items": []any{}}))
	}))
	defer provider.Close()
	f.syncer.Client = &http.Client{Transport: rewriteRecognitionTransport{target: provider.URL, base: provider.Client().Transport}}
	if err := f.syncer.SyncRoots(context.Background(), f.chain); err != nil {
		t.Fatal(err)
	}
	f.assertState("provider_unavailable")
}

func TestStaleGenerationResponsePreservesPendingEvidence(t *testing.T) {
	f := newRecognitionFixture(t, "pending_proof")
	f.age()
	f.generation.Store(2)
	if err := f.syncer.HydrateRecognition(context.Background(), f.chain); err == nil {
		t.Fatal("unobserved generation accepted")
	}
	f.assertState("pending_proof")
	f.refresh()
	f.assertState("pending_proof")
}
func TestCacheWriteCannotCrossSourceGenerationChange(t *testing.T) {
	f := newRecognitionFixture(t, "pending_proof")
	old, err := f.store.RecognitionContext(context.Background(), f.scope)
	if err != nil {
		t.Fatal(err)
	}
	f.generation.Store(2)
	if err = f.syncer.SyncRoots(context.Background(), f.chain); err != nil {
		t.Fatal(err)
	}
	if err = f.store.SaveBoundRecognition(context.Background(), f.op, []any{map[string]any{"status": "recognized"}}, time.Now().Unix(), old); err == nil {
		t.Fatal("old generation write accepted")
	}
	f.assertState("provider_unavailable")
}
func TestMalformedRecognitionCannotReplacePendingEvidence(t *testing.T) {
	f := newRecognitionFixture(t, "pending_proof")
	f.age()
	f.state.Store("bogus")
	if err := f.syncer.HydrateRecognition(context.Background(), f.chain); err == nil {
		t.Fatal("malformed state accepted")
	}
	f.assertState("pending_proof")
}
func TestRemovingRecognitionSourceInvalidatesCachedEvidence(t *testing.T) {
	f := newRecognitionFixture(t, "recognized")
	f.chain.ProvenanceURL = ""
	f.assertState("provider_unavailable")
	var n int
	if err := f.store.DB.QueryRow("SELECT COUNT(*) FROM current_recognition").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("removed source remains available to overview")
	}
}

func TestLegacyRecognitionRowsAreRevalidated(t *testing.T) {
	f := newRecognitionFixture(t, "recognized")
	if _, err := f.store.DB.Exec("DROP VIEW current_recognition; DROP TABLE recognition_bindings; DROP TABLE recognition_sources; DROP TABLE recognition_attempts"); err != nil {
		t.Fatal(err)
	}
	f.store.Close()
	var err error
	f.store, err = readmodel.Open(f.path)
	if err != nil {
		t.Fatal(err)
	}
	f.syncer.Store = f.store
	f.assertState("provider_unavailable")
	f.refresh()
	f.age()
	f.assertState("recognized")
}
func TestPendingRecognitionOutsideRecentWindowStillRefreshes(t *testing.T) {
	f := newRecognitionFixture(t, "pending_proof")
	f.age()
	binding, err := f.store.RecognitionContext(context.Background(), f.scope)
	if err != nil {
		t.Fatal(err)
	}
	var ops []model.Operation
	for i := 101; i <= 301; i++ {
		op := f.op
		op.BlockNumber = fmt.Sprint(i)
		op.TxHash = string(wire.UintField(uint64(i)))
		ops = append(ops, op)
	}
	if err = f.store.Stage(context.Background(), f.scope, ops, "", "301"); err != nil {
		t.Fatal(err)
	}
	if err = f.store.Publish(context.Background(), f.scope, model.Anchor{BlockNumber: "301", BlockHash: wire.ZeroHash}); err != nil {
		t.Fatal(err)
	}
	for _, op := range ops {
		if err = f.store.SaveBoundRecognition(context.Background(), op, []any{map[string]any{"status": "recognized"}}, time.Now().Unix(), binding); err != nil {
			t.Fatal(err)
		}
	}
	f.state.Store("recognized")
	if err = f.syncer.HydrateRecognition(context.Background(), f.chain); err != nil {
		t.Fatal(err)
	}
	f.assertState("recognized")
}

func TestFailedRecognitionDoesNotStarveLaterPendingRows(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(fmt.Sprint(typed), func(t *testing.T) {
			f := newRecognitionFixture(t, "pending_proof")
			f.age()
			binding, err := f.store.RecognitionContext(context.Background(), f.scope)
			if err != nil {
				t.Fatal(err)
			}
			n := 1
			if typed {
				n = 20
			}
			ops := []model.Operation{f.op}
			for i := 1; i <= n; i++ {
				op := f.op
				op.TxHash = string(wire.UintField(uint64(i)))
				ops = append(ops, op)
			}
			if err = f.store.Stage(context.Background(), f.scope, ops, "", "100"); err != nil {
				t.Fatal(err)
			}
			if err = f.store.Publish(context.Background(), f.scope, model.Anchor{BlockNumber: "100", BlockHash: wire.ZeroHash}); err != nil {
				t.Fatal(err)
			}
			for i, op := range ops {
				if err = f.store.SaveBoundRecognition(context.Background(), op, []any{map[string]any{"status": "pending_proof"}}, time.Now().Unix()-3600+int64(i), binding); err != nil {
					t.Fatal(err)
				}
			}
			known := ops[len(ops)-1]
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var q struct {
					Params []struct {
						TxHash string `json:"txHash"`
					} `json:"params"`
				}
				if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
					t.Error(err)
					return
				}
				if q.Params[0].TxHash != known.TxHash {
					if typed {
						fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32021,"message":"pending"}}`)
					} else {
						http.Error(w, "unavailable", 503)
					}
					return
				}
				fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":%s}`, mustJSON(map[string]any{"scope": f.scope, "providerId": "issuer", "chainGeneration": "1", "operation": known, "recognitionState": "recognized"}))
			}))
			defer server.Close()
			f.syncer.Client = &http.Client{Transport: rewriteRecognitionTransport{target: server.URL, base: server.Client().Transport}}
			// Hydration now yields to the shared quota. Advance several poll
			// intervals without aging failed attempts back into the due queue.
			for pass := 0; pass < 8; pass++ {
				f.pollTime = f.pollTime.Add(5 * time.Second)
				_ = f.syncer.HydrateRecognition(context.Background(), f.chain)
			}
			value, err := f.syncer.Recognition(context.Background(), f.chain, known)
			if err != nil || !strings.Contains(mustJSON(value), `"status":"recognized"`) {
				t.Fatalf("later pending row starved: %s %v", mustJSON(value), err)
			}
		})
	}
}
func TestRevalidatedBindingChangesSnapshotRevision(t *testing.T) {
	f := newRecognitionFixture(t, "recognized")
	if err := f.store.Rollback(context.Background(), f.scope, 100); err != nil {
		t.Fatal(err)
	}
	var before, after int64
	if err := f.store.DB.QueryRow("SELECT version FROM readmodel_versions WHERE id=?", f.scope.Key()).Scan(&before); err != nil {
		t.Fatal(err)
	}
	f.refresh()
	if err := f.store.DB.QueryRow("SELECT version FROM readmodel_versions WHERE id=?", f.scope.Key()).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after <= before {
		t.Fatal("revalidated result did not invalidate prior snapshots")
	}
}
func TestMalformedStatusDoesNotInvalidateSuccessfulRecognition(t *testing.T) {
	f := newRecognitionFixture(t, "recognized")
	f.chain.IssuerID = ""
	f.refresh()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":%s}`, mustJSON(map[string]any{"scope": f.scope, "chainGeneration": "1", "checkpointSequence": "1", "items": []any{}}))
	}))
	defer server.Close()
	f.syncer.Client = &http.Client{Transport: rewriteRecognitionTransport{target: server.URL, base: server.Client().Transport}}
	if err := f.syncer.SyncRoots(context.Background(), f.chain); err == nil {
		t.Fatal("missing provider identity accepted")
	}
	f.assertState("recognized")
}
