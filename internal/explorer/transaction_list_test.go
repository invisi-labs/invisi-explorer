package explorer

import (
	"context"
	"errors"
	"fmt"
	"github.com/invisi-labs/invisi-explorer/internal/api"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	p "github.com/invisi-labs/invisi-explorer/internal/wire"
	"net/url"
	"testing"
)

func TestTransactionListGroupsBeforePagination(t *testing.T) {
	e, s, scope, base := fixture(t)
	ctx := context.Background()
	for _, op := range base {
		op.Timestamp = "103"
		if err := s.Enrich(ctx, op); err != nil {
			t.Fatal(err)
		}
	}
	ops := []model.Operation{}
	for i := 0; i < 3; i++ {
		op := base[0]
		op.BlockNumber = "3"
		op.BlockHash = string(p.UintField(3))
		op.Timestamp = "103"
		op.TxHash = string(p.UintField(100))
		op.OperationIndex = fmt.Sprint([]int{0, 2, 10}[i])
		op.Kind = "transact"
		op.CreditOutputs = nil
		op.GrantRefs = nil
		if i == 1 {
			op.TokenAddress = "0x0000000000000000000000000000000000000004"
			op.Funding = []model.Funding{{Payer: base[0].Funding[0].Payer, TokenAddress: op.TokenAddress, Amount: "11"}}
		}
		if i == 2 {
			op.Success = false
		}
		ops = append(ops, op)
	}
	if err := s.Stage(ctx, scope, ops, "", "3"); err != nil {
		t.Fatal(err)
	}
	if err := s.Publish(ctx, scope, ops[0].Anchor()); err != nil {
		t.Fatal(err)
	}
	q := url.Values{"limit": {"1"}}
	seen := map[string]bool{}
	for pageNo := 0; pageNo < 3; pageNo++ {
		v, err := e.Public(ctx, "/transactions", q, nil)
		if err != nil {
			t.Fatal(err)
		}
		page := v.(Page)
		if page.Total != "3" {
			t.Fatalf("operation count used as tx total: %s", page.Total)
		}
		if len(page.Items) != 1 {
			t.Fatal(page)
		}
		row := page.Items[0].(map[string]any)
		hash := row["txHash"].(string)
		if seen[hash] {
			t.Fatal("transaction split across pages")
		}
		seen[hash] = true
		if hash == ops[0].TxHash {
			if row["operationCount"] != "3" || row["executionStatus"] != "partial_success" || row["kind"] != "aggregate" {
				t.Fatal(row)
			}
			amounts := row["publicDepositAmounts"].([]map[string]any)
			if len(amounts) != 2 || amounts[0]["amount"] != "7" || amounts[1]["amount"] != "11" {
				t.Fatalf("mixed token/failed amount: %+v", amounts)
			}
			summaries := row["operationSummaries"].([]map[string]any)
			for i, want := range []string{"0", "2", "10"} {
				if summaries[i]["operationIndex"] != want {
					t.Fatal(summaries)
				}
			}
			detail, err := e.Public(ctx, "/transactions/"+hash, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			d := detail.(map[string]any)
			if len(d["operations"].([]any)) != 3 || d["operationCount"] != "3" {
				t.Fatal(d)
			}
		}
		if pageNo == 2 && page.NextCursor != "" {
			t.Fatal("extra operation page")
		}
		q.Set("cursor", page.NextCursor)
	}
	overview, err := e.Public(ctx, "/overview", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if overview.(map[string]any)["transactions"] != "3" {
		t.Fatal(overview)
	}
	// Filtering finds the whole transaction; it must not silently drop other members.
	filtered, err := e.Public(ctx, "/transactions", url.Values{"tokenAddress": {ops[1].TokenAddress}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	detail, err := e.Public(ctx, "/transactions/"+ops[0].TxHash, url.Values{"tokenAddress": {ops[1].TokenAddress}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.(map[string]any)["operations"].([]any)) != 3 {
		t.Fatal("detail discarded filtered-out members")
	}
	f := filtered.(Page)
	if f.Total != "1" || f.Items[0].(map[string]any)["operationCount"] != "3" {
		t.Fatal(f)
	}
	matched, err := e.Public(ctx, "/transactions", url.Values{"kind": {"transact"}}, nil)
	if err != nil || matched.(Page).Total != "1" {
		t.Fatalf("kind filter: %v %v", matched, err)
	}
	v, err := e.Public(ctx, "/transactions", url.Values{"limit": {"1"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Rollback(ctx, scope, 2); err != nil {
		t.Fatal(err)
	}
	_, err = e.Public(ctx, "/transactions", url.Values{"limit": {"1"}, "cursor": {v.(Page).NextCursor}}, nil)
	var public *api.Error
	if !errors.As(err, &public) || public.Code != "CHAIN_REORGED" {
		t.Fatalf("group cursor crossed reorg: %v", err)
	}
}

func TestTransactionListRestartsLegacyOperationCursor(t *testing.T) {
	e, _, _, ops := fixture(t)
	ctx := context.Background()
	q := queryOptions()
	q.limit = 1
	snapshot, err := e.snapshot(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	token, err := e.cursorEncode(cursor{Version: 1, Route: "transactions", Filter: filterDigest("transactions", q), Snapshot: snapshot, After: position{ID: ops[0].ID(), Time: pad(ops[0].Timestamp)}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.Public(ctx, "/transactions", url.Values{"limit": {"1"}, "cursor": {token}}, nil)
	var public *api.Error
	if !errors.As(err, &public) || public.Code != "CONFLICT" {
		t.Fatalf("old operation cursor must restart: %v", err)
	}
}

func TestTransactionGroupsKeepDeploymentIdentity(t *testing.T) {
	e, s, scope, base := fixture(t)
	ctx := context.Background()
	other := scope
	other.ChainID = "1"
	op := base[0]
	op.ChainID = "1"
	if err := s.Stage(ctx, other, []model.Operation{op}, "", "1"); err != nil {
		t.Fatal(err)
	}
	if err := s.Publish(ctx, other, op.Anchor()); err != nil {
		t.Fatal(err)
	}
	options := e.options
	options.Scopes = append(options.Scopes, other)
	e = New(s.DB, options)
	v, err := e.Public(ctx, "/transactions", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if v.(Page).Total != "3" {
		t.Fatal("same hash across deployments merged", v)
	}
	v, err = e.Public(ctx, "/transactions", url.Values{"chainId": {"1"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	page := v.(Page)
	if page.Total != "1" || page.Items[0].(map[string]any)["chainId"] != "1" {
		t.Fatal(page)
	}
}
