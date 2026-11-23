package explorer

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/invisi-labs/invisi-explorer/internal/model"
	"github.com/invisi-labs/invisi-explorer/internal/wire"
)

func TestMigrationDoesNotPublishPrivateTransactOutputAmounts(t *testing.T) {
	e, store, scope, ops := fixture(t)
	op := ops[0]
	op.BlockNumber = "3"
	op.BlockHash = "0x" + strings.Repeat("3", 64)
	op.TxHash = "0x" + strings.Repeat("4", 64)
	op.Kind = "transact"
	op.Funding = nil
	op.CreditOutputs = nil
	op.GrantRefs = nil
	op.NoteOutputs = []model.NoteOutput{{NoteCommitment: wire.UintField(777), TreeID: "1", LeafIndex: "9", Amount: "987654321"}}
	ctx := context.Background()
	if err := store.Stage(ctx, scope, []model.Operation{op}, "", "3"); err != nil {
		t.Fatal(err)
	}
	if err := store.Publish(ctx, scope, model.Anchor{BlockNumber: "3", BlockHash: op.BlockHash}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/notes/" + string(op.NoteOutputs[0].NoteCommitment), "/transactions/" + op.TxHash} {
		value, err := e.Public(ctx, path, url.Values{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(value)
		if strings.Contains(string(raw), "987654321") {
			t.Fatalf("%s exposed a private or untrusted amount: %s", path, raw)
		}
	}
}

func TestMigrationPreservesInitialCreditPaymentAmounts(t *testing.T) {
	_, _, _, ops := fixture(t)
	op := ops[0]
	op.Kind = "createCredit"
	op.FeeAmount = "1"
	op.NoteOutputs[0].Amount = "6"
	gross, ok := publicMoney(op)
	if !ok || gross.String() != "7" || noteAmount(op, 0) != "6" {
		t.Fatal("initial payment must separate gross funding and net Note amount")
	}
	record := drawRecord(op)
	if record["totalAmount"] != "7" || record["netAmount"] != "6" {
		t.Fatalf("wrong initial credit payment: %+v", record)
	}
}

func TestMigrationKeepsKnownSecretsOutOfPublicSearch(t *testing.T) {
	e, _, _, _ := fixture(t)
	for _, secret := range []string{"0isecret-material", "seed: private words", "private key: secret", "witness: private opening"} {
		raw, _ := json.Marshal(map[string]string{"query": secret})
		if _, err := e.Public(context.Background(), "/search", nil, raw); err == nil {
			t.Errorf("accepted known private material %q", secret)
		}
	}
}
