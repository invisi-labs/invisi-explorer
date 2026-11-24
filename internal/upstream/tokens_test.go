package upstream

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/readmodel"
)

func abiText(text string) string {
	return "0x" + fmt.Sprintf("%064x%064x", 32, len(text)) + hex.EncodeToString([]byte(text)) + strings.Repeat("0", (32-len(text)%32)%32*2)
}
func TestTokenABIDecoding(t *testing.T) {
	for _, encoded := range []string{abiText("iUSD"), "0x" + hex.EncodeToString([]byte("iUSD")) + strings.Repeat("00", 28)} {
		if got := decodeTokenSymbol(encoded); got != "iUSD" {
			t.Fatalf("symbol %q", got)
		}
	}
	for _, encoded := range []string{"0x", abiText(""), abiText("x\nInjected"), abiText("\u202EUSD"), abiText(strings.Repeat("x", 65)), "0x" + strings.Repeat("f", 64), abiText("bad\x00symbol")} {
		if got := decodeTokenSymbol(encoded); got != "" {
			t.Fatalf("accepted unsafe symbol %q", got)
		}
	}
	for _, n := range []uint8{0, 6, 18, 255} {
		d := decodeTokenDecimals(fmt.Sprintf("0x%064x", n))
		if d == nil || *d != n {
			t.Fatal("valid precision rejected", n)
		}
	}
	for _, bad := range []string{"0x", "0x12", fmt.Sprintf("0x%064x", 256), "0x" + strings.Repeat("f", 64)} {
		if decodeTokenDecimals(bad) != nil {
			t.Fatal("invalid precision accepted", bad)
		}
	}
}

func TestTokenRefreshTTLPartialFailureAndOverride(t *testing.T) {
	ctx := context.Background()
	store, err := readmodel.Open(filepath.Join(t.TempDir(), "explorer.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	address := "0x2222222222222222222222222222222222222222"
	if err = store.SaveTokenMetadata(ctx, readmodel.TokenMetadata{ChainID: "97", Address: address}, 0); err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int64
	var brokenDecimals atomic.Bool
	var wrongChain atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		var q struct {
			Params []struct{ Kind, Block, Address, Data string }
		}
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			t.Error(err)
			return
		}
		p := q.Params[0]
		chain := "97"
		if wrongChain.Load() {
			chain = "1"
		}
		result := map[string]any{"deployment": map[string]any{"chainId": chain, "protocol": map[string]any{"address": "0x1111111111111111111111111111111111111111"}}, "headerHash": "0x" + strings.Repeat("a", 64), "header": map[string]any{"number": "0x2a"}}
		if p.Kind == "call" {
			if p.Address != address || p.Block != "0x2a" {
				t.Errorf("unbound metadata call: %+v", p)
			}
			switch p.Data {
			case "0x95d89b41":
				result["data"] = abiText("AUTO")
			case "0x313ce567":
				result["data"] = fmt.Sprintf("0x%064x", 18)
				if brokenDecimals.Load() {
					result["data"] = "0x"
				}
			default:
				t.Errorf("unexpected selector %s", p.Data)
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
	}))
	defer server.Close()
	syncer := &Syncer{Store: store, Client: server.Client()}
	chain := config.Chain{ChainID: "97", ProtocolAddress: "0x1111111111111111111111111111111111111111", RPCURL: server.URL}
	now := time.Unix(1000, 0)
	brokenDecimals.Store(true)
	if err = syncer.RefreshTokens(ctx, chain, nil, now); err != nil {
		t.Fatal(err)
	}
	tokens, _ := store.TokenMetadata(ctx)
	if len(tokens) != 1 || tokens[0].Symbol != "AUTO" || tokens[0].Decimals != nil {
		t.Fatalf("unknown decimals became known: %+v", tokens)
	}
	count := requests.Load()
	syncer.RefreshTokens(ctx, chain, nil, now.Add(time.Minute))
	if requests.Load() != count {
		t.Fatal("negative cache ignored")
	}
	brokenDecimals.Store(false)
	if err = syncer.RefreshTokens(ctx, chain, nil, now.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	tokens, _ = store.TokenMetadata(ctx)
	if tokens[0].Decimals == nil || *tokens[0].Decimals != 18 {
		t.Fatal("retry failed")
	}
	count = requests.Load()
	syncer.RefreshTokens(ctx, chain, nil, now.Add(23*time.Hour))
	if requests.Load() != count {
		t.Fatal("success cache ignored")
	}
	// Wrong-chain metadata must not replace cached values.
	wrongChain.Store(true)
	if _, err = syncer.ReadTokenMetadata(ctx, chain, address); err == nil {
		t.Fatal("wrong chain accepted")
	}
	// Configured metadata takes priority without a chain request.
	count = requests.Load()
	explicit := []config.Token{{ChainID: "97", Address: address, Symbol: "PINNED", Decimals: 6}}
	if err = syncer.RefreshTokens(ctx, chain, explicit, now.Add(25*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != count {
		t.Fatal("configured metadata fetched remotely")
	}
	tokens, _ = store.TokenMetadata(ctx)
	if tokens[0].Symbol != "AUTO" || *tokens[0].Decimals != 18 {
		t.Fatal("configured override contaminated automatic metadata cache")
	}
}

func TestConfiguredTokensDoNotStarveAutomaticDiscovery(t *testing.T) {
	ctx := context.Background()
	store, err := readmodel.Open(filepath.Join(t.TempDir(), "explorer.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	configured := []config.Token{}
	for i := 1; i <= 9; i++ {
		address := fmt.Sprintf("0x%040x", i)
		if err = store.SaveTokenMetadata(ctx, readmodel.TokenMetadata{ChainID: "97", Address: address}, 0); err != nil {
			t.Fatal(err)
		}
		if i < 9 {
			configured = append(configured, config.Token{ChainID: "97", Address: address, Symbol: "PINNED", Decimals: 6})
		}
	}
	// Only the unconfigured ninth token is eligible, despite a batch limit of eight.
	excluded := []string{}
	for _, c := range configured {
		excluded = append(excluded, c.Address)
	}
	due, err := store.DueTokens(ctx, "97", 1000, 8, excluded...)
	if err != nil || len(due) != 1 || due[0].Address != fmt.Sprintf("0x%040x", 9) {
		t.Fatalf("discovery starved: %+v %v", due, err)
	}
}
