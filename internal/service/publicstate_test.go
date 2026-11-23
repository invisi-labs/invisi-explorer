package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/publicchain"
	"github.com/invisi-labs/invisi-explorer/internal/upstream"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBroadcasterQueryRetainsFullCanonicalBlock(t *testing.T) {
	protocol := "0x" + strings.Repeat("1", 40)
	hash := "0x" + strings.Repeat("2", 64)
	parent := "0x" + strings.Repeat("3", 64)
	address := "0x" + strings.Repeat("4", 40)
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if e := json.NewDecoder(r.Body).Decode(&q); e != nil {
			t.Fatal(e)
		}
		var result any
		switch q.Method {
		case "invisi_getDeployment":
			result = map[string]any{"deployment": map[string]any{"chainId": "97", "protocol": map[string]any{"address": protocol, "runtimeCodeHash": hash, "fromBlock": 1}, "broadcasterRegistry": map[string]any{"address": address, "runtimeCodeHash": hash, "fromBlock": 1}}}
		case "invisi_readChainState":
			result = map[string]any{"headerHash": hash, "header": map[string]any{"number": "0xa", "parentHash": parent, "timestamp": "0x64"}}
		case "invisi_getBroadcasters":
			called = true
			var params struct {
				Block upstream.Block `json:"block"`
				Limit int            `json:"limit"`
			}
			json.Unmarshal(q.Params[0], &params)
			if params.Limit != 100 {
				t.Errorf("upstream limit %d exceeds bound", params.Limit)
			}
			if params.Block.Number != 10 || params.Block.Hash != hash || params.Block.ParentHash != parent || params.Block.Timestamp != 100 {
				t.Errorf("incomplete anchored query %+v", params.Block)
			}
			result = map[string]any{"block": params.Block, "broadcasters": []any{map[string]any{"operator": address, "endpoint": "https://example.invalid", "quoteSigner": address, "feeRecipient": address, "enabled": true, "block": params.Block}}}
		default:
			t.Errorf("unexpected method %s", q.Method)
		}
		b, _ := json.Marshal(result)
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":%s}`, b)
	}))
	defer server.Close()
	p := &publicState{syncer: &upstream.Syncer{Client: server.Client()}, chains: []config.Chain{{ChainID: "97", ProtocolAddress: protocol, RPCURL: server.URL}}}
	v, e := p.Broadcasters(context.Background(), "97", protocol, publicchain.DirectoryQuery{Anchor: publicchain.Anchor{BlockNumber: "10", BlockHash: hash}, Limit: 200})
	if e != nil || !called || len(v.Items) != 1 {
		t.Fatalf("directory unavailable %+v %v", v, e)
	}
}

func TestTreeUsesAnchoredIndexedRootWithoutInventingActiveState(t *testing.T) {
	protocol := "0x" + strings.Repeat("1", 40)
	hash := "0x" + strings.Repeat("2", 64)
	root := "0x" + strings.Repeat("0", 63) + "1"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			t.Error(err)
			return
		}
		var result any
		switch q.Method {
		case "invisi_readChainState":
			result = map[string]any{"headerHash": hash, "header": map[string]any{"number": "0xa", "parentHash": hash, "timestamp": "0x64"}}
		case "invisi_getRoots":
			result = []any{
				map[string]any{"treeId": 1, "leafCount": 5, "root": root, "block": map[string]any{"number": 9}},
				map[string]any{"treeId": 1, "leafCount": 6, "root": root, "block": map[string]any{"number": 11}},
			}
		default:
			t.Errorf("unexpected method %s", q.Method)
		}
		data, _ := json.Marshal(result)
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":%s}`, data)
	}))
	defer server.Close()
	p := &publicState{syncer: &upstream.Syncer{Client: server.Client()}, chains: []config.Chain{{ChainID: "97", ProtocolAddress: protocol, RPCURL: server.URL}}}
	value, err := p.Tree(context.Background(), "97", protocol, "1", publicchain.Anchor{BlockNumber: "10", BlockHash: hash})
	if err != nil || value.LeafCount != "5" || value.Root != root {
		t.Fatalf("bad anchored root: %+v %v", value, err)
	}
	data, _ := json.Marshal(value)
	for _, field := range []string{`"isActive"`, `"activeTreeId"`, `"treeCount"`, `"rootHistoryKnown"`} {
		if strings.Contains(string(data), field) {
			t.Errorf("invented unavailable contract state %s: %s", field, data)
		}
	}
}
