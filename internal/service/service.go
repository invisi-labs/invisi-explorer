// Package service composes the independent Explorer read model and HTTP server.
package service

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"github.com/gofrs/flock"
	"github.com/invisi-labs/invisi-explorer/internal/api"
	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/explorer"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	"github.com/invisi-labs/invisi-explorer/internal/readmodel"
	"github.com/invisi-labs/invisi-explorer/internal/upstream"
	"github.com/invisi-labs/invisi-explorer/internal/wire"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func Run(ctx context.Context, c config.Config) error {
	if e := c.Validate(); e != nil {
		return e
	}
	if e := os.MkdirAll(filepath.Dir(c.Database), 0700); e != nil {
		return e
	}
	lock := flock.New(c.Database + ".lock")
	locked, lockErr := lock.TryLock()
	if lockErr != nil {
		return lockErr
	}
	if !locked {
		return errors.New("explorer database is already in use")
	}
	defer lock.Unlock()
	store, e := readmodel.Open(c.Database)
	if e != nil {
		return e
	}
	defer store.Close()
	client := upstream.Client()
	defer client.CloseIdleConnections()
	syncer := &upstream.Syncer{Store: store, Client: client}
	key, e := cursorKey(c.Database + ".cursor-key")
	if e != nil {
		return e
	}
	scopes := []wire.Scope{}
	lookup := func(chain, protocol string) config.Chain {
		for _, v := range c.Chains {
			if v.ChainID == chain && strings.EqualFold(v.ProtocolAddress, protocol) {
				return v
			}
		}
		return config.Chain{}
	}
	for _, v := range c.Chains {
		if e = store.ConfigureRecognition(ctx, upstream.Scope(v), v.ProvenanceURL, v.IssuerID); e != nil {
			return e
		}
		scopes = append(scopes, upstream.Scope(v))
	}
	livePublicState := &publicState{syncer: syncer, chains: c.Chains}
	eview := explorer.New(store.DB, explorer.Options{BroadcasterDirectory: func(ctx context.Context) ([]model.BroadcasterDirectory, error) {
		return cachedBroadcasters(ctx, store, c.Chains)
	}, TransactionObservation: func(ctx context.Context, op model.Operation) (*model.TransactionObservation, error) {
		return syncer.TransactionObservation(ctx, lookup(op.ChainID, op.ProtocolAddress), op)
	}, Dialect: "sqlite", CursorKey: key, Scopes: scopes, MetadataProvider: func(ctx context.Context) (any, error) { return cachedMetadata(ctx, store, c) }, PublicState: &publicState{syncer: syncer, chains: c.Chains}, Recognition: func(ctx context.Context, op model.Operation) (any, error) {
		return syncer.Recognition(ctx, lookup(op.ChainID, op.ProtocolAddress), op)
	}, NoteRecognition: func(ctx context.Context, chain, protocol string, commitment wire.Field) (any, error) {
		return syncer.NoteRecognition(ctx, lookup(chain, protocol), commitment)
	}, Roots: func(ctx context.Context) (any, error) {
		sources := []any{}
		available := 0
		for _, chain := range c.Chains {
			var state string
			var updated int64
			_ = store.DB.QueryRowContext(ctx, "SELECT status,updated_at FROM source_status WHERE scope=? AND source='provenance'", upstream.Scope(chain).Key()).Scan(&state, &updated)
			if state == "available" && time.Now().Unix()-updated < 60 {
				available++
			} else {
				state = "unavailable"
			}
			sources = append(sources, map[string]any{"chainId": chain.ChainID, "protocolAddress": chain.ProtocolAddress, "status": state, "updatedAt": updated})
		}
		if available == 0 {
			return nil, &api.Error{Code: "PROVIDER_UNAVAILABLE"}
		}
		return map[string]any{"source": "provenance", "verification": "selected_upstream", "sources": sources}, nil
	}, Diagnostics: func(ctx context.Context) (any, error) { return syncer.Diagnostics(ctx, c.Chains) }})

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var workers sync.WaitGroup
	for _, chain := range c.Chains {
		workers.Add(1)
		go func() {
			defer workers.Done()
			timer := time.NewTimer(0)
			defer timer.Stop()
			for {
				select {
				case <-workerCtx.Done():
					return
				case <-timer.C:
					state := "ready"
					syncErr := syncer.CatchUp(workerCtx, chain)
					caughtUp := errors.Is(syncErr, upstream.ErrCaughtUp)
					if syncErr != nil && !caughtUp {
						state = "unavailable"
						log.Printf("chain %s synchronization unavailable", chain.ChainID)
					}
					delay := c.PollDuration()
					if !caughtUp && (state == "ready") {
						delay = 10 * time.Millisecond
					}
					timer.Reset(delay)
				}
			}
		}()
	}
	for _, chain := range c.Chains {
		workers.Add(1)
		go func() {
			defer workers.Done()
			timer := time.NewTimer(0)
			defer timer.Stop()
			for {
				select {
				case <-workerCtx.Done():
					return
				case <-timer.C:
					_ = syncer.HydrateCreditHistory(workerCtx, chain)
					timer.Reset(c.PollDuration())
				}
			}
		}()
	}
	metadataChains := map[string]bool{}
	for _, chain := range c.Chains {
		if metadataChains[chain.ChainID] {
			continue
		}
		metadataChains[chain.ChainID] = true
		workers.Add(1)
		go func() {
			defer workers.Done()
			timer := time.NewTimer(0)
			defer timer.Stop()
			for {
				select {
				case <-workerCtx.Done():
					return
				case <-timer.C:
					_ = syncer.RefreshTokens(workerCtx, chain, c.Tokens, time.Now())
					timer.Reset(c.PollDuration())
				}
			}
		}()
	}
	for _, chain := range c.Chains {
		if chain.ProvenanceURL == "" {
			continue
		}
		workers.Add(1)
		go func() {
			defer workers.Done()
			timer := time.NewTimer(0)
			defer timer.Stop()
			for {
				select {
				case <-workerCtx.Done():
					return
				case <-timer.C:
					optionalCtx, stop := context.WithTimeout(workerCtx, 3*time.Second)
					_ = syncer.SyncRoots(optionalCtx, chain)
					stop()
					_ = syncer.HydrateRecognition(workerCtx, chain)
					timer.Reset(c.PollDuration())
				}
			}
		}()
	}

	probeClient := upstream.BroadcasterClient()
	defer probeClient.CloseIdleConnections()
	for _, chain := range c.Chains {
		workers.Add(1)
		go func() {
			defer workers.Done()
			timer := time.NewTimer(0)
			defer timer.Stop()
			for {
				select {
				case <-workerCtx.Done():
					return
				case <-timer.C:
					probeCtx, stop := context.WithTimeout(workerCtx, 45*time.Second)
					_ = refreshBroadcasters(probeCtx, store, livePublicState, probeClient, chain)
					stop()
					timer.Reset(30 * time.Second)
				}
			}
		}()
	}

	defer func() { cancel(); workers.Wait() }()
	server := &http.Server{Addr: c.Listen, Handler: Handler(eview, c.WebDir), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	select {
	case <-ctx.Done():
		cancel()
		shutdownCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		return server.Shutdown(shutdownCtx)
	case err := <-done:
		cancel()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
func cursorKey(path string) ([]byte, error) {
	b, e := os.ReadFile(path)
	if e == nil {
		if len(b) != 32 {
			return nil, errors.New("invalid cursor key")
		}
		return b, nil
	}
	if !os.IsNotExist(e) {
		return nil, e
	}
	b = make([]byte, 32)
	if _, e = rand.Read(b); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(e) {
		return cursorKey(path)
	}
	if e != nil {
		return nil, e
	}
	_, e = f.Write(b)
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	return b, e
}
func metadata(c config.Config) any {
	networks := []any{}
	for _, chain := range c.Chains {
		networks = append(networks, map[string]any{"chainId": chain.ChainID, "protocolAddress": chain.ProtocolAddress, "name": chain.Name, "environment": chain.Environment, "explorerUrl": chain.ExplorerURL})
	}
	return map[string]any{"name": "Invisi Explorer", "apiVersion": "v1", "networks": networks, "tokens": c.Tokens}
}
func Handler(e *explorer.Explorer, webDir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		if r.URL.Path == "/healthz" {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"status":"ok"}`)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/") {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Content-Type", "application/json")
			path := strings.TrimPrefix(r.URL.Path, "/api/v1")
			if (path == "/search" && r.Method != "POST") || (path != "/search" && r.Method != "GET") {
				w.WriteHeader(405)
				json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "METHOD_NOT_ALLOWED"}})
				return
			}
			var body []byte
			if r.Method == "POST" {
				if r.URL.RawQuery != "" || !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
					writeError(w, &api.Error{Code: "INVALID_PARAMS"})
					return
				}
				var err error
				body, err = io.ReadAll(io.LimitReader(r.Body, 1025))
				if err != nil || len(body) > 1024 {
					writeError(w, &api.Error{Code: "INVALID_PARAMS"})
					return
				}
			}
			value, err := e.Public(r.Context(), path, r.URL.Query(), body)
			if err != nil {
				writeError(w, err)
				return
			}
			json.NewEncoder(w).Encode(value)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			w.WriteHeader(405)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		allowedAsset := strings.HasPrefix(r.URL.Path, "/assets/") || strings.HasPrefix(r.URL.Path, "/fonts/") || r.URL.Path == "/favicon.ico" || r.URL.Path == "/favicon.svg" || r.URL.Path == "/index.html"
		first := strings.Split(strings.Trim(r.URL.Path, "/"), "/")[0]
		appRoute := first == "" || first == "transactions" || first == "notes" || first == "credit-states" || first == "commitments" || first == "nullifiers" || first == "grants" || first == "trees" || first == "roots" || first == "addresses" || first == "broadcasters" || first == "diagnostics" || first == "search"
		if !allowedAsset && !appRoute {
			http.NotFound(w, r)
			return
		}
		clean := filepath.Clean(r.URL.Path)
		if strings.Contains(r.URL.Path, "..") || strings.Contains(r.URL.Path, "\\") {
			http.NotFound(w, r)
			return
		}
		file := filepath.Join(webDir, clean)
		if clean == "/" {
			file = filepath.Join(webDir, "index.html")
		}
		absoluteRoot, rootErr := filepath.EvalSymlinks(webDir)
		resolved, resolveErr := filepath.EvalSymlinks(file)
		if rootErr != nil || (resolveErr == nil && resolved != absoluteRoot && !strings.HasPrefix(resolved, absoluteRoot+string(os.PathSeparator))) {
			http.NotFound(w, r)
			return
		}
		info, err := os.Stat(file)
		if err != nil || info.IsDir() {
			if strings.HasPrefix(clean, "/assets/") || strings.HasPrefix(clean, "/fonts/") || filepath.Ext(clean) != "" {
				http.NotFound(w, r)
				return
			}
			file = filepath.Join(webDir, "index.html")
		}
		http.ServeFile(w, r, file)
	})
}
func writeError(w http.ResponseWriter, err error) {
	code := "UNAVAILABLE"
	var e *api.Error
	if errors.As(err, &e) {
		code = e.Code
	}
	status := 503
	switch code {
	case "INVALID_PARAMS", "SECRET_MATERIAL", "SCOPE_MISMATCH":
		status = 400
	case "NOT_FOUND", "NOT_INDEXED":
		status = 404
	case "CHAIN_REORGED", "CONFLICT":
		status = 409
	case "UNSUPPORTED":
		status = 422
	}
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code}})
}
