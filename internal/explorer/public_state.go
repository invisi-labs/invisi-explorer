package explorer

import (
	"context"
	"errors"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/invisi-labs/invisi-explorer/internal/api"
	"github.com/invisi-labs/invisi-explorer/internal/publicchain"
	p "github.com/invisi-labs/invisi-explorer/internal/wire"
)

type PublicState interface {
	Tree(context.Context, string, string, string, publicchain.Anchor) (publicchain.TreeState, error)
	Grant(context.Context, string, string, string, string, string, publicchain.Anchor) (publicchain.GrantState, error)
	Broadcasters(context.Context, string, string, publicchain.DirectoryQuery) (publicchain.DirectoryPage, error)
}

func stateError(err error) error {
	var public *publicchain.Error
	if errors.As(err, &public) {
		switch public.Code {
		case "INVALID_ARGUMENT", "DEPLOYMENT_MISMATCH", "CHAIN_REORGED", "INDEX_PENDING", "NOT_FOUND":
			return &api.Error{Code: public.Code}
		}
	}
	return publicError("UNAVAILABLE")
}
func chainAnchor(snapshot Snapshot, chain, protocol string) (publicchain.Anchor, error) {
	for _, instance := range snapshot.Chains {
		if instance.ChainID == chain && strings.EqualFold(instance.ProtocolAddress, protocol) {
			if !instance.PrefixReady {
				return publicchain.Anchor{}, publicError("INDEX_PENDING")
			}
			return publicchain.Anchor{BlockNumber: instance.BlockNumber, BlockHash: instance.BlockHash}, nil
		}
	}
	return publicchain.Anchor{}, publicError("DEPLOYMENT_MISMATCH")
}
func publicAddress(value string) bool {
	return hashHex(value, 20) && !strings.EqualFold(value, "0x"+strings.Repeat("0", 40))
}
func sameAnchor(actual, expected publicchain.Anchor) bool {
	return actual.BlockNumber == expected.BlockNumber && strings.EqualFold(actual.BlockHash, expected.BlockHash)
}
func (e *Explorer) enrichTree(ctx context.Context, value map[string]any, snapshot Snapshot) error {
	if e.options.PublicState == nil || value["namespace"] != "protocol" {
		return nil
	}
	chain, protocol, tree := stringValue(value["chainId"]), stringValue(value["protocolAddress"]), stringValue(value["treeId"])
	anchor, err := chainAnchor(snapshot, chain, protocol)
	if err != nil {
		return err
	}
	state, err := e.options.PublicState.Tree(ctx, chain, protocol, tree, anchor)
	if err != nil {
		return stateError(err)
	}
	if state.ChainID != chain || !strings.EqualFold(state.ProtocolAddress, protocol) || !sameAnchor(state.Anchor, anchor) || state.TreeID != tree || state.Namespace != "protocol" || state.Status != "verified" {
		return publicError("DEPLOYMENT_MISMATCH")
	}
	if _, err = p.ParseField(state.Root); err != nil {
		return publicError("DEPLOYMENT_MISMATCH")
	}
	if _, err = p.Uint64(state.LeafCount); err != nil {
		return publicError("DEPLOYMENT_MISMATCH")
	}
	value["chainState"] = state
	value["root"] = state.Root
	value["leafCount"] = state.LeafCount
	value["rootStatus"] = "verified"
	return nil
}
func (e *Explorer) directoryPage(ctx context.Context, instance ChainSnapshot, after string, limit int) (publicchain.DirectoryPage, error) {
	anchor := publicchain.Anchor{BlockNumber: instance.BlockNumber, BlockHash: instance.BlockHash}
	page, err := e.options.PublicState.Broadcasters(ctx, instance.ChainID, instance.ProtocolAddress, publicchain.DirectoryQuery{Anchor: anchor, After: after, Limit: limit})
	if err != nil {
		return page, stateError(err)
	}
	if page.ChainID != instance.ChainID || !strings.EqualFold(page.ProtocolAddress, instance.ProtocolAddress) || !sameAnchor(page.Anchor, anchor) || len(page.Items) > limit {
		return page, publicError("DEPLOYMENT_MISMATCH")
	}
	if page.Status == "not_configured" {
		if len(page.Items) != 0 || page.NextOperator != "" || page.RegistryAddress != "" || page.RegistryCodeHash != "" {
			return page, publicError("DEPLOYMENT_MISMATCH")
		}
		return page, nil
	}
	if page.Status != "verified" || !publicAddress(page.RegistryAddress) || !hashHex(page.RegistryCodeHash, 32) || page.RegistryCodeHash == p.ZeroHash {
		return page, publicError("DEPLOYMENT_MISMATCH")
	}
	previous := after
	for _, record := range page.Items {
		operator := strings.ToLower(record.Operator)
		if !publicAddress(operator) || operator <= previous || !publicAddress(record.QuoteSigner) || !publicAddress(record.FeeRecipient) {
			return page, publicError("DEPLOYMENT_MISMATCH")
		}
		previous = operator
	}
	if page.NextOperator != "" && (len(page.Items) != limit || strings.ToLower(page.NextOperator) != previous) {
		return page, publicError("DEPLOYMENT_MISMATCH")
	}
	return page, nil
}
func directoryRecord(record publicchain.Broadcaster, instance ChainSnapshot) map[string]any {
	value := copyMap(record)
	value["operator"] = strings.ToLower(record.Operator)
	value["chainId"] = instance.ChainID
	value["protocolAddress"] = instance.ProtocolAddress
	value["id"] = strings.ToLower(record.Operator)
	value["serviceHealth"] = "not_probed"
	return value
}
func directorySource(page publicchain.DirectoryPage) map[string]any {
	return map[string]any{"chainId": page.ChainID, "protocolAddress": page.ProtocolAddress, "status": page.Status, "reason": page.Reason, "anchor": page.Anchor, "registryAddress": page.RegistryAddress, "registryCodeHash": page.RegistryCodeHash}
}
func (e *Explorer) directory(ctx context.Context, q query, id string) (any, error) {
	if q.kind != "" || q.token != "" || q.namespace != "" {
		return nil, publicError("INVALID_PARAMS")
	}
	if id != "" {
		if !hashHex(id, 20) || id == "0x"+strings.Repeat("0", 40) || q.cursor != "" {
			return nil, publicError("INVALID_PARAMS")
		}
		id = strings.ToLower(id)
		q.hash = id
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	snapshot, after, err := e.pageContext(ctx, "broadcasters", q)
	if err != nil {
		return nil, err
	}
	instances := append([]ChainSnapshot(nil), snapshot.Chains...)
	sort.Slice(instances, func(i, j int) bool {
		return instances[i].ChainID+"/"+instances[i].ProtocolAddress < instances[j].ChainID+"/"+instances[j].ProtocolAddress
	})
	if len(instances) == 0 && (q.chain != "" || q.protocol != "") {
		return nil, publicError("DEPLOYMENT_MISMATCH")
	}
	if e.options.PublicState == nil {
		return map[string]any{"id": id, "items": []any{}, "total": "0", "status": "not_configured", "snapshot": snapshot, "sources": []any{}}, nil
	}
	for _, instance := range instances {
		if !instance.PrefixReady {
			return nil, publicError("INDEX_PENDING")
		}
	}
	if id != "" {
		return e.directoryDetail(ctx, q, id, snapshot, instances)
	}
	if after != nil {
		matched := false
		for _, instance := range instances {
			prefix := instance.ChainID + "/" + strings.ToLower(instance.ProtocolAddress) + "/"
			if strings.HasPrefix(after.ID, prefix) {
				operator := strings.TrimPrefix(after.ID, prefix)
				matched = hashHex(operator, 20) && operator != "0x"+strings.Repeat("0", 40)
				break
			}
		}
		if !matched {
			return nil, publicError("INVALID_PARAMS")
		}
	}

	items := []any{}
	keys := []string{}
	sources := []any{}
	configured := false
	for _, instance := range instances {
		prefix := instance.ChainID + "/" + strings.ToLower(instance.ProtocolAddress) + "/"
		start := ""
		if after != nil {
			if prefix < after.ID && !strings.HasPrefix(after.ID, prefix) {
				continue
			}
			if strings.HasPrefix(after.ID, prefix) {
				start = strings.TrimPrefix(after.ID, prefix)
				if !hashHex(start, 20) {
					return nil, publicError("INVALID_PARAMS")
				}
			}
		}
		var registryIdentity string
		for len(items) <= q.limit {
			limit := q.limit + 1 - len(items)
			if limit > 100 {
				limit = 100
			}
			page, err := e.directoryPage(ctx, instance, start, limit)
			if err != nil {
				return nil, err
			}
			identity := page.RegistryAddress + "/" + page.RegistryCodeHash + "/" + page.FromBlock
			if registryIdentity != "" && registryIdentity != identity {
				return nil, publicError("DEPLOYMENT_MISMATCH")
			}
			registryIdentity = identity
			sources = append(sources, directorySource(page))
			configured = configured || page.Status == "verified"
			for _, record := range page.Items {
				items = append(items, directoryRecord(record, instance))
				keys = append(keys, prefix+strings.ToLower(record.Operator))
			}
			if page.NextOperator == "" {
				break
			}
			start = strings.ToLower(page.NextOperator)
		}
		if len(items) > q.limit {
			break
		}
	}
	response := map[string]any{"items": items, "snapshot": snapshot, "sources": sources, "status": "verified_items", "totalBasis": "not_counted", "directoryCompleteness": "rpc_index"}
	if !configured {
		response["status"] = "not_configured"
		response["total"] = "0"
		response["totalBasis"] = "not_configured"
		response["directoryCompleteness"] = "not_configured"
	}
	if len(items) > q.limit {
		response["items"] = items[:q.limit]
		next, err := e.cursorEncode(cursor{Version: 1, Route: "broadcasters", Filter: filterHash("broadcasters", q), Snapshot: snapshot, After: position{ID: keys[q.limit-1]}})
		if err != nil {
			return nil, err
		}
		response["nextCursor"] = next
	}
	if err = e.validateSnapshot(ctx, snapshot); err != nil {
		return nil, err
	}
	return response, nil
}
func (e *Explorer) directoryDetail(ctx context.Context, q query, operator string, snapshot Snapshot, instances []ChainSnapshot) (any, error) {
	// Directory paging is exclusive by address; seeking immediately before the target is one bounded lookup.
	n := new(big.Int)
	if _, ok := n.SetString(operator[2:], 16); !ok || n.Sign() == 0 {
		return nil, publicError("INVALID_PARAMS")
	}
	n.Sub(n, big.NewInt(1))
	after := ""
	if n.Sign() > 0 {
		after = "0x" + strings.Repeat("0", 40-len(n.Text(16))) + n.Text(16)
	}
	occurrences := []any{}
	sources := []any{}
	configured := false
	for _, instance := range instances {
		page, err := e.directoryPage(ctx, instance, after, 1)
		if err != nil {
			return nil, err
		}
		configured = configured || page.Status == "verified"
		sources = append(sources, directorySource(page))
		if len(page.Items) == 1 && strings.EqualFold(page.Items[0].Operator, operator) {
			value := directoryRecord(page.Items[0], instance)
			value["anchor"] = page.Anchor
			value["registryAddress"] = page.RegistryAddress
			occurrences = append(occurrences, value)
		}
	}
	if err := e.validateSnapshot(ctx, snapshot); err != nil {
		return nil, err
	}
	if len(occurrences) == 1 {
		value := occurrences[0].(map[string]any)
		value["snapshot"] = snapshot
		value["sources"] = sources
		value["status"] = "verified"
		return value, nil
	}
	if len(occurrences) == 0 && configured {
		return nil, publicError("NOT_FOUND")
	}
	status := "verified_occurrences"
	if !configured {
		status = "not_configured"
	}
	return map[string]any{"id": operator, "operator": operator, "occurrences": occurrences, "total": strconv.Itoa(len(occurrences)), "status": status, "snapshot": snapshot, "sources": sources}, nil
}
