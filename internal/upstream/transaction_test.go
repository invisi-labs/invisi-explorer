package upstream

import (
	"context"
	"encoding/json"
	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestTransactionObservationBoundToCanonicalReceipt(t *testing.T) {
	for _, wrong := range []bool{false, true} {
		t.Run(map[bool]string{false: "valid", true: "wrong-block"}[wrong], func(t *testing.T) {
			hash := "0x" + strings.Repeat("a", 64)
			block := "0x" + strings.Repeat("b", 64)
			core := "0x" + strings.Repeat("1", 40)
			sender := "0x" + strings.Repeat("2", 40)
			var calls atomic.Int64
			var reorg atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				var q struct{ Params []struct{ Kind string } }
				json.NewDecoder(r.Body).Decode(&q)
				result := map[string]any{"deployment": map[string]any{"chainId": "97", "protocol": map[string]any{"address": core}}, "pending": false}
				switch q.Params[0].Kind {
				case "transaction":
					result["transaction"] = map[string]any{"hash": hash, "to": core, "type": "0x2"}
					result["transactionFrom"] = sender
				case "receipt":
					bh := block
					if wrong {
						bh = "0x" + strings.Repeat("c", 64)
					}
					result["receipt"] = map[string]any{"transactionHash": hash, "blockHash": bh, "blockNumber": "0x2a", "status": "0x1"}
				case "header":
					result["headerHash"] = block
					if reorg.Load() {
						result["headerHash"] = "0x" + strings.Repeat("c", 64)
					}
					result["header"] = map[string]any{"number": "0x2a"}
				}
				json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
			}))
			defer server.Close()
			s := &Syncer{Client: server.Client()}
			c := config.Chain{ChainID: "97", ProtocolAddress: core, RPCURL: server.URL}
			op := model.Operation{ChainID: "97", ProtocolAddress: core, TxHash: hash, BlockHash: block, BlockNumber: "42", Canonical: true, Success: true}
			got, err := s.TransactionObservation(context.Background(), c, op)
			if wrong {
				if err == nil {
					t.Fatal("wrong receipt accepted")
				}
				return
			}
			if err != nil || got.TransactionSender != sender || got.TransactionTo != core || got.TransactionType != "2" {
				t.Fatalf("bad observation: %+v %v", got, err)
			}
			count := calls.Load()
			_, err = s.TransactionObservation(context.Background(), c, op)
			if err != nil || calls.Load() != count+1 {
				t.Fatal("observation not cached")
			}
			reorg.Store(true)
			if _, err = s.TransactionObservation(context.Background(), c, op); err == nil {
				t.Fatal("cached observation survived a reorg")
			}
		})
	}
}
