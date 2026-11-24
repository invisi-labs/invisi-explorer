package upstream

import (
	"context"
	"errors"
	"github.com/invisi-labs/invisi-explorer/internal/api"
	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	"github.com/invisi-labs/invisi-explorer/internal/wire"
	"strconv"
	"strings"
	"sync"
	"time"
)

type transactionEntry struct {
	value model.TransactionObservation
	until time.Time
}
type transactionCache struct {
	sync.Mutex
	entries map[string]transactionEntry
}

func (s *Syncer) TransactionObservation(ctx context.Context, c config.Chain, op model.Operation) (*model.TransactionObservation, error) {
	if !op.Canonical || op.ChainID != c.ChainID || !strings.EqualFold(op.ProtocolAddress, c.ProtocolAddress) {
		return nil, errors.New("transaction scope mismatch")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	key := Scope(c).Key() + "/" + strings.ToLower(op.TxHash) + "/" + strings.ToLower(op.BlockHash)
	s.transactions.Lock()
	entry, ok := s.transactions.entries[key]
	s.transactions.Unlock()
	if ok && time.Now().Before(entry.until) {
		n, err := strconv.ParseUint(op.BlockNumber, 10, 64)
		if err != nil {
			return nil, err
		}
		current, err := s.Header(ctx, c, n)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(current, op.BlockHash) {
			return nil, &api.Error{Code: "CHAIN_REORGED"}
		}
		value := entry.value
		return &value, nil
	}
	var tx struct {
		Deployment  Deployment `json:"deployment"`
		Pending     bool       `json:"pending"`
		From        string     `json:"transactionFrom"`
		Transaction *struct {
			Hash string `json:"hash"`
			To   string `json:"to"`
			Type string `json:"type"`
		} `json:"transaction"`
	}
	if err := Call(ctx, s.Client, c.RPCURL, "invisi_readChainState", []any{map[string]any{"kind": "transaction", "transactionHash": op.TxHash}}, &tx); err != nil {
		return nil, err
	}
	if !tokenDeploymentMatches(tx.Deployment, c) || tx.Pending || tx.Transaction == nil || !strings.EqualFold(tx.Transaction.Hash, op.TxHash) {
		return nil, errors.New("transaction binding mismatch")
	}
	typ, err := strconv.ParseUint(strings.TrimPrefix(tx.Transaction.Type, "0x"), 16, 8)
	if err != nil || !strings.HasPrefix(tx.Transaction.Type, "0x") {
		return nil, errors.New("invalid transaction type")
	}
	if tx.Transaction.To != "" {
		if _, err = wire.DecodeHex(tx.Transaction.To, 20); err != nil {
			return nil, err
		}
	}
	if tx.From != "" {
		if _, err = wire.DecodeHex(tx.From, 20); err != nil {
			return nil, err
		}
	}
	var receipt struct {
		Deployment Deployment `json:"deployment"`
		Receipt    *struct {
			TxHash      string `json:"transactionHash"`
			BlockHash   string `json:"blockHash"`
			BlockNumber string `json:"blockNumber"`
			Status      string `json:"status"`
		} `json:"receipt"`
	}
	if err = Call(ctx, s.Client, c.RPCURL, "invisi_readChainState", []any{map[string]any{"kind": "receipt", "transactionHash": op.TxHash}}, &receipt); err != nil {
		return nil, err
	}
	if !tokenDeploymentMatches(receipt.Deployment, c) || receipt.Receipt == nil {
		return nil, errors.New("receipt unavailable")
	}
	r := receipt.Receipt
	n, err := strconv.ParseUint(strings.TrimPrefix(r.BlockNumber, "0x"), 16, 64)
	if err != nil || !strings.HasPrefix(r.BlockNumber, "0x") || strconv.FormatUint(n, 10) != op.BlockNumber || !strings.EqualFold(r.BlockHash, op.BlockHash) || !strings.EqualFold(r.TxHash, op.TxHash) || (r.Status != "0x0" && r.Status != "0x1") || (op.Success && r.Status != "0x1") {
		return nil, errors.New("receipt binding mismatch")
	}
	current, err := s.Header(ctx, c, n)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(current, op.BlockHash) {
		return nil, &api.Error{Code: "CHAIN_REORGED"}
	}
	value := model.TransactionObservation{ChainID: c.ChainID, ProtocolAddress: strings.ToLower(c.ProtocolAddress), TxHash: strings.ToLower(op.TxHash), BlockHash: strings.ToLower(op.BlockHash), BlockNumber: op.BlockNumber, TransactionType: strconv.FormatUint(typ, 10), TransactionTo: strings.ToLower(tx.Transaction.To), TransactionSender: strings.ToLower(tx.From)}
	ttl := 15 * time.Minute
	if value.TransactionSender == "" {
		ttl = 15 * time.Second
	}
	s.transactions.Lock()
	if len(s.transactions.entries) >= 256 || s.transactions.entries == nil {
		s.transactions.entries = map[string]transactionEntry{}
	}
	s.transactions.entries[key] = transactionEntry{value: value, until: time.Now().Add(ttl)}
	s.transactions.Unlock()
	return &value, nil
}
