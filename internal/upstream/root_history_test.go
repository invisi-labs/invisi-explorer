package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/invisi-labs/invisi-explorer/internal/api"
	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/explorer"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	"github.com/invisi-labs/invisi-explorer/internal/readmodel"
	"github.com/invisi-labs/invisi-explorer/internal/wire"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func TestHistoricalRootBackfillResumesAndInvalidatesCursor(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "db")
	store, e := readmodel.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { store.Close() }()
	scope := wire.Scope{ChainID: "97", ProtocolAddress: "0x0000000000000000000000000000000000000001"}
	if e = store.Stage(ctx, scope, nil, "", "1"); e != nil {
		t.Fatal(e)
	}
	if e = store.Publish(ctx, scope, model.Anchor{BlockNumber: "1", BlockHash: wire.ZeroHash}); e != nil {
		t.Fatal(e)
	}
	generation := "2"
	historyFailure := false
	historicalAfter := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if e := json.NewDecoder(r.Body).Decode(&q); e != nil {
			t.Fatal(e)
		}
		result := map[string]any{"scope": scope, "chainGeneration": generation, "checkpointSequence": "1", "providerId": "issuer"}
		if q.Method == "provenance_getRoots" {
			var query struct {
				Generation string `json:"generation"`
				After      string `json:"after"`
				Limit      int    `json:"limit"`
			}
			json.Unmarshal(q.Params[0], &query)
			if query.Limit != 200 {
				t.Error("unbounded backfill")
			}
			if historyFailure && query.Generation != generation {
				fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"history unavailable"}}`)
				return
			}
			items := []any{}
			if query.Generation == generation {
				if query.After == "0" {
					items = append(items, certificate(scope, generation, "1"))
				}
			} else if query.Generation == "1" {
				historicalAfter = append(historicalAfter, query.After)
				if query.After == "0" {
					items = append(items, certificate(scope, "1", "1"))
					result["nextCursor"] = "opaque-history-continuation"
				} else if query.After == "1" {
					items = append(items, certificate(scope, "1", "2"))
				}
			}
			result["items"] = items
		}
		b, _ := json.Marshal(result)
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":%s}`, b)
	}))
	defer server.Close()
	chain := config.Chain{ChainID: scope.ChainID, ProtocolAddress: scope.ProtocolAddress, ProvenanceURL: server.URL, IssuerID: "issuer"}
	syncer := &Syncer{Store: store, Client: server.Client()}
	if e = syncer.SyncRoots(ctx, chain); e != nil {
		t.Fatal(e)
	}
	var count int
	store.DB.QueryRow("SELECT COUNT(*) FROM checkpoints").Scan(&count)
	if count != 2 {
		t.Fatalf("fresh generation2 cache omitted historical root: %d", count)
	}
	newExplorer := func() *explorer.Explorer {
		return explorer.New(store.DB, explorer.Options{Dialect: "sqlite", CursorKey: []byte(strings.Repeat("k", 32)), Scopes: []wire.Scope{scope}})
	}
	values := url.Values{"limit": {"1"}}
	v, e := newExplorer().Public(ctx, "/roots", values, nil)
	if e != nil {
		t.Fatal(e)
	}
	page := v.(map[string]any)
	cursor := page["nextCursor"].(string)
	if cursor == "" {
		t.Fatal("missing root cursor")
	}
	store.Close()
	store, e = readmodel.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	syncer.Store = store
	if e = syncer.SyncRoots(ctx, chain); e != nil {
		t.Fatal(e)
	}
	if strings.Join(historicalAfter, ",") != "0,1" {
		t.Fatalf("history did not resume after restart: %v", historicalAfter)
	}
	values.Set("cursor", cursor)
	_, e = newExplorer().Public(ctx, "/roots", values, nil)
	var public *api.Error
	if !errors.As(e, &public) || public.Code != "CONFLICT" {
		t.Fatalf("old cursor ignored changed history: %v", e)
	}
	values = url.Values{"limit": {"200"}}
	v, e = newExplorer().Public(ctx, "/roots", values, nil)
	if e != nil {
		t.Fatal(e)
	}
	items := v.(map[string]any)["items"].([]any)
	old := 0
	for _, item := range items {
		root := item.(map[string]any)
		if root["chainGeneration"] == "1" {
			old++
			if root["currentGeneration"] != false || !strings.HasPrefix(root["signature"].(string), "original-") {
				t.Fatalf("historical certificate relabeled: %+v", root)
			}
		}
	}
	if old != 2 {
		t.Fatalf("old roots lost %d", old)
	}
	generation = "3"
	if e = syncer.SyncRoots(ctx, chain); e != nil {
		t.Fatal(e)
	}
	var current string
	store.DB.QueryRow("SELECT generation FROM scopes WHERE id=?", scope.Key()).Scan(&current)
	if current != pad("3") {
		t.Fatalf("historical page replaced current generation %s", current)
	}
	generation = "4"
	historyFailure = true
	if e = syncer.SyncRoots(ctx, chain); e != nil {
		t.Fatalf("historical outage blocked current refresh: %v", e)
	}
	store.DB.QueryRow("SELECT generation FROM scopes WHERE id=?", scope.Key()).Scan(&current)
	if current != pad("4") {
		t.Fatalf("current roots were not committed before historical failure: %s", current)
	}
	var sourceState, historyState string
	store.DB.QueryRow("SELECT status FROM source_status WHERE scope=? AND source='provenance'", scope.Key()).Scan(&sourceState)
	store.DB.QueryRow("SELECT status FROM source_status WHERE scope=? AND source='provenance_history'", scope.Key()).Scan(&historyState)
	if sourceState != "available" || historyState != "unavailable" {
		t.Fatalf("history failure mislabeled current source %s/%s", sourceState, historyState)
	}

}
func certificate(scope wire.Scope, generation, sequence string) wire.Certificate {
	return wire.Certificate{ChainID: scope.ChainID, ProtocolAddress: scope.ProtocolAddress, ProviderID: "issuer", ChainGeneration: generation, RootSequence: sequence, Root: wire.UintField(1), TreeID: "1", LeafCount: sequence, Signature: "original-" + generation + "-" + sequence}
}
