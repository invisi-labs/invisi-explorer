package explorer

import (
	"context"
	"encoding/json"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"github.com/invisi-labs/invisi-explorer/internal/model"
)

type transactionGroup struct{ chain, protocol, block, hash, id, time string }

// Select transactions before applying the page limit. Filters match transactions
// containing a matching member; summaries always include every member.
func (e *Explorer) transactionList(ctx context.Context, q query) (Page, error) {
	snapshot, after, err := e.pageContext(ctx, "transactions", q)
	if err != nil {
		return Page{}, err
	}
	where, args := snapshotWhere(snapshot)
	canonical := "o.canonical=1"
	if q.history || q.onlyOrphans {
		where, args = instanceWhere(snapshot)
		canonical = "1=1"
		if q.onlyOrphans {
			canonical = "o.canonical=0"
		}
	}
	filter, params := e.filters(q)
	where = canonical + " AND " + where + " AND " + filter
	args = append(args, params...)
	if q.address != "" {
		condition, extra := e.addressCondition(q.address)
		where += " AND " + condition
		args = append(args, extra...)
	}
	grouped := " FROM operations o WHERE " + where + " GROUP BY o.chain_id,o.protocol_address,o.block_hash,o.tx_hash"
	var count int64
	if err = e.db.QueryRowContext(ctx, e.sql("SELECT COUNT(*) FROM (SELECT o.tx_hash"+grouped+") txs"), args...).Scan(&count); err != nil {
		return Page{}, publicError("UNAVAILABLE")
	}
	orderTime := "MAX(" + e.timeSQL() + ")"
	orderID := "MIN(o.id)"
	having := ""
	pageArgs := append([]any(nil), args...)
	if after != nil {
		having = " HAVING (" + orderTime + "<? OR (" + orderTime + "=? AND " + orderID + "<?))"
		pageArgs = append(pageArgs, after.Time, after.Time, after.ID)
	}
	pageArgs = append(pageArgs, q.limit+1)
	rows, err := e.db.QueryContext(ctx, e.sql("SELECT o.chain_id,o.protocol_address,o.block_hash,o.tx_hash,"+orderID+","+orderTime+grouped+having+" ORDER BY "+orderTime+" DESC,"+orderID+" DESC LIMIT ?"), pageArgs...)
	if err != nil {
		return Page{}, publicError("UNAVAILABLE")
	}
	groups := []transactionGroup{}
	for rows.Next() {
		var g transactionGroup
		if err = rows.Scan(&g.chain, &g.protocol, &g.block, &g.hash, &g.id, &g.time); err != nil {
			rows.Close()
			return Page{}, publicError("UNAVAILABLE")
		}
		groups = append(groups, g)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Page{}, publicError("UNAVAILABLE")
	}
	more := len(groups) > q.limit
	if more {
		groups = groups[:q.limit]
	}
	page := Page{Items: []any{}, Total: strconv.FormatInt(count, 10), Snapshot: snapshot, Unit: "transactions"}
	for _, g := range groups {
		// Read members without a kind/token/address filter, preserving atomic grouping.
		membersWhere, membersArgs := snapshotWhere(snapshot)
		if q.history || q.onlyOrphans {
			membersWhere, membersArgs = instanceWhere(snapshot)
		}
		membersArgs = append(membersArgs, g.chain, g.protocol, g.block, g.hash)
		rows, err := e.db.QueryContext(ctx, e.sql("SELECT o.payload,o.canonical FROM operations o WHERE "+canonical+" AND "+membersWhere+" AND o.chain_id=? AND o.protocol_address=? AND o.block_hash=? AND o.tx_hash=? ORDER BY o.operation_index LIMIT 201"), membersArgs...)
		if err != nil {
			return Page{}, publicError("UNAVAILABLE")
		}
		ops := []model.Operation{}
		for rows.Next() {
			var body string
			var flag int
			if err = rows.Scan(&body, &flag); err != nil {
				rows.Close()
				return Page{}, publicError("UNAVAILABLE")
			}
			var op model.Operation
			if json.Unmarshal([]byte(body), &op) != nil {
				rows.Close()
				return Page{}, publicError("UNAVAILABLE")
			}
			op.Canonical = flag == 1
			ops = append(ops, op)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return Page{}, publicError("UNAVAILABLE")
		}
		if len(ops) == 0 {
			return Page{}, publicError("CONFLICT")
		}
		if len(ops) > 200 {
			return Page{}, publicError("UNSUPPORTED")
		}
		sort.SliceStable(ops, func(i, j int) bool {
			a, _ := strconv.ParseUint(ops[i].OperationIndex, 10, 64)
			b, _ := strconv.ParseUint(ops[j].OperationIndex, 10, 64)
			return a < b
		})
		item, err := e.transactionSummary(ctx, ops)
		if err != nil {
			return Page{}, err
		}
		page.Items = append(page.Items, item)
	}
	if more {
		last := groups[len(groups)-1]
		page.NextCursor, err = e.cursorEncode(cursor{Version: 1, Route: "transactions", Filter: filterHash("transactions", q), Snapshot: snapshot, After: position{Time: last.time, ID: last.id}})
		if err != nil {
			return Page{}, err
		}
	}
	if err = e.validateSnapshot(ctx, snapshot); err != nil {
		return Page{}, err
	}
	return page, nil
}

func (e *Explorer) transactionSummary(ctx context.Context, ops []model.Operation) (map[string]any, error) {
	first := ops[0]
	out := map[string]any{}
	summaries := []map[string]any{}
	recognition := []any{}
	totals := map[string]*big.Int{}
	successes := 0
	for _, op := range ops {
		projected, err := e.operation(ctx, op)
		if err != nil {
			return nil, err
		}
		if len(ops) == 1 {
			out = projected
		}
		summary := onlyPublic(projected, "operationIndex", "memberIndex", "kind", "success", "tokenAddress", "recognition", "publicDepositAmount")
		summaries = append(summaries, summary)
		if op.Success {
			successes++
		}
		if amount, ok := publicMoney(op); ok {
			token := strings.ToLower(op.TokenAddress)
			if totals[token] == nil {
				totals[token] = new(big.Int)
			}
			totals[token].Add(totals[token], amount)
		}
		if values, ok := projected["recognition"].([]any); ok {
			recognition = append(recognition, values...)
		} else if value := projected["recognition"]; value != nil {
			recognition = append(recognition, value)
		}
	}
	if len(ops) > 1 {
		out = map[string]any{"chainId": first.ChainID, "protocolAddress": first.ProtocolAddress, "txHash": first.TxHash, "blockNumber": first.BlockNumber, "blockHash": first.BlockHash, "timestamp": first.Timestamp, "canonical": first.Canonical, "kind": "aggregate", "recognition": recognition}
		if successes == len(ops) {
			out["success"] = true
		} else if successes == 0 {
			out["success"] = false
		} else {
			out["executionStatus"] = "partial_success"
		}
	}
	tokens := []string{}
	for token := range totals {
		tokens = append(tokens, token)
	}
	sort.Strings(tokens)
	amounts := []map[string]any{}
	for _, token := range tokens {
		amounts = append(amounts, map[string]any{"tokenAddress": token, "amount": totals[token].String()})
	}
	out["publicDepositAmounts"] = amounts
	out["operationCount"] = strconv.Itoa(len(ops))
	out["operationSummaries"] = summaries
	return out, nil
}
