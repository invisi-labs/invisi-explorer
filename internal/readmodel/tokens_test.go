package readmodel

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/invisi-labs/invisi-explorer/internal/model"
	"github.com/invisi-labs/invisi-explorer/internal/wire"
)

func TestTokenDiscoveryAndPersistentCache(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "explorer.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	scope := wire.Scope{ChainID: "97", ProtocolAddress: "0x1111111111111111111111111111111111111111"}
	token := "0xABCDEF0000000000000000000000000000000001"
	funding := "0xabcdef0000000000000000000000000000000002"
	op := model.Operation{ChainID: "97", ProtocolAddress: scope.ProtocolAddress, TokenAddress: token, BlockNumber: "1", BlockHash: wire.ZeroHash, TxHash: wire.ZeroHash, OperationIndex: "0", Timestamp: "1", FundingEvidence: &model.FundingEvidence{Transfers: []model.AssetTransfer{{TokenAddress: funding}, {TokenAddress: "0x0000000000000000000000000000000000000000"}}}}
	if err = s.Stage(ctx, scope, []model.Operation{op}, "", "1"); err != nil {
		t.Fatal(err)
	}
	due, err := s.DueTokens(ctx, "97", 100, 8)
	if err != nil || len(due) != 0 {
		t.Fatalf("staged tokens visible: %v %v", due, err)
	}
	if err = s.Publish(ctx, scope, model.Anchor{BlockNumber: "1", BlockHash: wire.ZeroHash}); err != nil {
		t.Fatal(err)
	}
	due, err = s.DueTokens(ctx, "97", 100, 8)
	if err != nil || len(due) != 2 {
		t.Fatalf("discovery: %v %v", due, err)
	}
	decimals := uint8(18)
	if err = s.SaveTokenMetadata(ctx, TokenMetadata{ChainID: "97", Address: token, Symbol: "TEST", Decimals: &decimals}, 200); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	due, err = s.DueTokens(ctx, "97", 199, 8)
	if err != nil || len(due) != 1 {
		t.Fatalf("cache TTL lost: %v %v", due, err)
	}
	other, err := s.DueTokens(ctx, "1", 100, 8)
	if err != nil || len(other) != 0 {
		t.Fatal("chain cache collision", err)
	}
	// Failed refreshes retain the last successful metadata.
	if err = s.SaveTokenMetadata(ctx, TokenMetadata{ChainID: "97", Address: token}, 300); err != nil {
		t.Fatal(err)
	}
	list, err := s.TokenMetadata(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Symbol != "TEST" || list[0].Decimals == nil || *list[0].Decimals != 18 {
		t.Fatalf("cache lost: %#v", list)
	}
}

func TestExistingTokenDiscoveryBackfill(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "explorer.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	op := model.Operation{ChainID: "97", TokenAddress: "0x1111111111111111111111111111111111111111", Funding: []model.Funding{{TokenAddress: "0x2222222222222222222222222222222222222222"}}}
	b, _ := json.Marshal(op)
	if _, err = s.DB.Exec("INSERT INTO operations VALUES('old','97','core','1','hash','tx','0',1,?)", string(b)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec("DELETE FROM readmodel_versions WHERE id='token_metadata_discovery'"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	due, err := s.DueTokens(ctx, "97", 1, 8)
	if err != nil || len(due) != 2 {
		t.Fatalf("legacy tokens missing: %v %v", due, err)
	}
}
