package service

import (
	"context"
	"github.com/gofrs/flock"
	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/explorer"
	"github.com/invisi-labs/invisi-explorer/internal/readmodel"
	"github.com/invisi-labs/invisi-explorer/internal/wire"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHTTPBoundaryAndStaticFonts(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "fonts"), 0700)
	os.WriteFile(filepath.Join(dir, "fonts", "font.woff2"), []byte("font"), 0600)
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("Explorer"), 0600)
	os.WriteFile(filepath.Join(dir, ".env"), []byte("secret"), 0600)
	s, e := readmodel.Open(filepath.Join(t.TempDir(), "db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	view := explorer.New(s.DB, explorer.Options{Dialect: "sqlite", CursorKey: []byte(strings.Repeat("k", 32)), Scopes: []wire.Scope{{ChainID: "97", ProtocolAddress: "0x0000000000000000000000000000000000000001"}}})
	h := Handler(view, dir)
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{{"GET", "/fonts/font.woff2", "", 200}, {"GET", "/.env", "", 404}, {"GET", "/api/v1/no-such-route", "", 404}, {"GET", "/api/anything", "", 404}, {"GET", "/notes/hash", "", 200}, {"GET", "/api/v1/search", "", 405}, {"POST", "/api/v1/search?a=b", `{"query":"0x0000000000000000000000000000000000000001"}`, 400}, {"POST", "/api/v1/search", `{"query":"x","query":"y"}`, 400}, {"POST", "/api/v1/search", `{"query":"x"} {}`, 400}, {"POST", "/api/v1/search", strings.Repeat(" ", 1100) + `{"query":"0x0000000000000000000000000000000000000001"}`, 503}, {"GET", "/api/v1/overview", "", 503}} {
		r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Errorf("%s %s: got %d %s want %d", tc.method, tc.path, w.Code, w.Body.String(), tc.status)
		}
	}
	_ = context.Background()
	_ = http.MethodGet
}

func TestRunEnforcesSingleDatabaseOwner(t *testing.T) {
	cfg := config.Default()
	cfg.Database = filepath.Join(t.TempDir(), "own.db")
	lock := flock.New(cfg.Database + ".lock")
	ok, e := lock.TryLock()
	if e != nil || !ok {
		t.Fatal(e)
	}
	defer lock.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e = Run(ctx, cfg); e == nil || !strings.Contains(e.Error(), "already in use") {
		t.Fatalf("missing ownership guard: %v", e)
	}
}
