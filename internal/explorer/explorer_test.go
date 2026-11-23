package explorer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/invisi-labs/invisi-explorer/internal/api"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	"github.com/invisi-labs/invisi-explorer/internal/readmodel"
	p "github.com/invisi-labs/invisi-explorer/internal/wire"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) (*Explorer, *readmodel.Store, p.Scope, []model.Operation) {
	t.Helper()
	s, err := readmodel.Open(filepath.Join(t.TempDir(), "explorer.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	scope := p.Scope{ChainID: "97", ProtocolAddress: "0x0000000000000000000000000000000000000001"}
	ops := []model.Operation{}
	for i := 1; i <= 2; i++ {
		ops = append(ops, model.Operation{ChainID: scope.ChainID, ProtocolAddress: scope.ProtocolAddress, BlockNumber: fmt.Sprint(i), BlockHash: fmt.Sprintf("0x%064x", i), TxHash: fmt.Sprintf("0x%064x", i+10), OperationIndex: "0", LogIndex: "0", Timestamp: fmt.Sprint(100 + i), Kind: "shield", TokenAddress: "0x0000000000000000000000000000000000000002", Success: true, Canonical: true, FeeAmount: "0", WithdrawalAmount: "0", NoteOutputs: []model.NoteOutput{{NoteCommitment: p.UintField(uint64(100 + i)), TreeID: "1", LeafIndex: fmt.Sprint(i), Amount: "7"}}, CreditOutputs: []model.CreditOutput{{CreditCommitment: p.UintField(uint64(200 + i)), TreeID: "1", LeafIndex: fmt.Sprint(i + 2)}}, NoteNullifiers: []p.Field{p.UintField(uint64(300 + i))}, Funding: []model.Funding{{Payer: "0x0000000000000000000000000000000000000003", TokenAddress: "0x0000000000000000000000000000000000000002", Amount: "7"}}, GrantRefs: []model.GrantRef{{PayerIndex: "0", GrantID: "1"}}})
	}
	ctx := context.Background()
	if err = s.Stage(ctx, scope, ops, "", "2"); err != nil {
		t.Fatal(err)
	}
	if err = s.Publish(ctx, scope, model.Anchor{BlockNumber: "2", BlockHash: ops[1].BlockHash}); err != nil {
		t.Fatal(err)
	}
	e := New(s.DB, Options{Dialect: "sqlite", CursorKey: []byte(strings.Repeat("k", 32)), Scopes: []p.Scope{scope}, Metadata: map[string]any{"networks": []any{}}, Recognition: func(context.Context, model.Operation) (any, error) {
		return []any{map[string]any{"status": "provider_unavailable", "reason": "PROVENANCE_UNAVAILABLE"}}, nil
	}, Roots: func(context.Context) (any, error) { return map[string]any{"status": "available"}, nil }, Diagnostics: func(context.Context) (any, error) { return map[string]any{"items": []any{}}, nil }})
	return e, s, scope, ops
}
func TestPublicRoutesKeepDomainsAndAmounts(t *testing.T) {
	e, _, scope, ops := fixture(t)
	paths := []string{"/metadata", "/overview", "/transactions", "/transactions/" + ops[0].TxHash, "/notes", "/notes/" + string(ops[0].NoteOutputs[0].NoteCommitment), "/notes/" + string(ops[0].NoteOutputs[0].NoteCommitment) + "/occurrences", "/credit-states", "/credit-states/" + string(ops[0].CreditOutputs[0].CreditCommitment), "/nullifiers", "/nullifiers/" + string(ops[0].NoteNullifiers[0]), "/commitments", "/grants", "/grants/97:" + scope.ProtocolAddress + ":" + ops[0].TokenAddress + ":" + ops[0].Funding[0].Payer + ":1", "/addresses/" + ops[0].Funding[0].Payer, "/trees", "/trees/protocol:1", "/roots", "/diagnostics"}
	for _, path := range paths {
		if _, err := e.Public(context.Background(), path, url.Values{}, nil); err != nil {
			t.Errorf("%s: %v", path, err)
		}
	}
	v, err := e.Public(context.Background(), "/credit-states", url.Values{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(v)
	if strings.Contains(string(b), `"amount"`) || strings.Contains(string(b), `"noteCommitment"`) {
		t.Fatalf("credit leaked note inference %s", b)
	}
	if _, err = e.Public(context.Background(), "/search", nil, json.RawMessage(`{"query":"`+ops[0].TxHash+`"}`)); err != nil {
		t.Fatal(err)
	}
}
func TestCursorTamperingConflictAndReorg(t *testing.T) {
	e, s, scope, ops := fixture(t)
	q := url.Values{"limit": {"1"}}
	v, err := e.Public(context.Background(), "/transactions", q, nil)
	if err != nil {
		t.Fatal(err)
	}
	page := v.(Page)
	if page.NextCursor == "" {
		t.Fatal("no continuation")
	}
	q.Set("cursor", page.NextCursor)
	v, err = e.Public(context.Background(), "/transactions", q, nil)
	if err != nil || len(v.(Page).Items) != 1 {
		t.Fatal(err)
	}
	q.Set("cursor", page.NextCursor+"x")
	_, err = e.Public(context.Background(), "/transactions", q, nil)
	requireCode(t, err, "INVALID_PARAMS")
	q.Set("cursor", page.NextCursor)
	if err = s.SaveRecognition(context.Background(), ops[0], []any{map[string]any{"status": "recognized"}}, 1); err != nil {
		t.Fatal(err)
	}
	_, err = e.Public(context.Background(), "/transactions", q, nil)
	requireCode(t, err, "CONFLICT")
	q.Del("cursor")
	v, err = e.Public(context.Background(), "/transactions", q, nil)
	if err != nil {
		t.Fatal(err)
	}
	q.Set("cursor", v.(Page).NextCursor)
	if err = s.Rollback(context.Background(), scope, 0); err != nil {
		t.Fatal(err)
	}
	_, err = e.Public(context.Background(), "/transactions", q, nil)
	if err == nil {
		t.Fatal("reorg cursor accepted")
	}
}
func requireCode(t *testing.T, err error, want string) {
	t.Helper()
	var v *api.Error
	if !errors.As(err, &v) || v.Code != want {
		t.Fatalf("got %v want %s", err, want)
	}
}
