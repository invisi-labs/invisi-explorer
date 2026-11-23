package explorer

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	p "github.com/invisi-labs/invisi-explorer/internal/wire"
)

func (e *Explorer) overview(ctx context.Context, q query) (any, error) {
	s, err := e.snapshot(ctx, q)
	if err != nil {
		return nil, err
	}
	where, args := snapshotWhere(s)
	filter, params := e.filters(q)
	where = "o.canonical=1 AND " + where + " AND " + filter
	args = append(args, params...)
	counts := map[string]string{}
	queries := map[string]string{"transactions": "SELECT COUNT(*) FROM (SELECT o.chain_id,o.protocol_address,o.tx_hash FROM operations o WHERE " + where + " GROUP BY o.chain_id,o.protocol_address,o.tx_hash) public_transactions", "operations": "SELECT COUNT(*) FROM operations o WHERE " + where, "notes": "SELECT COUNT(DISTINCT n.commitment) FROM public_notes n JOIN operations o ON o.id=n.operation_id WHERE " + where + " AND " + e.successSQL(), "creditStates": "SELECT COUNT(DISTINCT n.commitment) FROM public_credit_states n JOIN operations o ON o.id=n.operation_id WHERE " + where + " AND " + e.successSQL()}
	for key, sql := range queries {
		var n int64
		if err = e.db.QueryRowContext(ctx, e.sql(sql), args...).Scan(&n); err != nil {
			return nil, publicError("UNAVAILABLE")
		}
		counts[key] = strconv.FormatInt(n, 10)
	}
	out := map[string]any{"transactions": counts["transactions"], "operations": counts["operations"], "notes": counts["notes"], "creditStates": counts["creditStates"], "recognizedNotes": nil, "recognizedNotesBasis": "unavailable_remote_recognition", "updatedAt": s.AcquiredAt, "snapshot": s}
	var recognized, known int64
	var recognitionArgs []any
	countSQL := "SELECT COUNT(DISTINCT CASE WHEN json_extract(r.payload,'$[0].status')='recognized' THEN n.commitment END),COUNT(DISTINCT CASE WHEN r.updated_at>=? AND json_extract(r.payload,'$[0].status') NOT IN ('provider_unavailable','unavailable') THEN n.commitment END) FROM public_notes n JOIN operations o ON o.id=n.operation_id LEFT JOIN remote_recognition r ON r.operation_id=o.id WHERE " + where + " AND " + e.successSQL()
	// The freshness placeholder occurs before the snapshot/filter placeholders.
	recognitionArgs = append([]any{time.Now().Unix() - 60}, args...)
	if e.db.QueryRowContext(ctx, e.sql(countSQL), recognitionArgs...).Scan(&recognized, &known) == nil && known > 0 && strconv.FormatInt(known, 10) == counts["notes"] {
		out["recognizedNotes"] = strconv.FormatInt(recognized, 10)
		out["recognizedNotesBasis"] = "cached_provenance_recognition_at_indexed_prefix"
	}
	if err = e.validateSnapshot(ctx, s); err != nil {
		return nil, err
	}
	return out, nil
}
func (e *Explorer) scopeWhere(column string) (string, []any) {
	if len(e.options.Scopes) == 0 {
		return "0=1", nil
	}
	args := []any{}
	parts := []string{}
	seen := map[string]bool{}
	for _, s := range e.options.Scopes {
		if !seen[s.Key()] {
			seen[s.Key()] = true
			parts = append(parts, "?")
			args = append(args, s.Key())
		}
	}
	return column + " IN (" + strings.Join(parts, ",") + ")", args
}
func (e *Explorer) grant(ctx context.Context, q query, id string) (any, error) {
	parts := strings.Split(id, ":")
	if len(parts) != 5 {
		return nil, publicError("INVALID_PARAMS")
	}
	if _, err := p.Uint64(parts[0]); err != nil {
		return nil, publicError("INVALID_PARAMS")
	}
	for _, index := range []int{1, 2, 3} {
		if !hashHex(parts[index], 20) {
			return nil, publicError("INVALID_PARAMS")
		}
		parts[index] = strings.ToLower(parts[index])
	}
	if _, err := p.Uint(parts[4], 128); err != nil {
		return nil, publicError("INVALID_PARAMS")
	}
	if parts[4] == "0" {
		return nil, publicError("INVALID_PARAMS")
	}
	if q.chain != "" && q.chain != parts[0] || q.protocol != "" && q.protocol != parts[1] || q.token != "" && q.token != parts[2] {
		return nil, publicError("SCOPE_MISMATCH")
	}
	q.chain = parts[0]
	q.protocol = parts[1]
	q.token = parts[2]
	q.hash = strings.Join(parts, ":")
	page, err := e.list(ctx, "grants", q)
	if err != nil {
		return nil, err
	}
	if page.Total == "0" {
		return nil, publicError("NOT_INDEXED")
	}
	draws := []any{}
	for _, item := range page.Items {
		m := item.(map[string]any)
		if records, ok := m["drawRecords"].([]any); ok {
			draws = append(draws, records...)
		}
	}
	result := map[string]any{"id": q.hash, "chainId": q.chain, "protocolAddress": q.protocol, "tokenAddress": q.token, "payer": parts[3], "grantId": parts[4], "activities": page, "drawRecords": draws, "snapshot": page.Snapshot}
	if e.options.PublicState != nil {
		anchor, err := chainAnchor(page.Snapshot, q.chain, q.protocol)
		if err != nil {
			return nil, err
		}
		state, err := e.options.PublicState.Grant(ctx, q.chain, q.protocol, parts[3], parts[2], parts[4], anchor)
		if err != nil {
			return nil, stateError(err)
		}
		if state.ChainID != q.chain || !strings.EqualFold(state.ProtocolAddress, q.protocol) || !strings.EqualFold(state.Owner, parts[3]) || !strings.EqualFold(state.TokenAddress, parts[2]) || state.GrantID != parts[4] || !sameAnchor(state.Anchor, anchor) || (state.Status != "0" && state.Status != "1" && state.Status != "2") {
			return nil, publicError("DEPLOYMENT_MISMATCH")
		}
		result["chainState"] = state
		result["status"] = state.Status
		if err = e.validateSnapshot(ctx, page.Snapshot); err != nil {
			return nil, err
		}
	}
	return result, nil
}
func (e *Explorer) address(ctx context.Context, q query, address string) (any, error) {
	if !hashHex(address, 20) {
		return nil, publicError("INVALID_PARAMS")
	}
	q.address = strings.ToLower(address)
	page, err := e.list(ctx, "transactions", q)
	if err != nil {
		return nil, err
	}
	return map[string]any{"address": q.address, "activities": page, "snapshot": page.Snapshot, "roleBoundary": "public_payer_transaction_sender_funding_participant_protocol_token_feeRecipient_only"}, nil
}
func (e *Explorer) addressCondition(address string) (string, []any) {
	fee := e.extract("o.payload", "feeRecipient")
	token := e.extract("o.payload", "tokenAddress")
	player := "json_extract(o.payload,'$.routerFunding.playerAddress')"
	funding := "EXISTS (SELECT 1 FROM json_each(o.payload,'$.funding') f WHERE LOWER(json_extract(f.value,'$.payer'))=?)"
	sender := "json_extract(o.payload,'$.fundingEvidence.transactionSender')"
	flows := "EXISTS (SELECT 1 FROM json_each(o.payload,'$.fundingEvidence.transfers') x WHERE LOWER(json_extract(x.value,'$.from'))=? OR LOWER(json_extract(x.value,'$.to'))=? OR LOWER(json_extract(x.value,'$.tokenAddress'))=?)"
	if e.options.Dialect == "postgres" {
		sender = "(o.payload::jsonb->'fundingEvidence'->>'transactionSender')"
		flows = "EXISTS (SELECT 1 FROM jsonb_array_elements(o.payload::jsonb->'fundingEvidence'->'transfers') x WHERE LOWER(x->>'from')=? OR LOWER(x->>'to')=? OR LOWER(x->>'tokenAddress')=?)"
		player = "(o.payload::jsonb->'routerFunding'->>'playerAddress')"
		funding = "EXISTS (SELECT 1 FROM jsonb_array_elements(o.payload::jsonb->'funding') f WHERE LOWER(f->>'payer')=?)"
	}
	return "(LOWER(o.protocol_address)=? OR LOWER(" + token + ")=? OR LOWER(" + fee + ")=? OR " + funding + " OR LOWER(" + player + ")=? OR LOWER(" + sender + ")=? OR " + flows + ")", []any{address, address, address, address, address, address, address, address, address}
}
func (e *Explorer) search(ctx context.Context, values url.Values, body json.RawMessage) (any, error) {
	if len(values) != 0 || len(body) == 0 || len(body) > 1024 {
		return nil, publicError("INVALID_PARAMS")
	}
	var request struct{ Query string }
	decoder := json.NewDecoder(bytes.NewReader(body))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, publicError("INVALID_PARAMS")
	}
	if !decoder.More() {
		return nil, publicError("INVALID_PARAMS")
	}
	key, err := decoder.Token()
	if err != nil || key != "query" || decoder.Decode(&request.Query) != nil || decoder.More() {
		return nil, publicError("INVALID_PARAMS")
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') {
		return nil, publicError("INVALID_PARAMS")
	}
	if _, err = decoder.Token(); err != io.EOF {
		return nil, publicError("INVALID_PARAMS")
	}
	query := strings.TrimSpace(request.Query)
	lower := strings.ToLower(query)
	for _, prefix := range []string{"0i", "ipn", "viewkey", "view key", "privatekey", "private key", "secretkey", "secret key", "seed:", "mnemonic:"} {
		if strings.HasPrefix(lower, prefix) {
			return nil, publicError("SECRET_MATERIAL")
		}
	}
	if !hashHex(query, 32) && !hashHex(query, 20) {
		return nil, publicError("INVALID_PARAMS")
	}
	query = lower
	q := queryOptions()
	snapshot, err := e.snapshot(ctx, q)
	if err != nil {
		return nil, err
	}
	where, args := snapshotWhere(snapshot)
	where = "o.canonical=1 AND " + where
	results := []any{}
	if hashHex(query, 20) {
		condition, extra := e.addressCondition(query)
		params := append(append([]any(nil), args...), extra...)
		rows, err := e.db.QueryContext(ctx, e.sql("SELECT o.chain_id,o.protocol_address FROM operations o WHERE "+where+" AND "+condition+" GROUP BY o.chain_id,o.protocol_address LIMIT 201"), params...)
		if err != nil {
			return nil, publicError("UNAVAILABLE")
		}
		for rows.Next() {
			var chain, protocol string
			if rows.Scan(&chain, &protocol) != nil {
				rows.Close()
				return nil, publicError("UNAVAILABLE")
			}
			results = append(results, map[string]any{"type": "address", "hash": query, "chainId": chain, "protocolAddress": protocol})
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, publicError("UNAVAILABLE")
		}
	} else {
		definitions := []struct{ kind, from, column string }{{"transaction", "operations o", "o.tx_hash"}, {"note", "public_notes n JOIN operations o ON o.id=n.operation_id", "n.commitment"}, {"creditState", "public_credit_states n JOIN operations o ON o.id=n.operation_id", "n.commitment"}, {"nullifier", "public_nullifiers n JOIN operations o ON o.id=n.operation_id", "n.nullifier"}}
		for _, d := range definitions {
			params := append(append([]any(nil), args...), query)
			extraWhere := ""
			if d.kind == "note" || d.kind == "creditState" {
				extraWhere = " AND " + e.successSQL()
			}
			rows, err := e.db.QueryContext(ctx, e.sql("SELECT o.chain_id,o.protocol_address,COUNT(*) FROM "+d.from+" WHERE "+where+" AND LOWER("+d.column+")=?"+extraWhere+" GROUP BY o.chain_id,o.protocol_address LIMIT 201"), params...)
			if err != nil {
				return nil, publicError("UNAVAILABLE")
			}
			for rows.Next() {
				var chain, protocol string
				var count int64
				if rows.Scan(&chain, &protocol, &count) != nil {
					rows.Close()
					return nil, publicError("UNAVAILABLE")
				}
				results = append(results, map[string]any{"type": d.kind, "hash": query, "chainId": chain, "protocolAddress": protocol, "occurrenceCount": strconv.FormatInt(count, 10)})
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return nil, publicError("UNAVAILABLE")
			}
		}
		rootWhere, rootArgs := e.scopeWhere("c.scope")
		rootArgs = append(rootArgs, query)
		rootRows, rootErr := e.db.QueryContext(ctx, e.sql("SELECT c.payload,c.generation,s.generation FROM checkpoints c JOIN scopes s ON s.id=c.scope WHERE c.published=1 AND "+rootWhere+" AND LOWER("+e.extract("c.payload", "root")+")=? ORDER BY c.scope,c.generation DESC,c.sequence DESC LIMIT 201"), rootArgs...)
		if rootErr != nil {
			return nil, publicError("UNAVAILABLE")
		}
		for rootRows.Next() {
			var payload, generation, current string
			if rootRows.Scan(&payload, &generation, &current) != nil {
				rootRows.Close()
				return nil, publicError("UNAVAILABLE")
			}
			var cert p.Certificate
			if json.Unmarshal([]byte(payload), &cert) != nil {
				rootRows.Close()
				return nil, publicError("UNAVAILABLE")
			}
			id := cert.ChainID + ":" + strings.ToLower(cert.ProtocolAddress) + ":" + cert.ChainGeneration + ":" + cert.RootSequence
			results = append(results, map[string]any{"type": "root", "hash": id, "rootId": id, "root": cert.Root, "chainId": cert.ChainID, "protocolAddress": cert.ProtocolAddress, "currentGeneration": generation == current})
		}
		rootErr = rootRows.Err()
		rootRows.Close()
		if rootErr != nil {
			return nil, publicError("UNAVAILABLE")
		}

	}
	truncated := len(results) > 200
	if truncated {
		results = results[:200]
	}
	if err = e.validateSnapshot(ctx, snapshot); err != nil {
		return nil, err
	}
	return map[string]any{"results": results, "snapshot": snapshot, "searchBoundary": "public_identifiers_only", "truncated": truncated}, nil
}
func queryOptions() query { return query{limit: 20} }
