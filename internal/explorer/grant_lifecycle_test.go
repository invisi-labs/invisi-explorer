package explorer

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/invisi-labs/invisi-explorer/internal/model"
	p "github.com/invisi-labs/invisi-explorer/internal/wire"
)

func TestGrantLifecycleUsesAllHistoryOutsideCurrentPage(t *testing.T) {
	e, s, scope, base := fixture(t)
	ctx := context.Background()
	owner := base[0].Funding[0].Payer
	ops := []model.Operation{}
	for i, kind := range []string{"createCredit", "updateCredit", "revokeCredit", "revokeCredit"} {
		op := base[0]
		op.Kind = kind
		op.BlockNumber = fmt.Sprint(i + 3)
		op.BlockHash = string(p.UintField(uint64(i + 3)))
		op.TxHash = string(p.UintField(uint64(i + 30)))
		op.Timestamp = fmt.Sprint(i + 103)
		op.CreditSlotIndexed = true
		id := "77"
		if i == 3 {
			id = "88"
		}
		op.GrantRefs = []model.GrantRef{{Payer: owner, GrantID: id}}
		op.NoteOutputs = nil
		op.Funding = nil
		ops = append(ops, op)
	}
	if err := s.Stage(ctx, scope, ops, "", "6"); err != nil {
		t.Fatal(err)
	}
	if err := s.Publish(ctx, scope, ops[3].Anchor()); err != nil {
		t.Fatal(err)
	}
	path := "/grants/97:" + scope.ProtocolAddress + ":" + base[0].TokenAddress + ":" + owner + ":77"
	q := url.Values{"limit": {"1"}}
	for i := 0; i < 3; i++ {
		v, err := e.Public(ctx, path, q, nil)
		if err != nil {
			t.Fatal(err)
		}
		d := v.(map[string]any)
		if d["historyCount"] != int64(3) {
			t.Fatalf("history count changed with page: %+v", d)
		}
		if d["lifecycleStatus"] != "revoked_after_creation" || d["creationStatus"] != "created" {
			t.Fatalf("creation lost on page %d: %+v", i, d)
		}
		creation := d["creationTransaction"].(map[string]any)
		if creation["txHash"] != ops[0].TxHash {
			t.Fatal(creation)
		}
		page := d["activities"].(Page)
		if i == 0 && page.Items[0].(map[string]any)["stateEffect"] != "revoked_after_creation" {
			t.Fatal(page.Items)
		}
		q.Set("cursor", page.NextCursor)
	}
	list, err := e.Public(ctx, "/grants", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range list.(Page).Items {
		row := item.(map[string]any)
		switch row["grantId"] {
		case "77":
			if row["lifecycleStatus"] != "revoked_after_creation" {
				t.Fatal(row)
			}
		case "88":
			if row["lifecycleStatus"] != "pre_revoked" || row["creationStatus"] != "not_created" {
				t.Fatal(row)
			}
		}
	}
	v, err := e.Public(ctx, strings.TrimSuffix(path, "77")+"88", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if v.(map[string]any)["activities"].(Page).Items[0].(map[string]any)["stateEffect"] != "pre_revoked" {
		t.Fatal(v)
	}
}

func TestGrantLifecycleDoesNotTreatMissingEvidenceAsPreRevocation(t *testing.T) {
	for _, mode := range []string{"pending", "active-history", "other-identity", "failed-create", "orphan-create"} {
		t.Run(mode, func(t *testing.T) {
			e, s, scope, base := fixture(t)
			ctx := context.Background()
			owner := base[0].Funding[0].Payer
			revoke := base[0]
			revoke.Kind = "revokeCredit"
			revoke.BlockNumber = "4"
			revoke.BlockHash = string(p.UintField(4))
			revoke.TxHash = string(p.UintField(40))
			revoke.Timestamp = "104"
			revoke.CreditSlotIndexed = true
			revoke.GrantRefs = []model.GrantRef{{Payer: owner, GrantID: "77"}}
			extra := revoke
			extra.Kind = "createCredit"
			extra.BlockNumber = "3"
			extra.BlockHash = string(p.UintField(3))
			extra.TxHash = string(p.UintField(30))
			extra.Timestamp = "103"
			switch mode {
			case "pending":
				extra.CreditSlotIndexed = false
				extra.GrantRefs = nil
			case "active-history":
				extra.Kind = "creditDraw"
			case "other-identity":
				extra.GrantRefs = []model.GrantRef{{Payer: "0x0000000000000000000000000000000000000004", GrantID: "77"}}
			case "failed-create":
				extra.Success = false
			}
			if err := s.Stage(ctx, scope, []model.Operation{extra, revoke}, "", "4"); err != nil {
				t.Fatal(err)
			}
			if err := s.Publish(ctx, scope, revoke.Anchor()); err != nil {
				t.Fatal(err)
			}
			if mode == "orphan-create" {
				if _, err := s.DB.Exec("UPDATE operations SET canonical=0 WHERE id=?", extra.ID()); err != nil {
					t.Fatal(err)
				}
			}
			path := "/grants/97:" + scope.ProtocolAddress + ":" + revoke.TokenAddress + ":" + owner + ":77"
			v, err := e.Public(ctx, path, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			d := v.(map[string]any)
			if mode == "pending" || mode == "active-history" {
				if d["lifecycleStatus"] != "revocation_unconfirmed" || d["creationStatus"] != "unconfirmed" {
					t.Fatal(d)
				}
			} else if d["lifecycleStatus"] != "pre_revoked" {
				t.Fatal(d)
			}
		})
	}
}
