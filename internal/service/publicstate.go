package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/publicchain"
	"github.com/invisi-labs/invisi-explorer/internal/upstream"
	"strconv"
	"strings"
)

type publicState struct {
	syncer *upstream.Syncer
	chains []config.Chain
}

func (p *publicState) chain(chain, protocol string) (config.Chain, error) {
	for _, c := range p.chains {
		if c.ChainID == chain && strings.EqualFold(c.ProtocolAddress, protocol) {
			return c, nil
		}
	}
	return config.Chain{}, errors.New("unknown scope")
}
func (p *publicState) verify(ctx context.Context, c config.Chain, a publicchain.Anchor) error {
	n, e := strconv.ParseUint(a.BlockNumber, 10, 64)
	if e != nil {
		return e
	}
	h, e := p.syncer.Header(ctx, c, n)
	if e != nil {
		return e
	}
	if h != strings.ToLower(a.BlockHash) {
		return &publicchain.Error{Code: "CHAIN_REORGED"}
	}
	return nil
}
func (p *publicState) Tree(ctx context.Context, chain, protocol, tree string, a publicchain.Anchor) (publicchain.TreeState, error) {
	v := publicchain.TreeState{Scope: publicchain.Scope{ChainID: chain, ProtocolAddress: protocol}, Anchor: a, TreeID: tree, Namespace: "protocol", Status: "verified"}
	c, e := p.chain(chain, protocol)
	if e != nil {
		return v, e
	}
	if e = p.verify(ctx, c, a); e != nil {
		return v, e
	}
	id, e := strconv.ParseUint(tree, 10, 64)
	if e != nil {
		return v, e
	}
	var roots []struct {
		TreeID    uint64         `json:"treeId"`
		LeafCount uint64         `json:"leafCount"`
		Root      string         `json:"root"`
		Block     upstream.Block `json:"block"`
	}
	if e = upstream.Call(ctx, p.syncer.Client, c.RPCURL, "invisi_getRoots", []any{id}, &roots); e != nil {
		return v, e
	}
	height, _ := strconv.ParseUint(a.BlockNumber, 10, 64)
	found := false
	var max uint64
	for _, r := range roots {
		if r.TreeID == id && r.Block.Number <= height && (!found || r.LeafCount > max) {
			found = true
			max = r.LeafCount
			v.Root = r.Root
			v.LeafCount = strconv.FormatUint(r.LeafCount, 10)
			v.NextLeafIndex = v.LeafCount
		}
	}
	if !found {
		return v, &publicchain.Error{Code: "NOT_FOUND"}
	}
	v.Depth = "16"
	v.Capacity = "65536"
	// The RPC root index does not establish active-tree or contract root-history state.
	v.IsFull = max >= 65536
	return v, p.verify(ctx, c, a)
}
func (p *publicState) Grant(ctx context.Context, chain, protocol, owner, token, id string, a publicchain.Anchor) (publicchain.GrantState, error) {
	v := publicchain.GrantState{Scope: publicchain.Scope{ChainID: chain, ProtocolAddress: protocol}, Anchor: a, Owner: owner, TokenAddress: token, GrantID: id}
	c, e := p.chain(chain, protocol)
	if e != nil {
		return v, e
	}
	if e = p.verify(ctx, c, a); e != nil {
		return v, e
	}
	n, _ := strconv.ParseUint(a.BlockNumber, 10, 64)
	var result struct {
		Block  upstream.Block `json:"block"`
		Status uint8          `json:"status"`
	}
	if e = upstream.Call(ctx, p.syncer.Client, c.RPCURL, "invisi_getCreditStatus", []any{owner, token, id, n}, &result); e != nil {
		return v, e
	}
	if result.Block.Number != n || !strings.EqualFold(result.Block.Hash, a.BlockHash) || result.Status > 2 {
		return v, errors.New("grant binding mismatch")
	}
	v.Status = strconv.Itoa(int(result.Status))
	return v, p.verify(ctx, c, a)
}
func (p *publicState) Broadcasters(ctx context.Context, chain, protocol string, q publicchain.DirectoryQuery) (publicchain.DirectoryPage, error) {
	v := publicchain.DirectoryPage{Scope: publicchain.Scope{ChainID: chain, ProtocolAddress: protocol}, Anchor: q.Anchor, Items: []publicchain.Broadcaster{}}
	c, e := p.chain(chain, protocol)
	if e != nil {
		return v, e
	}
	d, e := p.syncer.Deployment(ctx, c)
	if e != nil {
		return v, e
	}
	var reg struct {
		Address   string `json:"address"`
		Hash      string `json:"runtimeCodeHash"`
		FromBlock uint64 `json:"fromBlock"`
	}
	if len(d.BroadcasterRegistry) == 0 || string(d.BroadcasterRegistry) == "null" {
		v.Status = "not_configured"
		return v, nil
	}
	if e = json.Unmarshal(d.BroadcasterRegistry, &reg); e != nil {
		return v, e
	}
	v.RegistryAddress = reg.Address
	v.RegistryCodeHash = reg.Hash
	v.FromBlock = strconv.FormatUint(reg.FromBlock, 10)
	v.Status = "verified"
	if e = p.verify(ctx, c, q.Anchor); e != nil {
		return v, e
	}
	n, _ := strconv.ParseUint(q.Anchor.BlockNumber, 10, 64)
	var result struct {
		Block        upstream.Block `json:"block"`
		Broadcasters []struct {
			Operator     string         `json:"operator"`
			Endpoint     string         `json:"endpoint"`
			MetadataURI  string         `json:"metadataURI"`
			QuoteSigner  string         `json:"quoteSigner"`
			FeeRecipient string         `json:"feeRecipient"`
			Enabled      bool           `json:"enabled"`
			Block        upstream.Block `json:"block"`
		} `json:"broadcasters"`
		Next string `json:"next"`
	}
	fullBlock, err := p.syncer.BlockAt(ctx, c, n)
	if err != nil {
		return v, err
	}
	if !strings.EqualFold(fullBlock.Hash, q.Anchor.BlockHash) {
		return v, &publicchain.Error{Code: "CHAIN_REORGED"}
	}
	after := q.After
	for len(v.Items) < q.Limit {
		limit := min(100, q.Limit-len(v.Items))
		query := map[string]any{"limit": limit, "block": fullBlock}
		if after != "" {
			query["after"] = after
		}
		// Reset slices/continuation between bounded upstream pages.
		result.Broadcasters = nil
		result.Next = ""
		if e = upstream.Call(ctx, p.syncer.Client, c.RPCURL, "invisi_getBroadcasters", []any{query}, &result); e != nil {
			return v, e
		}
		if result.Block.Number != n || !strings.EqualFold(result.Block.Hash, q.Anchor.BlockHash) || len(result.Broadcasters) > limit {
			return v, errors.New("directory binding mismatch")
		}
		for _, b := range result.Broadcasters {
			v.Items = append(v.Items, publicchain.Broadcaster{Operator: b.Operator, Endpoint: b.Endpoint, MetadataURI: b.MetadataURI, QuoteSigner: b.QuoteSigner, FeeRecipient: b.FeeRecipient, Enabled: b.Enabled, UpdatedAt: publicchain.Anchor{BlockNumber: strconv.FormatUint(b.Block.Number, 10), BlockHash: b.Block.Hash, ParentHash: b.Block.ParentHash, Timestamp: strconv.FormatUint(b.Block.Timestamp, 10)}, ServiceHealth: "not_probed"})
		}
		v.NextOperator = result.Next
		if result.Next == "" {
			break
		}
		if len(result.Broadcasters) != limit || strings.ToLower(result.Next) <= strings.ToLower(after) {
			return v, errors.New("directory continuation mismatch")
		}
		after = result.Next
	}
	return v, p.verify(ctx, c, q.Anchor)
}
