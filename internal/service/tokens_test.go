package service

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/explorer"
	"github.com/invisi-labs/invisi-explorer/internal/readmodel"
)

func TestMetadataMergesPersistentTokensWithConfiguredPriority(t *testing.T) {
	ctx := context.Background()
	store, err := readmodel.Open(filepath.Join(t.TempDir(), "explorer.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	address := "0xabcdef0000000000000000000000000000000001"
	other := "0xabcdef0000000000000000000000000000000002"
	d := uint8(18)
	for _, v := range []readmodel.TokenMetadata{{ChainID: "97", Address: address, Symbol: "AUTO", Decimals: &d}, {ChainID: "97", Address: other, Decimals: &d}} {
		if err = store.SaveTokenMetadata(ctx, v, 100); err != nil {
			t.Fatal(err)
		}
	}
	c := config.Config{Chains: []config.Chain{{ChainID: "97"}}, Tokens: []config.Token{{ChainID: "97", Address: address, Symbol: "PINNED", Decimals: 6}}}
	e := explorer.New(store.DB, explorer.Options{MetadataProvider: func(ctx context.Context) (any, error) { return cachedMetadata(ctx, store, c) }})
	value, err := e.Public(ctx, "/metadata", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(value)
	var response struct {
		Tokens []readmodel.TokenMetadata `json:"tokens"`
	}
	if err = json.Unmarshal(encoded, &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Tokens) != 2 {
		t.Fatalf("tokens: %s", encoded)
	}
	for _, v := range response.Tokens {
		if v.Address == address && (v.Symbol != "PINNED" || *v.Decimals != 6) {
			t.Fatal("configured metadata overwritten")
		}
		if v.Address == other && (v.Symbol != "" || *v.Decimals != 18) {
			t.Fatal("partial metadata not exposed")
		}
	}
}
