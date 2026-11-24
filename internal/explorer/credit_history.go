package explorer

import (
	"github.com/invisi-labs/invisi-explorer/internal/model"
	p "github.com/invisi-labs/invisi-explorer/internal/wire"
	"math/big"
	"strings"
)

func creditActivity(op model.Operation, owner, grantID string) map[string]any {
	out := map[string]any{"actions": []string{op.Kind}, "grantRefs": grantRefs(op), "feeAmount": op.FeeAmount, "feeRecipient": op.FeeRecipient, "amountScope": "operation"}
	if effect, ok := map[string]string{"createCredit": "created", "updateCredit": "updated", "creditDraw": "advanced", "revokeCredit": "revoked"}[op.Kind]; ok {
		out["stateEffect"] = effect
	}
	if op.Kind == "createCredit" && len(op.NoteOutputs) > 0 {
		out["actions"] = []string{"createCredit", "creditDraw"}
	}
	// Core binds grantRefs[i] to creditCommitments[i] in the proof inputs.
	// RPC settlement projection preserves that order. Only attribute complete,
	// matching arrays; never identify a slot by owner alone.
	refs := grantRefs(op)
	credits := []map[string]any{}
	scopedNullifiers := []p.Field{}
	matched := false
	if len(refs) == len(op.CreditOutputs) {
		for i, ref := range refs {
			if strings.EqualFold(stringValue(ref["payer"]), owner) && stringValue(ref["grantId"]) == grantID {
				credits = append(credits, onlyPublic(copyMap(op.CreditOutputs[i]), "creditCommitment", "treeId", "leafIndex"))
				if len(op.CreditNullifiers) == len(refs) {
					scopedNullifiers = append(scopedNullifiers, op.CreditNullifiers[i])
				}
				matched = true
			}
		}
	}
	out["creditOutputs"] = credits
	out["creditOutputScope"] = "grant"
	out["creditOutputMappingConfirmed"] = matched
	out["noteOutputScope"] = "operation"
	out["creditNullifiers"] = scopedNullifiers

	notes := []map[string]any{}
	for i, n := range op.NoteOutputs {
		m := onlyPublic(copyMap(n), "noteCommitment", "treeId", "leafIndex")
		if amount := noteAmount(op, i); amount != "" {
			m["amount"] = amount
		}
		notes = append(notes, m)
	}
	out["noteOutputs"] = notes
	if total, ok := publicMoney(op); ok {
		out["operationFundingAmount"] = total.String()
	}
	count := 0
	for _, ref := range refs {
		if strings.EqualFold(stringValue(ref["payer"]), owner) {
			count++
		}
	}
	if count == 1 {
		total := new(big.Int)
		found := false
		for _, f := range op.Funding {
			if !strings.EqualFold(f.Payer, owner) || !strings.EqualFold(f.TokenAddress, op.TokenAddress) {
				continue
			}
			n, err := p.Uint(f.Amount, 128)
			if err != nil {
				return out
			}
			total.Add(total, n)
			found = true
		}
		if found {
			out["slotFundingAmount"] = total.String()
		}
	}
	return out
}
