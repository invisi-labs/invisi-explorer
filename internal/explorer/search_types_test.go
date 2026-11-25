package explorer

import (
	"context"
	"encoding/json"
	"testing"
)

func TestSearchFiltersPublicObjectTypes(t *testing.T) {
	e, _, _, ops := fixture(t)
	for _, c := range []struct{ kind, query string }{{"transaction", ops[0].TxHash}, {"note", string(ops[0].NoteOutputs[0].NoteCommitment)}, {"creditState", string(ops[0].CreditOutputs[0].CreditCommitment)}, {"nullifier", string(ops[0].NoteNullifiers[0])}, {"address", ops[0].Funding[0].Payer}} {
		body, _ := json.Marshal(map[string]string{"query": c.query, "type": c.kind})
		value, err := e.Public(context.Background(), "/search", nil, body)
		if err != nil {
			t.Fatal(c.kind, err)
		}
		rows := value.(map[string]any)["results"].([]any)
		if len(rows) == 0 {
			t.Fatal("missing", c.kind)
		}
		for _, row := range rows {
			if row.(map[string]any)["type"] != c.kind {
				t.Fatal("mixed object types", row)
			}
		}
	}
	body, _ := json.Marshal(map[string]string{"query": ops[0].TxHash, "type": "note"})
	value, err := e.Public(context.Background(), "/search", nil, body)
	if err != nil {
		t.Fatal(err)
	}
	if len(value.(map[string]any)["results"].([]any)) != 0 {
		t.Fatal("transaction matched a Note filter")
	}
}
func TestSearchTypeRejectsUnknownDuplicateAndPrivateMaterial(t *testing.T) {
	e, _, _, ops := fixture(t)
	for _, body := range []string{`{"query":"` + ops[0].TxHash + `","type":null}`, `{"query":"` + ops[0].TxHash + `","type":"domain"}`, `{"query":"` + ops[0].TxHash + `","type":"note","type":"transaction"}`, `{"query":"seed: secret words","type":"note"}`, `{"query":"` + ops[0].TxHash + `","unknown":"note"}`, `{"query":"` + ops[0].TxHash + `","type":"address"}`} {
		if _, err := e.Public(context.Background(), "/search", nil, json.RawMessage(body)); err == nil {
			t.Fatal("accepted invalid typed search", body)
		}
	}
}
