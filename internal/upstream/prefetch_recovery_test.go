package upstream

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/invisi-labs/invisi-explorer/internal/model"
)

func TestCatchUpDropsInterruptedTerminalStageBeforeRefetch(t *testing.T) {
	s, c, _ := prefetchFixture(t, "empty")
	op := model.Operation{ChainID: c.ChainID, ProtocolAddress: c.ProtocolAddress, BlockNumber: "130", BlockHash: prefetchHash(130), TxHash: prefetchHash(999), OperationIndex: "0"}
	if e := s.Store.Stage(context.Background(), Scope(c), []model.Operation{op}, "", "256"); e != nil {
		t.Fatal(e)
	}
	if e := s.CatchUp(context.Background(), c); e != nil {
		t.Fatal(e)
	}
	var count int
	s.Store.DB.QueryRow("SELECT COUNT(*) FROM operations WHERE canonical=1").Scan(&count)
	if count != 0 {
		t.Fatal("orphan operation from interrupted terminal stage became canonical")
	}
}
func failingProfileProxy(t *testing.T, root, method string, delay time.Duration) string {
	t.Helper()
	target, _ := url.Parse(root)
	proxy := httputil.NewSingleHostReverseProxy(target)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		r.Body.Close()
		r.Body = io.NopCloser(strings.NewReader(string(raw)))
		var q struct{ Method string }
		json.Unmarshal(raw, &q)
		if q.Method == method {
			if delay == 0 {
				io.WriteString(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"unavailable"}}`)
				return
			}
			select {
			case <-time.After(delay):
			case <-r.Context().Done():
				return
			}
		}
		proxy.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return server.URL
}
func TestCatchUpRecordsErrorsAndRetainsLastSuccess(t *testing.T) {
	for _, method := range []string{"invisi_getDeployment", "invisi_getSettlementOperations"} {
		t.Run(method, func(t *testing.T) {
			s, c, _ := prefetchFixture(t, "")
			if e := s.CatchUp(context.Background(), c); e != nil {
				t.Fatal(e)
			}
			var payload string
			s.Store.DB.QueryRow("SELECT payload FROM source_status WHERE scope=? AND source='protocol'", Scope(c).Key()).Scan(&payload)
			var previous map[string]any
			json.Unmarshal([]byte(payload), &previous)
			if e := s.Store.Rollback(context.Background(), Scope(c), 128); e != nil {
				t.Fatal(e)
			}
			c.RPCURL = failingProfileProxy(t, c.RPCURL, method, 0)
			if e := s.CatchUp(context.Background(), c); e == nil {
				t.Fatal("upstream failure ignored")
			}
			var state string
			s.Store.DB.QueryRow("SELECT status,payload FROM source_status WHERE scope=? AND source='protocol'", Scope(c).Key()).Scan(&state, &payload)
			var current map[string]any
			json.Unmarshal([]byte(payload), &current)
			if state != "unavailable" || current["lastSuccessfulAt"] != previous["lastSuccessfulAt"] {
				t.Fatalf("bad observation: %s %s", state, payload)
			}
		})
	}
}
func TestCatchUpSlowPrefetchFallsBackToDurableTick(t *testing.T) {
	s, c, _ := prefetchFixture(t, "paged")
	c.RPCURL = failingProfileProxy(t, c.RPCURL, "invisi_getSettlementOperations", 10*time.Millisecond)
	if e := s.catchUp(context.Background(), c, 20*time.Millisecond); e != nil {
		t.Fatal(e)
	}
	if n := checkpointNumber(t, s, c); n != 256 {
		t.Fatalf("healthy slow upstream made no durable progress: %d", n)
	}
}
