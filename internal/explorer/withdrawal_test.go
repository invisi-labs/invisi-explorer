package explorer

import (
	"fmt"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	p "github.com/invisi-labs/invisi-explorer/internal/wire"
	"testing"
)

func TestWithdrawalAddressFromBoundPublicStatement(t *testing.T) {
	address := "0x0000000000000000000000000000000000000004"
	op := model.Operation{ChainID: "97", ProtocolAddress: "0x0000000000000000000000000000000000000001", TokenAddress: "0x0000000000000000000000000000000000000002", Kind: "transact", MainCircuitCapacity: "2", WithdrawalAmount: "7", Success: true, Canonical: true, PublicInputs: make([]p.Field, 21)}
	for i := range op.PublicInputs {
		op.PublicInputs[i] = p.UintField(0)
	}
	op.PublicInputs[0] = p.UintField(97)
	op.PublicInputs[1] = p.UintField(1)
	op.PublicInputs[2] = p.UintField(2)
	op.PublicInputs[14] = p.UintField(4)
	op.PublicInputs[15] = p.UintField(7)
	if got := withdrawalAddress(op); got != address {
		t.Fatalf("missing withdrawal address: %s", got)
	}
	for _, change := range []func(*model.Operation){func(o *model.Operation) { o.Success = false }, func(o *model.Operation) { o.WithdrawalAmount = "8" }, func(o *model.Operation) {
		o.PublicInputs = append([]p.Field{}, o.PublicInputs...)
		o.PublicInputs[0] = p.UintField(1)
	}, func(o *model.Operation) {
		o.PublicInputs = append([]p.Field{}, o.PublicInputs...)
		o.PublicInputs[14] = p.Field(fmt.Sprintf("0x%064x", 1<<20) + "ff")
	}} {
		bad := op
		change(&bad)
		if withdrawalAddress(bad) != "" {
			t.Fatal("unbound withdrawal address exposed")
		}
	}
}
