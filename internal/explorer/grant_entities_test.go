package explorer

import (
	"context"
	"fmt"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	p "github.com/invisi-labs/invisi-explorer/internal/wire"
	"net/url"
	"testing"
)

func TestGrantEntitiesDeduplicateBeforePagingKeepFullHistory(t *testing.T) {
	e, s, scope, base := fixture(t)
	ctx := context.Background()
	owner := base[0].Funding[0].Payer
	token := base[0].TokenAddress
	// Fixture already contains two actions for slot1. Add four for slot77,
	// including two in one block whose hash order differs from log order.
	ops := []model.Operation{}
	for i, kind := range []string{"createCredit", "creditDraw", "updateCredit", "revokeCredit"} {
		op := base[0]
		op.Kind = kind
		op.GrantRefs = []model.GrantRef{{Payer: owner, GrantID: "77"}}
		op.CreditSlotIndexed = true
		op.BlockNumber = fmt.Sprint(3 + i/2)
		op.BlockHash = string(p.UintField(uint64(3 + i/2)))
		op.Timestamp = fmt.Sprint(103 + i/2)
		op.LogIndex = fmt.Sprint(i + 1)
		op.TxHash = string(p.UintField(uint64(90 - i)))
		ops = append(ops, op)
	}
	if err := s.Stage(ctx, scope, ops, "", "4"); err != nil {
		t.Fatal(err)
	}
	if err := s.Publish(ctx, scope, ops[3].Anchor()); err != nil {
		t.Fatal(err)
	}
	q := url.Values{"limit": {"1"}}
	seen := map[string]bool{}
	for i := 0; i < 2; i++ {
		value, err := e.Public(ctx, "/grants", q, nil)
		if err != nil {
			t.Fatal(err)
		}
		page := value.(Page)
		if page.Total != "2" || len(page.Items) != 1 {
			t.Fatalf("grant actions leaked into entity pagination: %+v", page)
		}
		row := page.Items[0].(map[string]any)
		id := row["id"].(string)
		if seen[id] {
			t.Fatal("same grant on multiple pages")
		}
		seen[id] = true
		if row["grantId"] == "77" && (row["kind"] != "revokeCredit" || row["authorizationStatus"] != "revoked") {
			t.Fatal(row)
		}
		q.Set("cursor", page.NextCursor)
	}
	path := "/grants/97:" + scope.ProtocolAddress + ":" + token + ":" + owner + ":77"
	value, err := e.Public(ctx, path, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if value.(map[string]any)["activities"].(Page).Total != "4" {
		t.Fatal("entity grouping discarded historical actions")
	}
	overview, err := e.Public(ctx, "/overview", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if overview.(map[string]any)["grants"] != "2" {
		t.Fatal(overview)
	}
	// A cursor's snapshot must not pick a newer action beyond its original prefix.
	first, err := e.Public(ctx, "/grants", url.Values{"limit": {"1"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	op := ops[0]
	op.BlockNumber = "5"
	op.BlockHash = string(p.UintField(5))
	op.Timestamp = "105"
	op.TxHash = string(p.UintField(95))
	op.Kind = "updateCredit"
	op.GrantRefs = []model.GrantRef{{Payer: owner, GrantID: "1"}}
	if err := s.Stage(ctx, scope, []model.Operation{op}, "", "5"); err != nil {
		t.Fatal(err)
	}
	if err := s.Publish(ctx, scope, op.Anchor()); err != nil {
		t.Fatal(err)
	}
	second, err := e.Public(ctx, "/grants", url.Values{"limit": {"1"}, "cursor": {first.(Page).NextCursor}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if second.(Page).Items[0].(map[string]any)["blockNumber"] != "2" {
		t.Fatal("new action entered frozen grant list", second)
	}
}

func TestGrantEntityCountNormalizesAddressCase(t *testing.T) {
	e, s, scope, base := fixture(t)
	ctx := context.Background()
	ops := []model.Operation{}
	for i, token := range []string{"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"} {
		op := base[0]
		op.TokenAddress = token
		op.Kind = "updateCredit"
		op.BlockNumber = fmt.Sprint(3 + i)
		op.BlockHash = string(p.UintField(uint64(3 + i)))
		op.Timestamp = fmt.Sprint(103 + i)
		op.TxHash = string(p.UintField(uint64(90 + i)))
		op.GrantRefs = []model.GrantRef{{Payer: base[0].Funding[0].Payer, GrantID: "77"}}
		op.CreditSlotIndexed = true
		ops = append(ops, op)
	}
	if err := s.Stage(ctx, scope, ops, "", "4"); err != nil {
		t.Fatal(err)
	}
	if err := s.Publish(ctx, scope, ops[1].Anchor()); err != nil {
		t.Fatal(err)
	}
	v, err := e.Public(ctx, "/grants", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	total := v.(Page).Total
	overview, err := e.Public(ctx, "/overview", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if total != "2" || overview.(map[string]any)["grants"] != total {
		t.Fatalf("case-sensitive entity count: %s %+v", total, overview)
	}
}

func TestGrantEntitiesSeparateIdentityIgnoreFailedAndOrphanedActions(t *testing.T) {
	e, s, scope, base := fixture(t)
	ctx := context.Background()
	owner := base[0].Funding[0].Payer
	ops := []model.Operation{}
	for i := 0; i < 3; i++ {
		op := base[0]
		op.Kind = "createCredit"
		op.BlockNumber = "3"
		op.BlockHash = string(p.UintField(3))
		op.Timestamp = "103"
		op.TxHash = string(p.UintField(uint64(90 + i)))
		op.LogIndex = fmt.Sprint(i)
		op.CreditSlotIndexed = true
		payer := owner
		if i == 1 {
			payer = "0x0000000000000000000000000000000000000004"
		}
		if i == 2 {
			op.TokenAddress = "0x0000000000000000000000000000000000000005"
		}
		op.GrantRefs = []model.GrantRef{{Payer: payer, GrantID: "77"}}
		ops = append(ops, op)
	}
	if err := s.Stage(ctx, scope, ops, "", "3"); err != nil {
		t.Fatal(err)
	}
	if err := s.Publish(ctx, scope, ops[0].Anchor()); err != nil {
		t.Fatal(err)
	}
	failed := ops[0]
	failed.Kind = "revokeCredit"
	failed.Success = false
	failed.BlockNumber = "4"
	failed.BlockHash = string(p.UintField(4))
	failed.Timestamp = "104"
	failed.TxHash = string(p.UintField(94))
	if err := s.Stage(ctx, scope, []model.Operation{failed}, "", "4"); err != nil {
		t.Fatal(err)
	}
	if err := s.Publish(ctx, scope, failed.Anchor()); err != nil {
		t.Fatal(err)
	}
	orphan := failed
	orphan.Success = true
	orphan.BlockNumber = "5"
	orphan.BlockHash = string(p.UintField(5))
	orphan.Timestamp = "105"
	orphan.TxHash = string(p.UintField(95))
	if err := s.Stage(ctx, scope, []model.Operation{orphan}, "", "5"); err != nil {
		t.Fatal(err)
	}
	if err := s.Publish(ctx, scope, orphan.Anchor()); err != nil {
		t.Fatal(err)
	}
	if err := s.Rollback(ctx, scope, 4); err != nil {
		t.Fatal(err)
	}
	other := scope
	other.ProtocolAddress = "0x0000000000000000000000000000000000000006"
	op := ops[0]
	op.ProtocolAddress = other.ProtocolAddress
	if err := s.Stage(ctx, other, []model.Operation{op}, "", "3"); err != nil {
		t.Fatal(err)
	}
	if err := s.Publish(ctx, other, op.Anchor()); err != nil {
		t.Fatal(err)
	}
	options := e.options
	options.Scopes = append(options.Scopes, other)
	e = New(s.DB, options)
	v, err := e.Public(ctx, "/grants", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	page := v.(Page)
	if page.Total != "5" {
		t.Fatalf("grant identities merged: %+v", page)
	}
	ids := map[string]bool{}
	for _, item := range page.Items {
		row := item.(map[string]any)
		id := row["id"].(string)
		if ids[id] {
			t.Fatal("duplicate identity", id)
		}
		ids[id] = true
		if row["grantId"] == "77" && (row["kind"] != "createCredit" || row["authorizationStatus"] != "active") {
			t.Fatalf("failed/orphaned revocation changed status: %+v", row)
		}
	}
}
