package explorer

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	p "github.com/invisi-labs/invisi-explorer/internal/wire"
)

// Search pages contain object/deployment matches, not every address activity.
// A unique key orders every domain, so no per-domain LIMIT can hide matches.
func (e *Explorer) searchPage(ctx context.Context, q query) (any, error) {
	snapshot, after, err := e.pageContext(ctx, "search", q)
	if err != nil {
		return nil, err
	}
	where, args := snapshotWhere(snapshot)
	where = "o.canonical=1 AND " + where
	parts := []string{}
	params := []any{}
	if hashHex(q.hash, 20) {
		condition, extra := e.addressCondition(q.hash)
		parts = append(parts, "SELECT 'address' AS kind,o.chain_id,LOWER(o.protocol_address) AS protocol_address,CAST(COUNT(*) AS TEXT) AS occurrences,'' AS payload,'' AS generation,'address/'||o.chain_id||'/'||LOWER(o.protocol_address) AS entry_key FROM operations o WHERE "+where+" AND "+condition+" GROUP BY o.chain_id,LOWER(o.protocol_address)")
		params = append(params, args...)
		params = append(params, extra...)
	} else {
		definitions := []struct{ kind, from, column string }{{"transaction", "operations o", "o.tx_hash"}, {"note", "public_notes n JOIN operations o ON o.id=n.operation_id", "n.commitment"}, {"creditState", "public_credit_states n JOIN operations o ON o.id=n.operation_id", "n.commitment"}, {"nullifier", "public_nullifiers n JOIN operations o ON o.id=n.operation_id", "n.nullifier"}}
		for _, d := range definitions {
			if q.namespace != "all" && q.namespace != d.kind {
				continue
			}
			extra := ""
			if d.kind == "note" || d.kind == "creditState" {
				extra = " AND " + e.successSQL()
			}
			parts = append(parts, "SELECT '"+d.kind+"' AS kind,o.chain_id,LOWER(o.protocol_address) AS protocol_address,CAST(COUNT(*) AS TEXT) AS occurrences,'' AS payload,'' AS generation,'"+d.kind+"/'||o.chain_id||'/'||LOWER(o.protocol_address) AS entry_key FROM "+d.from+" WHERE "+where+" AND LOWER("+d.column+")=?"+extra+" GROUP BY o.chain_id,LOWER(o.protocol_address)")
			params = append(params, args...)
			params = append(params, q.hash)
		}
		if q.namespace == "all" || q.namespace == "root" {
			rootWhere, rootArgs := e.scopeWhere("c.scope")
			parts = append(parts, "SELECT 'root' AS kind,s.chain_id,LOWER(s.protocol_address) AS protocol_address,'1' AS occurrences,c.payload,s.generation,'root/'||c.scope||'/'||c.generation||'/'||c.sequence AS entry_key FROM checkpoints c JOIN scopes s ON s.id=c.scope WHERE c.published=1 AND "+rootWhere+" AND LOWER("+e.extract("c.payload", "root")+")=?")
			params = append(params, rootArgs...)
			params = append(params, q.hash)
		}
	}
	base := "(" + strings.Join(parts, " UNION ALL ") + ") search_matches"
	var total int64
	if err = e.db.QueryRowContext(ctx, e.sql("SELECT COUNT(*) FROM "+base), params...).Scan(&total); err != nil {
		return nil, publicError("UNAVAILABLE")
	}
	paged := "1=1"
	pageArgs := append([]any(nil), params...)
	if after != nil {
		paged = "entry_key<?"
		pageArgs = append(pageArgs, after.ID)
	}
	pageArgs = append(pageArgs, q.limit+1)
	rows, err := e.db.QueryContext(ctx, e.sql("SELECT kind,chain_id,protocol_address,occurrences,payload,generation,entry_key FROM "+base+" WHERE "+paged+" ORDER BY entry_key DESC LIMIT ?"), pageArgs...)
	if err != nil {
		return nil, publicError("UNAVAILABLE")
	}
	results := []any{}
	keys := []string{}
	for rows.Next() {
		var kind, chain, protocol, count, payload, generation, key string
		if rows.Scan(&kind, &chain, &protocol, &count, &payload, &generation, &key) != nil {
			rows.Close()
			return nil, publicError("UNAVAILABLE")
		}
		value := map[string]any{"type": kind, "hash": q.hash, "chainId": chain, "protocolAddress": protocol, "occurrenceCount": count}
		if kind == "root" {
			var cert p.Certificate
			if json.Unmarshal([]byte(payload), &cert) != nil {
				rows.Close()
				return nil, publicError("UNAVAILABLE")
			}
			id := cert.ChainID + ":" + strings.ToLower(cert.ProtocolAddress) + ":" + cert.ChainGeneration + ":" + cert.RootSequence
			value["hash"] = id
			value["rootId"] = id
			value["root"] = cert.Root
			value["currentGeneration"] = pad(cert.ChainGeneration) == generation
		}
		results = append(results, value)
		keys = append(keys, key)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, publicError("UNAVAILABLE")
	}
	next := ""
	if len(results) > q.limit {
		results = results[:q.limit]
		next, err = e.cursorEncode(cursor{Version: 1, Route: "search", Filter: filterHash("search", q), Snapshot: snapshot, After: position{ID: keys[q.limit-1]}})
		if err != nil {
			return nil, err
		}
	}
	if err = e.validateSnapshot(ctx, snapshot); err != nil {
		return nil, err
	}
	return map[string]any{"results": results, "total": strconv.FormatInt(total, 10), "nextCursor": next, "snapshot": snapshot, "searchBoundary": "public_identifiers_only", "truncated": false}, nil
}
