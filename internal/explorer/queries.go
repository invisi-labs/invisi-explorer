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
	queries["grants"] = "SELECT COUNT(*) FROM (SELECT n.chain_id,n.protocol_address,n.token_address,n.payer,n.grant_id FROM public_grant_ids n JOIN operations o ON o.id=n.operation_id WHERE " + where + " AND " + e.successSQL() + " AND n.payer<>'' AND n.grant_id<>'0' GROUP BY n.chain_id,n.protocol_address,LOWER(n.token_address),LOWER(n.payer),n.grant_id) grant_slots"
	for key, sql := range queries {
		var n int64
		if err = e.db.QueryRowContext(ctx, e.sql(sql), args...).Scan(&n); err != nil {
			return nil, publicError("UNAVAILABLE")
		}
		counts[key] = strconv.FormatInt(n, 10)
	}
	out := map[string]any{"transactions": counts["transactions"], "operations": counts["operations"], "notes": counts["notes"], "creditStates": counts["creditStates"], "grants": counts["grants"], "recognizedNotes": nil, "recognizedNotesBasis": "unavailable_remote_recognition", "updatedAt": s.AcquiredAt, "snapshot": s}
	var recognized, known int64
	var recognitionArgs []any
	countSQL := "SELECT COUNT(DISTINCT CASE WHEN json_extract(r.payload,'$[0].status')='recognized' THEN n.commitment END),COUNT(DISTINCT CASE WHEN (r.updated_at>=? OR (r.source_generation<>'' AND json_extract(r.payload,'$[0].status')='recognized')) AND json_extract(r.payload,'$[0].status') NOT IN ('provider_unavailable','unavailable') THEN n.commitment END) FROM public_notes n JOIN operations o ON o.id=n.operation_id LEFT JOIN current_recognition r ON r.operation_id=o.id WHERE " + where + " AND " + e.successSQL()
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
	result["nextCursor"] = page.NextCursor
	result["owner"] = parts[3]
	if err = e.enrichGrantLifecycle(ctx, result, page.Snapshot); err != nil {
		return nil, err
	}
	for _, item := range page.Items {
		row := item.(map[string]any)
		if row["kind"] == "revokeCredit" {
			switch result["lifecycleStatus"] {
			case "pre_revoked", "revoked_after_creation":
				row["stateEffect"] = result["lifecycleStatus"]
			}
		}
	}
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
	}
	if err = e.validateSnapshot(ctx, page.Snapshot); err != nil {
		return nil, err
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
	return map[string]any{"address": q.address, "activities": page, "nextCursor": page.NextCursor, "total": page.Total, "snapshot": page.Snapshot, "roleBoundary": "public_payer_transaction_sender_funding_participant_protocol_token_feeRecipient_only"}, nil
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
	if len(values) != 0 || len(body) == 0 || len(body) > 8192 {
		return nil, publicError("INVALID_PARAMS")
	}
	var request struct {
		Query, Type, Cursor string
		Limit               int
	}
	request.Limit = 20
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, publicError("INVALID_PARAMS")
	}
	seen := map[string]bool{}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, publicError("INVALID_PARAMS")
		}
		name, ok := key.(string)
		if !ok || seen[name] {
			return nil, publicError("INVALID_PARAMS")
		}
		seen[name] = true
		var rawValue any
		if decoder.Decode(&rawValue) != nil {
			return nil, publicError("INVALID_PARAMS")
		}
		if name == "limit" {
			number, ok := rawValue.(json.Number)
			if !ok {
				return nil, publicError("INVALID_PARAMS")
			}
			n, err := strconv.Atoi(string(number))
			if err != nil || n < 1 || n > 200 || strconv.Itoa(n) != string(number) {
				return nil, publicError("INVALID_PARAMS")
			}
			request.Limit = n
			continue
		}
		value, ok := rawValue.(string)
		if !ok {
			return nil, publicError("INVALID_PARAMS")
		}
		switch name {
		case "query":
			request.Query = value
		case "type":
			request.Type = value
		case "cursor":
			request.Cursor = value
		default:
			return nil, publicError("INVALID_PARAMS")
		}

	}
	if !seen["query"] {
		return nil, publicError("INVALID_PARAMS")
	}
	switch request.Type {
	case "", "all", "transaction", "address", "note", "creditState", "nullifier", "root":
	default:
		return nil, publicError("INVALID_PARAMS")
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') {
		return nil, publicError("INVALID_PARAMS")
	}
	if _, err = decoder.Token(); err != io.EOF {
		return nil, publicError("INVALID_PARAMS")
	}
	identifier := strings.TrimSpace(request.Query)
	lower := strings.ToLower(identifier)
	for _, prefix := range []string{"0i", "ipn", "viewkey", "view key", "privatekey", "private key", "secretkey", "secret key", "seed:", "mnemonic:"} {
		if strings.HasPrefix(lower, prefix) {
			return nil, publicError("SECRET_MATERIAL")
		}
	}
	if !hashHex(identifier, 32) && !hashHex(identifier, 20) {
		return nil, publicError("INVALID_PARAMS")
	}
	if (request.Type == "address" && !hashHex(identifier, 20)) || (request.Type != "" && request.Type != "all" && request.Type != "address" && !hashHex(identifier, 32)) {
		return nil, publicError("INVALID_PARAMS")
	}
	kind := request.Type
	if kind == "" {
		kind = "all"
	}
	return e.searchPage(ctx, query{limit: request.Limit, cursor: request.Cursor, hash: lower, namespace: kind})
}

func queryOptions() query { return query{limit: 20} }
