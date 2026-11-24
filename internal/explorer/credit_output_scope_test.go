package explorer

import (
	"github.com/invisi-labs/invisi-explorer/internal/model"
	p "github.com/invisi-labs/invisi-explorer/internal/wire"
	"testing"
)

func TestCreditActivityMatchesOutputByOwnerAndGrantID(t *testing.T) {
	owner := "0x0000000000000000000000000000000000000001"
	op := model.Operation{Kind: "creditDraw", GrantRefs: []model.GrantRef{{Payer: owner, GrantID: "77"}, {Payer: owner, GrantID: "88"}}, CreditOutputs: []model.CreditOutput{{CreditCommitment: p.UintField(10)}, {CreditCommitment: p.UintField(20)}}}
	for i, id := range []string{"77", "88"} {
		out := creditActivity(op, owner, id)
		credits := out["creditOutputs"].([]map[string]any)
		if len(credits) != 1 || stringValue(credits[0]["creditCommitment"]) != string(op.CreditOutputs[i].CreditCommitment) {
			t.Fatal(out)
		}
	}
	out := creditActivity(op, "0x0000000000000000000000000000000000000002", "77")
	if len(out["creditOutputs"].([]map[string]any)) != 0 {
		t.Fatal("unrelated owner output leaked", out)
	}
	op.CreditOutputs = op.CreditOutputs[:1]
	out = creditActivity(op, owner, "77")
	if len(out["creditOutputs"].([]map[string]any)) != 0 {
		t.Fatal("mismatched arrays must not be assigned", out)
	}
}
