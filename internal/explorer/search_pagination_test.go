package explorer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"

	"github.com/invisi-labs/invisi-explorer/internal/model"
	p "github.com/invisi-labs/invisi-explorer/internal/wire"
)

func TestSearchContinuesPastTwoHundredWithBoundCursors(t *testing.T) {
	e, s, scope, ops := fixture(t)
	ctx := context.Background()
	root := string(p.UintField(999))
	if _, err := s.DB.Exec("INSERT INTO scopes(id,chain_id,protocol_address,generation,published) VALUES(?,?,?,?,?)", scope.Key(), scope.ChainID, scope.ProtocolAddress, pad("1"), pad("205")); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 205; i++ {
		cert := p.Certificate{ChainID: scope.ChainID, ProtocolAddress: scope.ProtocolAddress, ChainGeneration: "1", RootSequence: fmt.Sprint(i), Root: p.Field(root)}
		body, _ := json.Marshal(cert)
		if _, err := s.DB.Exec("INSERT INTO checkpoints(scope,generation,sequence,payload,signature,published) VALUES(?,?,?,?,?,1)", scope.Key(), pad("1"), pad(fmt.Sprint(i)), string(body), ""); err != nil {
			t.Fatal(err)
		}
	}
	search := func(cursor, kind, query string, limit int) (map[string]any, error) {
		body, _ := json.Marshal(map[string]any{"query": query, "type": kind, "limit": limit, "cursor": cursor})
		value, err := e.Public(ctx, "/search", nil, body)
		if err != nil {
			return nil, err
		}
		return value.(map[string]any), nil
	}
	first, err := search("", "root", root, 20)
	if err != nil {
		t.Fatal(err)
	}
	if first["total"] != "205" || len(first["results"].([]any)) != 20 {
		t.Fatal("unexpected first page", first)
	}
	token := first["nextCursor"].(string)
	for _, c := range []struct {
		cursor, kind, query string
		limit               int
	}{{token + "x", "root", root, 20}, {token, "all", root, 20}, {token, "root", ops[0].TxHash, 20}, {token, "root", root, 21}} {
		_, err = search(c.cursor, c.kind, c.query, c.limit)
		requireCode(t, err, "INVALID_PARAMS")
	}
	seen := map[string]bool{}
	page := first
	for {
		for _, raw := range page["results"].([]any) {
			id := raw.(map[string]any)["rootId"].(string)
			if seen[id] {
				t.Fatal("duplicate", id)
			}
			seen[id] = true
		}
		next, _ := page["nextCursor"].(string)
		if next == "" {
			break
		}
		page, err = search(next, "root", root, 20)
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(seen) != 205 {
		t.Fatal("missing search matches", len(seen))
	}
	if err = s.Rollback(ctx, scope, 0); err != nil {
		t.Fatal(err)
	}
	_, err = search(token, "root", root, 20)
	requireCode(t, err, "CHAIN_REORGED")
}

func TestAddressExposesContinuation(t *testing.T) {
	e, _, _, ops := fixture(t)
	path := "/addresses/" + ops[0].Funding[0].Payer
	value, err := e.Public(context.Background(), path, url.Values{"limit": {"1"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	first := value.(map[string]any)
	token, _ := first["nextCursor"].(string)
	if token == "" {
		t.Fatal("address continuation hidden")
	}
	value, err = e.Public(context.Background(), path, url.Values{"limit": {"1"}, "cursor": {token}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	second := value.(map[string]any)["activities"].(Page)
	if len(second.Items) != 1 || fmt.Sprint(first["activities"].(Page).Items[0]) == fmt.Sprint(second.Items[0]) {
		t.Fatal("address did not advance")
	}
}

func TestNoteAnomalyKeepsPageSizeAndContinuation(t *testing.T) {
	e, s, scope, ops := fixture(t)
	hash := string(ops[0].NoteOutputs[0].NoteCommitment)
	if _, err := s.DB.Exec("UPDATE public_notes SET commitment=?", hash); err != nil {
		t.Fatal(err)
	}
	values := url.Values{"limit": {"1"}, "chainId": {scope.ChainID}, "protocolAddress": {scope.ProtocolAddress}}
	path := "/notes/" + hash
	value, err := e.Public(context.Background(), path, values, nil)
	if err != nil {
		t.Fatal(err)
	}
	first := value.(map[string]any)
	if len(first["occurrences"].([]any)) != 1 {
		t.Fatal("anomaly ignored limit")
	}
	token, _ := first["nextCursor"].(string)
	if token == "" {
		t.Fatal("missing occurrence continuation")
	}
	values.Set("cursor", token)
	value, err = e.Public(context.Background(), path, values, nil)
	if err != nil {
		t.Fatal(err)
	}
	second := value.(map[string]any)
	if fmt.Sprint(first["occurrences"]) == fmt.Sprint(second["occurrences"]) {
		t.Fatal("occurrences did not advance")
	}
}

func TestTreeDetailExposesContinuationAcrossDeployments(t *testing.T) {
	e, s, scope, ops := fixture(t)
	second := scope
	second.ProtocolAddress = "0x0000000000000000000000000000000000000009"
	for i := range ops {
		ops[i].ProtocolAddress = second.ProtocolAddress
	}
	ctx := context.Background()
	if err := s.Stage(ctx, second, ops, "", "2"); err != nil {
		t.Fatal(err)
	}
	if err := s.Publish(ctx, second, model.Anchor{BlockNumber: "2", BlockHash: ops[1].BlockHash}); err != nil {
		t.Fatal(err)
	}
	options := e.options
	options.Scopes = append(options.Scopes, second)
	e = New(s.DB, options)
	value, err := e.Public(ctx, "/trees/protocol:1", url.Values{"limit": {"1"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	first := value.(map[string]any)
	token, _ := first["nextCursor"].(string)
	if token == "" {
		t.Fatal("tree continuation hidden")
	}
	value, err = e.Public(ctx, "/trees/protocol:1", url.Values{"limit": {"1"}, "cursor": {token}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(first["occurrences"]) == fmt.Sprint(value.(map[string]any)["occurrences"]) {
		t.Fatal("tree did not advance")
	}
}

func TestSearchMixedDomainsKeepContinuationAndRejectChangedSnapshot(t *testing.T) {
	e, s, _, ops := fixture(t)
	ctx := context.Background()
	hash := string(ops[0].NoteOutputs[0].NoteCommitment)
	if _, err := s.DB.Exec("UPDATE public_credit_states SET commitment=? WHERE commitment=?", hash, string(ops[0].CreditOutputs[0].CreditCommitment)); err != nil {
		t.Fatal(err)
	}
	search := func(token string) (map[string]any, error) {
		body, _ := json.Marshal(map[string]any{"query": hash, "limit": 1, "cursor": token})
		v, err := e.Public(ctx, "/search", nil, body)
		if err != nil {
			return nil, err
		}
		return v.(map[string]any), nil
	}
	first, err := search("")
	if err != nil {
		t.Fatal(err)
	}
	token := first["nextCursor"].(string)
	second, err := search(token)
	if err != nil {
		t.Fatal(err)
	}
	a := first["results"].([]any)[0].(map[string]any)
	b := second["results"].([]any)[0].(map[string]any)
	if first["total"] != "2" || a["type"] == b["type"] {
		t.Fatal("mixed domains lost", first, second)
	}
	if err = s.SaveRecognition(ctx, ops[0], []any{map[string]any{"status": "recognized"}}, 1); err != nil {
		t.Fatal(err)
	}
	_, err = search(token)
	requireCode(t, err, "CONFLICT")
}

func TestSearchRejectsInvalidPagingFields(t *testing.T) {
	e, _, _, ops := fixture(t)
	for _, suffix := range []string{`,"limit":0`, `,"limit":201`, `,"limit":1.0`, `,"limit":"20"`, `,"limit":null`, `,"cursor":null`, `,"limit":1,"limit":2`, `,"cursor":"a","cursor":"b"`} {
		_, err := e.Public(context.Background(), "/search", nil, json.RawMessage(`{"query":"`+ops[0].TxHash+`"`+suffix+`}`))
		requireCode(t, err, "INVALID_PARAMS")
	}
}
