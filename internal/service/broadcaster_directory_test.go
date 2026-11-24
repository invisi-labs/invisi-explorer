package service

import (
	"context"
	"encoding/json"
	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/readmodel"
	"github.com/invisi-labs/invisi-explorer/internal/upstream"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegisteredDirectoryCachePersistsAndDoesNotProbeDisabledServices(t *testing.T) {
	ctx := context.Background()
	protocol := "0x" + strings.Repeat("1", 40)
	hash := "0x" + strings.Repeat("2", 64)
	operator := "0x" + strings.Repeat("4", 40)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		json.NewDecoder(r.Body).Decode(&q)
		var result any
		switch q.Method {
		case "invisi_getDeployment":
			result = map[string]any{"deployment": map[string]any{"chainId": "97", "protocol": map[string]any{"address": protocol, "runtimeCodeHash": hash, "fromBlock": 1}, "broadcasterRegistry": map[string]any{"address": operator, "runtimeCodeHash": hash, "fromBlock": 1}}}
		case "invisi_readChainState":
			result = map[string]any{"headerHash": hash, "header": map[string]any{"number": "0xa", "parentHash": hash, "timestamp": "0x64"}}
		case "invisi_getBroadcasters":
			var params struct {
				Block upstream.Block `json:"block"`
			}
			json.Unmarshal(q.Params[0], &params)
			result = map[string]any{"block": params.Block, "broadcasters": []any{map[string]any{"operator": operator, "endpoint": "https://127.0.0.1/private", "quoteSigner": operator, "feeRecipient": operator, "enabled": false, "block": params.Block}}}
		default:
			t.Error(q.Method)
		}
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "cache.sqlite")
	store, err := readmodel.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	chain := config.Chain{ChainID: "97", ProtocolAddress: protocol, RPCURL: server.URL}
	_, err = store.DB.Exec("INSERT INTO chain_checkpoints VALUES(?,?,?)", upstream.Scope(chain).Key(), "10", hash)
	if err != nil {
		t.Fatal(err)
	}
	live := &publicState{syncer: &upstream.Syncer{Store: store, Client: server.Client()}, chains: []config.Chain{chain}}
	if err = refreshBroadcasters(ctx, store, live, upstream.BroadcasterClient(), chain); err != nil {
		t.Fatal(err)
	}
	store.Close()
	server.Close()
	store, err = readmodel.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	cached, err := cachedBroadcasters(ctx, store, []config.Chain{chain})
	if err != nil || len(cached) != 1 || len(cached[0].Items) != 1 || !cached[0].Complete {
		t.Fatal(cached, err)
	}
	if cached[0].Items[0].Observation.Status != "not_probed" {
		t.Fatal(cached)
	}
	live.syncer.Store = store
	if err = refreshBroadcasters(ctx, store, live, upstream.BroadcasterClient(), chain); err == nil {
		t.Fatal("offline registry accepted")
	}
	cached, err = cachedBroadcasters(ctx, store, []config.Chain{chain})
	if err != nil || cached[0].Status != "unavailable" || len(cached[0].Items) != 1 {
		t.Fatal(cached, err)
	}
}
