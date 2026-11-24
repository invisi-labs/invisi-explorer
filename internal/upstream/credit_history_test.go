package upstream

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	"github.com/invisi-labs/invisi-explorer/internal/readmodel"
	"github.com/invisi-labs/invisi-explorer/internal/wire"
)

func TestCreditGrantHydrationBindsIndexedEvent(t *testing.T) {
	for _, kind := range []string{"createCredit", "updateCredit", "revokeCredit"} {
		for _, bad := range []string{"", "tx", "block", "log", "token", "commitment", "grant", "reorg", "missing-owner"} {
			t.Run(kind+"/"+bad, func(t *testing.T) {
				if kind == "revokeCredit" && bad == "commitment" {
					t.Skip("revocation has no output")
				}
				op := model.Operation{ChainID: "97", ProtocolAddress: "0x" + strings.Repeat("1", 40), TokenAddress: "0x" + strings.Repeat("2", 40), BlockNumber: "7", BlockHash: string(wire.UintField(7)), TxHash: string(wire.UintField(8)), LogIndex: "3", Kind: kind, Success: true, Canonical: true, CreditOutputs: []model.CreditOutput{{CreditCommitment: wire.UintField(9)}}}
				owner := "0x" + strings.Repeat("3", 40)
				data := map[string]any{"owner": owner, "token": op.TokenAddress, "grantId": "11", "creditCommitment": op.CreditOutputs[0].CreditCommitment}
				if bad == "token" {
					data["token"] = "0x" + strings.Repeat("4", 40)
				}
				if bad == "commitment" {
					data["creditCommitment"] = wire.UintField(10)
				}
				if bad == "grant" {
					data["grantId"] = "0"
				}
				if bad == "missing-owner" {
					delete(data, "owner")
				}
				if kind == "updateCredit" {
					data = map[string]any{"owner": owner, "params": data}
				}
				event := map[string]any{"name": map[string]string{"createCredit": "CreditCreated", "updateCredit": "CreditUpdated", "revokeCredit": "CreditRevoked"}[kind], "transactionHash": op.TxHash, "logIndex": 3, "block": map[string]any{"number": 7, "hash": op.BlockHash}, "data": data}
				if bad == "tx" {
					event["transactionHash"] = wire.ZeroHash
				}
				if bad == "block" {
					event["block"] = map[string]any{"number": 7, "hash": wire.ZeroHash}
				}
				if bad == "log" {
					event["logIndex"] = 4
				}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var q struct{ Method string }
					json.NewDecoder(r.Body).Decode(&q)
					var result any
					switch q.Method {
					case "invisi_getDeployment":
						result = map[string]any{"deployment": map[string]any{"chainId": "97", "protocol": map[string]any{"address": op.ProtocolAddress}}}
					case "invisi_getTransactionCreditUpdates":
						result = map[string]any{"status": "indexed", "transactionHash": op.TxHash, "blockNumber": 7, "blockHash": op.BlockHash, "receiptStatus": 1, "transitions": []any{event}}
					case "invisi_readChainState":
						h := op.BlockHash
						if bad == "reorg" {
							h = wire.ZeroHash
						}
						result = map[string]any{"header": map[string]any{"number": "0x7"}, "headerHash": h}
					default:
						t.Errorf("unexpected method %s", q.Method)
					}
					b, _ := json.Marshal(result)
					fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":%s}`, b)
				}))
				defer server.Close()
				s := &Syncer{Client: server.Client()}
				c := config.Chain{ChainID: op.ChainID, ProtocolAddress: op.ProtocolAddress, RPCURL: server.URL}
				ref, err := s.creditGrant(context.Background(), c, op)
				if bad != "" {
					if err == nil {
						t.Fatal("accepted mismatched event")
					}
					return
				}
				if err != nil || ref.Payer != owner || ref.GrantID != "11" {
					t.Fatalf("ref=%+v err=%v", ref, err)
				}
			})
		}
	}
}

func TestCreditHistoryBackfillsExistingOperation(t *testing.T) {
	s, err := readmodel.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	scope := wire.Scope{ChainID: "97", ProtocolAddress: "0x" + strings.Repeat("1", 40)}
	op := model.Operation{ChainID: scope.ChainID, ProtocolAddress: scope.ProtocolAddress, Kind: "createCredit", Success: true, BlockNumber: "1", BlockHash: wire.ZeroHash, TxHash: wire.ZeroHash}
	ctx := context.Background()
	if err = s.Stage(ctx, scope, []model.Operation{op}, "", "1"); err != nil {
		t.Fatal(err)
	}
	if err = s.Publish(ctx, scope, op.Anchor()); err != nil {
		t.Fatal(err)
	}
	due, err := s.DueCreditHistory(ctx, scope, 0, 8)
	if err != nil || len(due) != 1 {
		t.Fatalf("due %d %v", len(due), err)
	}
	op = due[0]
	op.CreditSlotIndexed = true
	op.GrantRefs = []model.GrantRef{{Payer: "0x" + strings.Repeat("3", 40), GrantID: "11"}}
	if err = s.Enrich(ctx, op); err != nil {
		t.Fatal(err)
	}
	due, err = s.DueCreditHistory(ctx, scope, 0, 8)
	if err != nil || len(due) != 0 {
		t.Fatalf("due %d %v", len(due), err)
	}
	var count int
	s.DB.QueryRow("SELECT COUNT(*) FROM public_grant_ids").Scan(&count)
	if count != 1 {
		t.Fatal(count)
	}
}

func TestCreditHistoryPrioritizesUnattemptedRows(t *testing.T) {
	s, err := readmodel.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	scope := wire.Scope{ChainID: "97", ProtocolAddress: "0x" + strings.Repeat("1", 40)}
	ops := []model.Operation{}
	for i := 1; i <= 10; i++ {
		ops = append(ops, model.Operation{ChainID: scope.ChainID, ProtocolAddress: scope.ProtocolAddress, Kind: "createCredit", Success: true, BlockNumber: fmt.Sprint(i), BlockHash: string(wire.UintField(uint64(i))), TxHash: string(wire.UintField(uint64(i)))})
	}
	ctx := context.Background()
	if err = s.Stage(ctx, scope, ops, "", "10"); err != nil {
		t.Fatal(err)
	}
	if err = s.Publish(ctx, scope, ops[9].Anchor()); err != nil {
		t.Fatal(err)
	}
	for _, op := range ops[:9] {
		if err = s.RetryCreditHistory(ctx, op, 90); err != nil {
			t.Fatal(err)
		}
	}
	due, err := s.DueCreditHistory(ctx, scope, 100, 1)
	if err != nil || len(due) != 1 || due[0].BlockNumber != "10" {
		t.Fatalf("new row starved: %+v %v", due, err)
	}
}
