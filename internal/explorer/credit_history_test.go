package explorer

import (
	"context"
	"fmt"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	p "github.com/invisi-labs/invisi-explorer/internal/wire"
	"net/url"
	"strings"
	"testing"
)

func TestGrantHistoryFourActionsAndScopedAmounts(t *testing.T) {
	e, s, scope, base := fixture(t)
	ctx := context.Background()
	owner := base[0].Funding[0].Payer
	token := base[0].TokenAddress
	ops := []model.Operation{}
	for i, kind := range []string{"createCredit", "updateCredit", "creditDraw", "revokeCredit"} {
		op := base[0]
		op.Kind = kind
		op.BlockNumber = fmt.Sprint(i + 3)
		op.BlockHash = string(p.UintField(uint64(i + 3)))
		op.TxHash = string(p.UintField(uint64(i + 30)))
		op.Timestamp = fmt.Sprint(i + 103)
		op.GrantRefs = []model.GrantRef{{Payer: owner, GrantID: "77"}}
		op.CreditSlotIndexed = true
		op.NoteOutputs = nil
		op.Funding = nil
		op.CreditOutputs = nil
		if kind == "createCredit" {
			op.Funding = []model.Funding{{Payer: owner, TokenAddress: token, Amount: "10"}}
			op.NoteOutputs = base[0].NoteOutputs
		}
		if kind == "creditDraw" {
			op.Funding = []model.Funding{{Payer: owner, TokenAddress: token, Amount: "7"}}
			op.NoteOutputs = base[0].NoteOutputs
			op.GrantRefs = append(op.GrantRefs, model.GrantRef{Payer: owner, GrantID: "88"})
		}
		ops = append(ops, op)
	}
	if err := s.Stage(ctx, scope, ops, "", "6"); err != nil {
		t.Fatal(err)
	}
	if err := s.Publish(ctx, scope, ops[3].Anchor()); err != nil {
		t.Fatal(err)
	}
	path := "/grants/97:" + scope.ProtocolAddress + ":" + token + ":" + owner + ":77"
	q := url.Values{"limit": {"2"}}
	seen := map[string]bool{}
	rows := []map[string]any{}
	for {
		value, err := e.Public(ctx, path, q, nil)
		if err != nil {
			t.Fatal(err)
		}
		result := value.(map[string]any)
		if result["historyStatus"] != "complete_at_indexed_head" {
			t.Fatal(result)
		}
		page := result["activities"].(Page)
		for _, item := range page.Items {
			m := item.(map[string]any)
			seen[m["kind"].(string)] = true
			rows = append(rows, m)
		}
		if result["nextCursor"] != page.NextCursor {
			t.Fatal("missing grant cursor")
		}
		if page.NextCursor == "" {
			break
		}
		q.Set("cursor", page.NextCursor)
	}
	if len(seen) != 4 || len(rows) != 4 {
		t.Fatalf("history: %+v", rows)
	}
	for _, row := range rows {
		if row["kind"] == "createCredit" {
			actions := row["actions"].([]string)
			if len(actions) != 2 || actions[1] != "creditDraw" {
				t.Fatal(row)
			}
			if row["slotFundingAmount"] != "10" {
				t.Fatal(row)
			}
		}
		if row["kind"] == "creditDraw" && row["slotFundingAmount"] != nil {
			t.Fatal("attributed entire payer amount to each grant", row)
		}
	}
	if _, err := e.Public(ctx, strings.Replace(path, ":77", ":99", 1), nil, nil); err == nil {
		t.Fatal("unrelated slot matched")
	}
}

func TestGrantHistoryOrdersSameBlockByLogAcrossPages(t *testing.T) {
	e, s, scope, base := fixture(t)
	ctx := context.Background()
	ops := []model.Operation{}
	for i, kind := range []string{"createCredit", "updateCredit", "revokeCredit"} {
		op := base[0]
		op.Kind = kind
		op.BlockNumber = "3"
		op.BlockHash = string(p.UintField(3))
		op.Timestamp = "103"
		op.TxHash = string(p.UintField(uint64(90 - i)))
		op.LogIndex = fmt.Sprint(i + 1)
		op.CreditSlotIndexed = true
		op.GrantRefs = []model.GrantRef{{Payer: base[0].Funding[0].Payer, GrantID: "77"}}
		ops = append(ops, op)
	}
	if err := s.Stage(ctx, scope, ops, "", "3"); err != nil {
		t.Fatal(err)
	}
	if err := s.Publish(ctx, scope, ops[0].Anchor()); err != nil {
		t.Fatal(err)
	}
	path := "/grants/97:" + scope.ProtocolAddress + ":" + base[0].TokenAddress + ":" + base[0].Funding[0].Payer + ":77"
	q := url.Values{"limit": {"1"}}
	for _, want := range []string{"revokeCredit", "updateCredit", "createCredit"} {
		v, err := e.Public(ctx, path, q, nil)
		if err != nil {
			t.Fatal(err)
		}
		page := v.(map[string]any)["activities"].(Page)
		if len(page.Items) != 1 || page.Items[0].(map[string]any)["kind"] != want {
			t.Fatalf("want %s got %+v", want, page.Items)
		}
		q.Set("cursor", page.NextCursor)
	}
}
