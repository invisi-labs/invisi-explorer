package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	"github.com/invisi-labs/invisi-explorer/internal/wire"
)

// creditGrant reads authenticated indexed events, never guesses owner from tx sender.
func (s *Syncer) creditGrant(ctx context.Context, c config.Chain, op model.Operation) (model.GrantRef, error) {
	fail := errors.New("credit grant binding mismatch")
	if !op.Canonical || !op.Success || op.UnsupportedReason != "" || op.ChainID != c.ChainID || !strings.EqualFold(op.ProtocolAddress, c.ProtocolAddress) {
		return model.GrantRef{}, fail
	}
	if _, err := s.Deployment(ctx, c); err != nil {
		return model.GrantRef{}, err
	}
	var result struct {
		TransactionHash string  `json:"transactionHash"`
		Status          string  `json:"status"`
		BlockNumber     *uint64 `json:"blockNumber"`
		BlockHash       string  `json:"blockHash"`
		ReceiptStatus   *uint64 `json:"receiptStatus"`
		Transitions     []struct {
			Name            string          `json:"name"`
			TransactionHash string          `json:"transactionHash"`
			LogIndex        uint64          `json:"logIndex"`
			Block           Block           `json:"block"`
			Data            json.RawMessage `json:"data"`
		} `json:"transitions"`
	}
	if err := Call(ctx, s.Client, c.RPCURL, "invisi_getTransactionCreditUpdates", []any{op.TxHash}, &result); err != nil {
		return model.GrantRef{}, err
	}
	n, err := wire.Uint64(op.BlockNumber)
	if err != nil || result.Status != "indexed" || result.BlockNumber == nil || *result.BlockNumber != n || !strings.EqualFold(result.BlockHash, op.BlockHash) || !strings.EqualFold(result.TransactionHash, op.TxHash) || result.ReceiptStatus == nil || *result.ReceiptStatus != 1 {
		return model.GrantRef{}, fail
	}
	name := map[string]string{"createCredit": "CreditCreated", "updateCredit": "CreditUpdated", "revokeCredit": "CreditRevoked"}[op.Kind]
	if name == "" {
		return model.GrantRef{}, fail
	}
	var found *model.GrantRef
	for _, event := range result.Transitions {
		if event.Name != name || strconv.FormatUint(event.LogIndex, 10) != op.LogIndex {
			continue
		}
		if found != nil || event.Block.Number != n || !strings.EqualFold(event.Block.Hash, op.BlockHash) || !strings.EqualFold(event.TransactionHash, op.TxHash) {
			return model.GrantRef{}, fail
		}
		var data struct {
			Owner            string          `json:"owner"`
			Token            string          `json:"token"`
			GrantID          string          `json:"grantId"`
			CreditCommitment string          `json:"creditCommitment"`
			Params           json.RawMessage `json:"params"`
		}
		if json.Unmarshal(event.Data, &data) != nil {
			return model.GrantRef{}, fail
		}
		if op.Kind == "updateCredit" {
			owner, params := data.Owner, data.Params
			data.Owner, data.Token, data.GrantID, data.CreditCommitment, data.Params = "", "", "", "", nil
			if json.Unmarshal(params, &data) != nil || !strings.EqualFold(owner, data.Owner) {
				return model.GrantRef{}, fail
			}
		}
		if _, err = wire.DecodeHex(data.Owner, 20); err != nil || strings.EqualFold(data.Owner, "0x"+strings.Repeat("0", 40)) {
			return model.GrantRef{}, fail
		}
		if !strings.EqualFold(data.Token, op.TokenAddress) {
			return model.GrantRef{}, fail
		}
		grant, err := wire.Uint(data.GrantID, 128)
		if err != nil || grant.Sign() == 0 {
			return model.GrantRef{}, fail
		}
		if op.Kind != "revokeCredit" && (len(op.CreditOutputs) != 1 || !strings.EqualFold(data.CreditCommitment, string(op.CreditOutputs[0].CreditCommitment))) {
			return model.GrantRef{}, fail
		}
		found = &model.GrantRef{Payer: strings.ToLower(data.Owner), GrantID: data.GrantID}
	}
	if found == nil {
		return model.GrantRef{}, fail
	}
	header, err := s.Header(ctx, c, n)
	if err != nil {
		return model.GrantRef{}, err
	}
	if !strings.EqualFold(header, op.BlockHash) {
		return model.GrantRef{}, fail
	}
	return *found, nil
}

// HydrateCreditHistory is bounded and retries independently of the chain sync checkpoint.
func (s *Syncer) HydrateCreditHistory(ctx context.Context, c config.Chain) error {
	ops, err := s.Store.DueCreditHistory(ctx, Scope(c), time.Now().Unix(), 8)
	if err != nil {
		return err
	}
	for _, op := range ops {
		bounded, stop := context.WithTimeout(ctx, 5*time.Second)
		ref, err := s.creditGrant(bounded, c, op)
		stop()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err = s.Store.RetryCreditHistory(ctx, op, time.Now().Add(time.Minute).Unix()); err != nil {
				return err
			}
			continue
		}
		op.GrantRefs = []model.GrantRef{ref}
		op.CreditSlotIndexed = true
		if err = s.Store.Enrich(ctx, op); err != nil {
			return err
		}
	}
	return nil
}
