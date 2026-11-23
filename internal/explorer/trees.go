package explorer

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	p "github.com/invisi-labs/invisi-explorer/internal/wire"
)

func (e *Explorer) roots(ctx context.Context, q query, id string) (any, error) {
	if q.kind != "" || q.token != "" {
		return nil, publicError("UNSUPPORTED")
	}
	q.hash = id
	s, after, err := e.pageContext(ctx, "roots", q)
	if err != nil {
		return nil, err
	}
	where, args := e.scopeWhere("c.scope")
	where = "c.published=1 AND " + where
	if q.chain != "" {
		where += " AND s.chain_id=?"
		args = append(args, q.chain)
	}
	if q.protocol != "" {
		where += " AND LOWER(s.protocol_address)=?"
		args = append(args, q.protocol)
	}
	if id != "" {
		if hashHex(id, 32) {
			where += " AND LOWER(" + e.extract("c.payload", "root") + ")=?"
			args = append(args, strings.ToLower(id))
		} else {
			parts := strings.Split(id, ":")
			if len(parts) != 4 || !(pad(parts[0]) != "" && parts[0] != "0") || !hashHex(parts[1], 20) || pad(parts[2]) == "" || pad(parts[3]) == "" {
				return nil, publicError("INVALID_PARAMS")
			}
			where += " AND s.chain_id=? AND LOWER(s.protocol_address)=? AND c.generation=? AND c.sequence=?"
			args = append(args, parts[0], strings.ToLower(parts[1]), pad(parts[2]), pad(parts[3]))
		}
	}
	key := "c.scope||'/'||c.generation||'/'||c.sequence"
	base := "checkpoints c JOIN scopes s ON s.id=c.scope"
	var count int64
	if err = e.db.QueryRowContext(ctx, e.sql("SELECT COUNT(*) FROM "+base+" WHERE "+where), args...).Scan(&count); err != nil {
		return nil, publicError("UNAVAILABLE")
	}
	params := append([]any(nil), args...)
	paged := where
	if after != nil {
		paged += " AND (" + key + ")<?"
		params = append(params, after.ID)
	}
	params = append(params, q.limit+1)
	rows, err := e.db.QueryContext(ctx, e.sql("SELECT c.payload,c.signature,c.generation,s.generation,"+key+" FROM "+base+" WHERE "+paged+" ORDER BY "+key+" DESC LIMIT ?"), params...)
	if err != nil {
		return nil, publicError("UNAVAILABLE")
	}
	type rootRow struct {
		certificate p.Certificate
		current     bool
		key         string
	}
	raw := []rootRow{}
	for rows.Next() {
		var body, sig, generation, current, keyValue string
		if rows.Scan(&body, &sig, &generation, &current, &keyValue) != nil {
			rows.Close()
			return nil, publicError("UNAVAILABLE")
		}
		var cert p.Certificate
		if json.Unmarshal([]byte(body), &cert) != nil {
			rows.Close()
			return nil, publicError("UNAVAILABLE")
		}
		cert.Signature = sig
		raw = append(raw, rootRow{cert, generation == current, keyValue})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, publicError("UNAVAILABLE")
	}
	more := len(raw) > q.limit
	if more {
		raw = raw[:q.limit]
	}
	page := Page{Items: []any{}, Total: strconv.FormatInt(count, 10), Snapshot: s, Unit: "checkpoints"}
	for _, r := range raw {
		v := copyMap(r.certificate)
		v["rootId"] = r.certificate.ChainID + ":" + strings.ToLower(r.certificate.ProtocolAddress) + ":" + r.certificate.ChainGeneration + ":" + r.certificate.RootSequence
		v["currentGeneration"] = r.current
		page.Items = append(page.Items, v)
	}
	if more {
		page.NextCursor, err = e.cursorEncode(cursor{Version: 1, Route: "roots", Filter: filterHash("roots", q), Snapshot: s, After: position{ID: raw[len(raw)-1].key}})
		if err != nil {
			return nil, err
		}
	}
	if err = e.validateSnapshot(ctx, s); err != nil {
		return nil, err
	}
	var rootContext any
	if e.options.Roots != nil {
		rootContext, err = e.options.Roots(ctx)
		if err != nil {
			return nil, publicError("UNAVAILABLE")
		}
	}
	if id != "" {
		if len(page.Items) == 0 {
			return nil, publicError("NOT_INDEXED")
		}
		if count == 1 {
			out := copyMap(page.Items[0])
			out["snapshot"] = s
			out["rootContext"] = rootContext
			return out, nil
		}
		return map[string]any{"root": id, "occurrences": page.Items, "nextCursor": page.NextCursor, "total": page.Total, "snapshot": s, "rootContext": rootContext}, nil
	}
	return map[string]any{"items": page.Items, "total": page.Total, "nextCursor": page.NextCursor, "snapshot": s, "rootContext": rootContext}, nil
}
func (e *Explorer) padSQL(value string) string {
	if e.options.Dialect == "postgres" {
		return "lpad(" + value + ",20,'0')"
	}
	return "substr('00000000000000000000'||" + value + ",-20)"
}
func (e *Explorer) trees(ctx context.Context, q query, id string) (any, error) {
	if q.namespace == "recognition" && e.options.Roots != nil {
		if _, err := e.options.Roots(ctx); err != nil {
			return nil, publicError("PROVIDER_UNAVAILABLE")
		}
	}

	if q.kind != "" || q.token != "" {
		return nil, publicError("UNSUPPORTED")
	}
	q.hash = id
	s, after, err := e.pageContext(ctx, "trees", q)
	if err != nil {
		return nil, err
	}
	where, args := snapshotWhere(s)
	where = "o.canonical=1 AND " + e.successSQL() + " AND " + where
	parts := []string{}
	allArgs := []any{}
	if q.namespace == "" || q.namespace == "protocol" {
		entries := []string{}
		for _, table := range []string{"public_notes", "public_credit_states"} {
			entries = append(entries, "SELECT o.chain_id,o.protocol_address,n.tree_id FROM "+table+" n JOIN operations o ON o.id=n.operation_id WHERE "+where)
			allArgs = append(allArgs, args...)
		}
		key := "'protocol/'||chain_id||'/'||protocol_address||'/'||" + e.padSQL("tree_id")
		parts = append(parts, "SELECT 'protocol' AS namespace,chain_id,protocol_address,'' AS generation,tree_id,CAST(COUNT(*) AS TEXT) AS metric,'' AS root,'' AS sequence,"+key+" AS entry_key FROM ("+strings.Join(entries, " UNION ALL ")+") protocol_entries GROUP BY chain_id,protocol_address,tree_id")
	}
	if q.namespace == "" || q.namespace == "recognition" {
		scopeWhere, scopeArgs := e.scopeWhere("c.scope")
		rootWhere := "c.published=1 AND c.generation=s.generation AND " + scopeWhere
		if q.chain != "" {
			rootWhere += " AND s.chain_id=?"
			scopeArgs = append(scopeArgs, q.chain)
		}
		if q.protocol != "" {
			rootWhere += " AND LOWER(s.protocol_address)=?"
			scopeArgs = append(scopeArgs, q.protocol)
		}
		tree := e.extract("c.payload", "treeId")
		latest := "SELECT MAX(c2.sequence) FROM checkpoints c2 WHERE c2.scope=c.scope AND c2.generation=c.generation AND c2.published=1 AND " + e.extract("c2.payload", "treeId") + "=" + tree
		key := "'recognition/'||c.scope||'/'||c.generation||'/'||" + e.padSQL(tree)
		parts = append(parts, "SELECT 'recognition' AS namespace,s.chain_id,s.protocol_address,c.generation,"+tree+" AS tree_id,"+e.extract("c.payload", "leafCount")+" AS metric,"+e.extract("c.payload", "root")+" AS root,c.sequence,"+key+" AS entry_key FROM checkpoints c JOIN scopes s ON s.id=c.scope WHERE "+rootWhere+" AND c.sequence=("+latest+")")
		allArgs = append(allArgs, scopeArgs...)
	}
	if len(parts) == 0 {
		return nil, publicError("INVALID_PARAMS")
	}
	base := "(" + strings.Join(parts, " UNION ALL ") + ") tree_entries"
	treeID := "namespace||':'||tree_id"
	recognitionID := "namespace||':'||chain_id||':'||protocol_address||':'||generation||':'||tree_id"
	condition := "1=1"
	if id != "" {
		segments := strings.Split(id, ":")
		if len(segments) == 2 {
			if pad(segments[1]) == "" {
				return nil, publicError("INVALID_PARAMS")
			}
			condition = treeID + "=?"
			allArgs = append(allArgs, id)
		} else if len(segments) == 5 && segments[0] == "recognition" && (pad(segments[1]) != "" && segments[1] != "0") && hashHex(segments[2], 20) && pad(segments[3]) != "" && pad(segments[4]) != "" {
			condition = recognitionID + "=?"
			allArgs = append(allArgs, "recognition:"+segments[1]+":"+strings.ToLower(segments[2])+":"+pad(segments[3])+":"+segments[4])
		} else {
			return nil, publicError("INVALID_PARAMS")
		}
	}
	var count int64
	if err = e.db.QueryRowContext(ctx, e.sql("SELECT COUNT(*) FROM "+base+" WHERE "+condition), allArgs...).Scan(&count); err != nil {
		return nil, publicError("UNAVAILABLE")
	}
	paged := condition
	params := append([]any(nil), allArgs...)
	if after != nil {
		paged += " AND entry_key<?"
		params = append(params, after.ID)
	}
	params = append(params, q.limit+1)
	rows, err := e.db.QueryContext(ctx, e.sql("SELECT namespace,chain_id,protocol_address,generation,tree_id,metric,root,sequence,entry_key FROM "+base+" WHERE "+paged+" ORDER BY entry_key DESC LIMIT ?"), params...)
	if err != nil {
		return nil, publicError("UNAVAILABLE")
	}
	type treeRow struct {
		value map[string]any
		key   string
	}
	raw := []treeRow{}
	for rows.Next() {
		var namespace, chain, protocol, generation, tree, metric, root, sequence, key string
		if rows.Scan(&namespace, &chain, &protocol, &generation, &tree, &metric, &root, &sequence, &key) != nil {
			rows.Close()
			return nil, publicError("UNAVAILABLE")
		}
		v := map[string]any{"namespace": namespace, "chainId": chain, "protocolAddress": protocol, "treeId": tree, "id": namespace + ":" + tree}
		if namespace == "recognition" {
			gen := strconv.FormatUint(parseNumber(generation), 10)
			v["id"] = "recognition:" + chain + ":" + strings.ToLower(protocol) + ":" + gen + ":" + tree
			v["chainGeneration"] = gen
			v["leafCount"] = metric
			v["root"] = root
			v["checkpointSequence"] = strconv.FormatUint(parseNumber(sequence), 10)
		} else {
			v["indexedOutputCount"] = metric
			v["rootStatus"] = "not_indexed"
		}
		raw = append(raw, treeRow{v, key})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, publicError("UNAVAILABLE")
	}
	more := len(raw) > q.limit
	if more {
		raw = raw[:q.limit]
	}
	page := Page{Items: []any{}, Total: strconv.FormatInt(count, 10), Snapshot: s, Unit: "physical_trees"}
	for _, r := range raw {
		if id != "" {
			if err = e.enrichTree(ctx, r.value, s); err != nil {
				return nil, err
			}
		}
		page.Items = append(page.Items, r.value)
	}
	if more {
		page.NextCursor, err = e.cursorEncode(cursor{Version: 1, Route: "trees", Filter: filterHash("trees", q), Snapshot: s, After: position{ID: raw[len(raw)-1].key}})
		if err != nil {
			return nil, err
		}
	}
	if err = e.validateSnapshot(ctx, s); err != nil {
		return nil, err
	}
	if id != "" {
		if count == 0 {
			return nil, publicError("NOT_INDEXED")
		}
		if count == 1 {
			return page.Items[0], nil
		}
		return map[string]any{"occurrences": page.Items, "total": page.Total, "snapshot": s}, nil
	}
	return page, nil
}
