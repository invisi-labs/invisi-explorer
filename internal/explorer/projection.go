package explorer

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/invisi-labs/invisi-explorer/internal/api"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/invisi-labs/invisi-explorer/internal/model"
	p "github.com/invisi-labs/invisi-explorer/internal/wire"
)

type publicRow struct {
	op        model.Operation
	id, time  string
	index     int64
	payload   string
	namespace string
}

func (e *Explorer) source(route string) (from, index, extra string) {
	switch route {
	case "commitments":
		return "(SELECT operation_id,commitment,position,payload,'notes' AS public_type FROM public_notes UNION ALL SELECT operation_id,commitment,position,payload,'credit-states' AS public_type FROM public_credit_states) n JOIN operations o ON o.id=n.operation_id", "(n.position*2+CASE WHEN n.public_type='credit-states' THEN 1 ELSE 0 END)", "n.payload"
	case "notes":
		return "public_notes n JOIN operations o ON o.id=n.operation_id", "n.position", "n.payload"
	case "credit-states":
		return "public_credit_states n JOIN operations o ON o.id=n.operation_id", "n.position", "n.payload"
	case "nullifiers":
		return "public_nullifiers n JOIN operations o ON o.id=n.operation_id", "(n.position*2+CASE WHEN n.kind='credit' THEN 1 ELSE 0 END)", "n.kind"
	case "grants":
		return "public_grant_ids n JOIN operations o ON o.id=n.operation_id", "n.position", "n.payload"
	default:
		return "operations o", "CAST(0 AS BIGINT)", "''"
	}
}
func (e *Explorer) list(ctx context.Context, route string, q query) (Page, error) {
	if route == "transactions" {
		return e.transactionList(ctx, q)
	}
	snapshot, after, err := e.pageContext(ctx, route, q)
	if err != nil {
		return Page{}, err
	}
	from, index, extra := e.source(route)
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
	if route == "notes" || route == "credit-states" || route == "commitments" || route == "grants" {
		where += " AND " + e.successSQL()
	}
	args = append(args, params...)
	if q.hash != "" {
		if route == "grants" {
			parts := strings.Split(q.hash, ":")
			if len(parts) != 5 {
				return Page{}, publicError("INVALID_PARAMS")
			}
			where += " AND LOWER(n.payer)=? AND n.grant_id=?"
			args = append(args, parts[3], parts[4])
		} else {
			column := "n.commitment"
			if route == "nullifiers" {
				column = "n.nullifier"
			}
			where += " AND " + column + "=?"
			args = append(args, q.hash)
		}
	}
	if q.address != "" {
		condition, extra := e.addressCondition(q.address)
		where += " AND " + condition
		args = append(args, extra...)
	}
	if route == "nullifiers" && q.namespace != "" {
		if q.namespace != "note" && q.namespace != "credit" {
			return Page{}, publicError("INVALID_PARAMS")
		}
		where += " AND n.kind=?"
		args = append(args, q.namespace)
	}
	if route == "commitments" && q.namespace != "" {
		kind := ""
		if q.namespace == "note" {
			kind = "notes"
		}
		if q.namespace == "credit" || q.namespace == "creditState" {
			kind = "credit-states"
		}
		if kind == "" {
			return Page{}, publicError("INVALID_PARAMS")
		}
		where += " AND n.public_type=?"
		args = append(args, kind)
	}
	if route == "grants" && q.hash == "" {
		latest, latestArgs := e.latestGrantCondition(snapshot)
		where += " AND " + latest
		args = append(args, latestArgs...)
	}
	var count int64
	if err = e.db.QueryRowContext(ctx, e.sql("SELECT COUNT(*) FROM "+from+" WHERE "+where), args...).Scan(&count); err != nil {
		return Page{}, publicError("UNAVAILABLE")
	}
	timeSQL := e.timeSQL()
	if route == "grants" {
		log := e.extract("o.payload", "logIndex")
		operation := e.extract("o.payload", "operationIndex")
		if e.options.Dialect == "postgres" {
			timeSQL = "o.block_number||lpad(" + log + ",20,'0')||lpad(" + operation + ",20,'0')"
		} else {
			timeSQL = "o.block_number||substr('00000000000000000000'||" + log + ",-20)||substr('00000000000000000000'||" + operation + ",-20)"
		}
	}
	paged := where
	pageArgs := append([]any(nil), args...)
	if after != nil {
		paged += " AND (" + timeSQL + "<? OR (" + timeSQL + "=? AND o.id<?) OR (" + timeSQL + "=? AND o.id=? AND " + index + "<?))"
		pageArgs = append(pageArgs, after.Time, after.Time, after.ID, after.Time, after.ID, after.Index)
	}
	pageArgs = append(pageArgs, q.limit+1)
	namespaceSQL := "''"
	if route == "commitments" {
		namespaceSQL = "n.public_type"
	}
	rows, err := e.db.QueryContext(ctx, e.sql("SELECT o.payload,o.canonical,o.id,"+timeSQL+","+index+","+extra+","+namespaceSQL+" FROM "+from+" WHERE "+paged+" ORDER BY "+timeSQL+" DESC,o.id DESC,"+index+" DESC LIMIT ?"), pageArgs...)
	if err != nil {
		return Page{}, publicError("UNAVAILABLE")
	}
	raw := []publicRow{}
	for rows.Next() {
		var r publicRow
		var body string
		var canonical int
		if err = rows.Scan(&body, &canonical, &r.id, &r.time, &r.index, &r.payload, &r.namespace); err != nil {
			rows.Close()
			return Page{}, publicError("UNAVAILABLE")
		}
		if json.Unmarshal([]byte(body), &r.op) != nil {
			rows.Close()
			return Page{}, publicError("UNAVAILABLE")
		}
		r.op.Canonical = canonical == 1
		raw = append(raw, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Page{}, publicError("UNAVAILABLE")
	}
	more := len(raw) > q.limit
	if more {
		raw = raw[:q.limit]
	}
	page := Page{Items: []any{}, Total: strconv.FormatInt(count, 10), Snapshot: snapshot, Unit: route}
	for _, r := range raw {
		item, err := e.project(ctx, route, r)
		if err != nil {
			return Page{}, err
		}
		if route == "grants" && q.hash == "" {
			if err = e.enrichGrantLifecycle(ctx, item, snapshot); err != nil {
				return Page{}, err
			}
		}
		page.Items = append(page.Items, item)
	}
	if more {
		last := raw[len(raw)-1]
		page.NextCursor, err = e.cursorEncode(cursor{Version: 1, Route: route, Filter: filterHash(route, q), Snapshot: snapshot, After: position{last.time, last.id, last.index}})
		if err != nil {
			return Page{}, err
		}
	}
	if err = e.validateSnapshot(ctx, snapshot); err != nil {
		return Page{}, err
	}
	return page, nil
}
func publicMoney(op model.Operation) (*big.Int, bool) {
	if !op.Success || op.UnsupportedReason != "" || op.Kind != "shield" && op.Kind != "transact" && op.Kind != "creditDraw" && op.Kind != "createCredit" || len(op.Funding) == 0 {
		return nil, false
	}
	sum := new(big.Int)
	for _, f := range op.Funding {
		n, err := p.Uint(f.Amount, 128)
		if err != nil {
			return nil, false
		}
		sum.Add(sum, n)
	}
	return sum, true
}
func noteAmount(op model.Operation, index int) string {
	if !op.Success || index < 0 || index >= len(op.NoteOutputs) {
		return ""
	}
	if op.Kind == "shield" {
		if n, err := p.Uint(op.NoteOutputs[index].Amount, 128); err == nil {
			return n.String()
		}
		return ""
	}
	if (op.Kind == "creditDraw" || op.Kind == "createCredit") && len(op.NoteOutputs) == 1 {
		gross, ok := publicMoney(op)
		fee, err := p.Uint(op.FeeAmount, 128)
		if ok && err == nil && gross.Cmp(fee) >= 0 {
			return new(big.Int).Sub(gross, fee).String()
		}
	}
	return ""
}
func copyMap(value any) map[string]any {
	b, _ := json.Marshal(value)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if m == nil {
		m = map[string]any{}
	}
	return m
}
func onlyPublic(value map[string]any, keys ...string) map[string]any {
	out := map[string]any{}
	for _, key := range keys {
		if v, ok := value[key]; ok {
			out[key] = v
		}
	}
	return out
}
func grantRefs(op model.Operation) []map[string]any {
	out := []map[string]any{}
	for _, g := range op.GrantRefs {
		r := onlyPublic(copyMap(g), "payerIndex", "payer", "owner", "grantId", "tokenAddress", "revoked")
		if i, err := strconv.ParseUint(g.PayerIndex, 10, 64); err == nil && i < uint64(len(op.Funding)) {
			r["payer"] = op.Funding[i].Payer
		}
		if _, ok := r["tokenAddress"]; !ok {
			r["tokenAddress"] = op.TokenAddress
		}
		if payer := stringValue(r["payer"]); hashHex(payer, 20) {
			r["id"] = strings.Join([]string{op.ChainID, strings.ToLower(op.ProtocolAddress), strings.ToLower(op.TokenAddress), strings.ToLower(payer), stringValue(r["grantId"])}, ":")
		}
		out = append(out, r)
	}
	return out
}
func stringValue(v any) string {
	switch n := v.(type) {
	case p.Field:
		return string(n)
	case string:
		return n
	case json.Number:
		return n.String()
	case float64:
		if n >= 0 && n <= 9007199254740991 {
			return strconv.FormatUint(uint64(n), 10)
		}
	}
	return ""
}
func drawRecord(op model.Operation) map[string]any {
	r := map[string]any{"chainId": op.ChainID, "protocolAddress": op.ProtocolAddress, "tokenAddress": op.TokenAddress, "txHash": op.TxHash, "operationIndex": op.OperationIndex, "timestamp": op.Timestamp, "grantRefs": grantRefs(op), "feeAmount": op.FeeAmount}
	if gross, ok := publicMoney(op); ok {
		r["totalAmount"] = gross.String()
	}
	if len(op.NoteOutputs) == 1 {
		r["noteCommitment"] = op.NoteOutputs[0].NoteCommitment
		if amount := noteAmount(op, 0); amount != "" {
			r["netAmount"] = amount
		}
	}
	return r
}
func (e *Explorer) operation(ctx context.Context, op model.Operation) (map[string]any, error) {
	out := copyMap(op)
	if recipient := withdrawalAddress(op); recipient != "" {
		out["withdrawalAddress"] = recipient
	}
	notes := []map[string]any{}
	for i, n := range op.NoteOutputs {
		v := map[string]any{"noteCommitment": n.NoteCommitment, "treeId": n.TreeID, "leafIndex": n.LeafIndex}
		if n.Ciphertext != "" {
			v["ciphertext"] = n.Ciphertext
		}
		if n.Clue != "" {
			v["clue"] = n.Clue
		}
		if amount := noteAmount(op, i); amount != "" {
			v["amount"] = amount
			v["amountVisibility"] = "publicly_derivable"
		}
		notes = append(notes, v)
	}
	if !op.Success {
		notes = []map[string]any{}
		out["funding"] = []any{}
		out["withdrawalAmount"] = "0"
		out["feeAmount"] = "0"
		out["settlementStatus"] = "failed"
	}
	out["noteOutputs"] = notes
	credits := []map[string]any{}
	for _, c := range op.CreditOutputs {
		credits = append(credits, onlyPublic(copyMap(c), "creditCommitment", "commitment", "treeId", "leafIndex", "role", "ownerCiphertext", "spenderCiphertext", "ownerClue", "spenderClue", "ciphertext", "clue", "delivery"))
	}
	if !op.Success {
		credits = []map[string]any{}
	}
	out["creditOutputs"] = credits
	out["grantRefs"] = grantRefs(op)
	if principal, ok := publicMoney(op); ok {
		out["publicDepositAmount"] = principal.String()
	}
	if !op.Canonical {
		out["recognition"] = []any{map[string]any{"status": "chain_reorged", "reason": "CHAIN_REORGED"}}
	} else if e.options.Recognition != nil {
		r, err := e.options.Recognition(ctx, op)
		if err != nil {
			return nil, publicError("UNAVAILABLE")
		}
		normalized := normalizeRecognition(r)
		if entries, ok := normalized.([]any); ok {
			for _, entry := range entries {
				if row, ok := entry.(map[string]any); ok {
					row["operationIndex"] = op.OperationIndex
					if op.MemberIndex != "" {
						row["memberIndex"] = op.MemberIndex
					}
				}
			}
		}
		out["recognition"] = normalized
	}
	if op.Kind == "creditDraw" || op.Kind == "createCredit" && len(op.NoteOutputs) > 0 {
		out["drawRecords"] = []any{drawRecord(op)}
	}
	return out, nil
}
func (e *Explorer) project(ctx context.Context, route string, r publicRow) (map[string]any, error) {
	if route == "transactions" {
		return e.operation(ctx, r.op)
	}
	if route == "commitments" {
		route = r.namespace
		r.index /= 2
	}
	op := r.op
	base := map[string]any{"chainId": op.ChainID, "protocolAddress": op.ProtocolAddress, "tokenAddress": op.TokenAddress, "txHash": op.TxHash, "originTransactionHash": op.TxHash, "operationIndex": op.OperationIndex, "memberIndex": op.MemberIndex, "timestamp": op.Timestamp, "blockNumber": op.BlockNumber, "blockHash": op.BlockHash, "canonical": op.Canonical, "success": op.Success, "kind": op.Kind}
	var payload map[string]any
	if route != "nullifiers" {
		if json.Unmarshal([]byte(r.payload), &payload) != nil {
			return nil, publicError("UNAVAILABLE")
		}
	}
	switch route {
	case "notes":
		v := onlyPublic(payload, "noteCommitment", "commitment", "treeId", "leafIndex", "ciphertext", "clue", "delivery")
		for k, x := range v {
			base[k] = x
		}
		base["domain"] = "note"
		if r.index >= 0 && r.index < int64(len(op.NoteOutputs)) {
			n := op.NoteOutputs[r.index]
			base["noteCommitment"] = n.NoteCommitment
			if amount := noteAmount(op, int(r.index)); amount != "" {
				base["amount"] = amount
				base["amountVisibility"] = "publicly_derivable"
			}
		}
		if !op.Canonical {
			base["recognition"] = map[string]any{"status": "chain_reorged", "reason": "CHAIN_REORGED"}
		} else if e.options.NoteRecognition != nil {
			c, err := p.ParseField(stringValue(base["noteCommitment"]))
			if err != nil {
				return nil, publicError("UNAVAILABLE")
			}
			recognition, err := e.options.NoteRecognition(ctx, op.ChainID, op.ProtocolAddress, c)
			if err != nil {
				return nil, publicError("UNAVAILABLE")
			}
			base["recognitionEvidence"] = normalizeRecognition(recognition)
			base["recognition"] = normalizeRecognition(recognition)
		}
		if op.Canonical && e.options.Recognition != nil {
			states, err := e.options.Recognition(ctx, op)
			if err != nil {
				return nil, publicError("UNAVAILABLE")
			}
			base["recognition"] = normalizeRecognition(states)
		}
		if op.Kind == "creditDraw" || op.Kind == "createCredit" && len(op.NoteOutputs) > 0 {
			base["drawRecords"] = []any{drawRecord(op)}
		}
	case "credit-states":
		for k, v := range onlyPublic(payload, "creditCommitment", "commitment", "treeId", "leafIndex", "role", "ownerCiphertext", "spenderCiphertext", "ownerClue", "spenderClue", "ciphertext", "clue", "delivery") {
			base[k] = v
		}
		base["domain"] = "credit"
		base["recognitionStatus"] = "not_applicable"
		base["grantRefs"] = grantRefs(op)
	case "nullifiers":
		r.index /= 2
		base["domain"] = r.payload
		if !op.Canonical {
			base["usageStatus"] = "orphaned"
		} else if op.Success && op.UnsupportedReason == "" {
			base["usageStatus"] = "executed"
		} else {
			base["usageStatus"] = "attempted"
		}
		list := op.NoteNullifiers
		if r.payload == "credit" {
			list = op.CreditNullifiers
		}
		if r.index < 0 || r.index >= int64(len(list)) {
			return nil, publicError("UNAVAILABLE")
		}
		base["nullifier"] = list[r.index]
	case "grants":
		base["authorizationStatus"] = grantAuthorizationStatus(op.Kind)
		for k, v := range creditActivity(op, stringValue(payload["payer"]), stringValue(payload["grantId"])) {
			base[k] = v
		}
		for k, v := range onlyPublic(payload, "payer", "owner", "payerIndex", "grantId", "tokenAddress", "revoked") {
			base[k] = v
		}
		if payer := stringValue(base["payer"]); hashHex(payer, 20) {
			base["id"] = strings.Join([]string{op.ChainID, strings.ToLower(op.ProtocolAddress), strings.ToLower(op.TokenAddress), strings.ToLower(payer), stringValue(base["grantId"])}, ":")
		}
		if op.Kind == "creditDraw" || op.Kind == "createCredit" && len(op.NoteOutputs) > 0 {
			base["drawRecords"] = []any{drawRecord(op)}
		}
	}
	return base, nil
}
func (e *Explorer) transaction(ctx context.Context, q query, hash string) (any, error) {
	// Kind/token/address are list selectors. A transaction detail always returns
	// all its members within the requested chain/Core deployment.
	q.kind, q.token, q.address = "", "", ""
	if !hashHex(hash, 32) {
		return nil, publicError("INVALID_PARAMS")
	}
	snapshot, err := e.snapshot(ctx, q)
	if err != nil {
		return nil, err
	}
	where, args := instanceWhere(snapshot)
	filter, params := e.filters(q)
	args = append(args, params...)
	args = append(args, strings.ToLower(hash))
	raw := []model.Operation{}
	orphaned := false
	for _, canonical := range []int{1, 0} {
		selectedWhere, selectedArgs := where, args
		if canonical == 1 {
			selectedWhere, selectedArgs = snapshotWhere(snapshot)
			selectedArgs = append(selectedArgs, params...)
			selectedArgs = append(selectedArgs, strings.ToLower(hash))
		}
		rows, err := e.db.QueryContext(ctx, e.sql("SELECT payload,canonical FROM operations o WHERE "+selectedWhere+" AND "+filter+" AND LOWER(tx_hash)=? AND canonical="+strconv.Itoa(canonical)+" ORDER BY block_number DESC,operation_index LIMIT 201"), selectedArgs...)
		if err != nil {
			return nil, publicError("UNAVAILABLE")
		}
		for rows.Next() {
			var body string
			var flag int
			if rows.Scan(&body, &flag) != nil {
				rows.Close()
				return nil, publicError("UNAVAILABLE")
			}
			var op model.Operation
			if json.Unmarshal([]byte(body), &op) != nil {
				rows.Close()
				return nil, publicError("UNAVAILABLE")
			}
			op.Canonical = flag == 1
			raw = append(raw, op)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, publicError("UNAVAILABLE")
		}
		if len(raw) > 0 {
			orphaned = canonical == 0
			break
		}
	}
	if len(raw) == 0 {
		return nil, publicError("NOT_INDEXED")
	}
	if len(raw) > 200 {
		return nil, publicError("UNSUPPORTED")
	}
	sort.SliceStable(raw, func(i, j int) bool {
		a, _ := strconv.ParseUint(raw[i].OperationIndex, 10, 64)
		b, _ := strconv.ParseUint(raw[j].OperationIndex, 10, 64)
		return a < b
	})
	ops := []any{}
	recognition := []any{}
	type observedTransaction struct {
		value *model.TransactionObservation
		err   error
	}
	observations := map[string]observedTransaction{}
	observationCtx, stopObservation := context.WithTimeout(ctx, 5*time.Second)
	defer stopObservation()
	for _, op := range raw {
		projected, err := e.operation(ctx, op)
		if err != nil {
			return nil, err
		}
		if op.Canonical && e.options.TransactionObservation != nil {
			observationKey := op.ChainID + "/" + strings.ToLower(op.ProtocolAddress) + "/" + strings.ToLower(op.TxHash) + "/" + strings.ToLower(op.BlockHash)
			cached, found := observations[observationKey]
			if !found {
				cached.value, cached.err = e.options.TransactionObservation(observationCtx, op)
				observations[observationKey] = cached
			}
			observation, observationErr := cached.value, cached.err
			if observationErr == nil && observation != nil {
				projected["transactionObservation"] = observation
			}
			if observationErr != nil {
				var public *api.Error
				if errors.As(observationErr, &public) && public.Code == "CHAIN_REORGED" {
					return nil, public
				}
			}
		}
		ops = append(ops, projected)
		if r, ok := projected["recognition"]; ok {
			if values, ok := r.([]any); ok {
				recognition = append(recognition, values...)
			} else {
				recognition = append(recognition, r)
			}
		}
	}
	if err = e.validateSnapshot(ctx, snapshot); err != nil {
		return nil, err
	}
	return map[string]any{"transactionHash": strings.ToLower(hash), "operations": ops, "operationCount": strconv.Itoa(len(ops)), "recognition": recognition, "snapshot": snapshot, "orphaned": orphaned}, nil
}
func (e *Explorer) object(ctx context.Context, route string, q query, hash string, occurrences bool) (any, error) {
	f, err := p.ParseField(hash)
	if err != nil {
		return nil, publicError("INVALID_PARAMS")
	}
	q.hash = string(f)
	if occurrences {
		q.history = true
	}
	page, err := e.list(ctx, route, q)
	if err != nil {
		return nil, err
	}
	if occurrences {
		return page, nil
	}
	if len(page.Items) == 0 {
		q.onlyOrphans = true
		page, err = e.list(ctx, route, q)
		if err != nil {
			return nil, err
		}
		if len(page.Items) == 0 {
			return nil, publicError("NOT_INDEXED")
		}
		return map[string]any{"commitment": f, "domain": route, "occurrences": page.Items, "total": page.Total, "nextCursor": page.NextCursor, "snapshot": page.Snapshot, "orphaned": true, "recognition": []any{map[string]any{"status": "chain_reorged", "reason": "CHAIN_REORGED"}}}, nil
	}
	global := page
	if route == "notes" {
		globalQuery := query{limit: 1, hash: string(f)}
		global, err = e.list(ctx, route, globalQuery)
		if err != nil {
			return nil, err
		}
	}
	if global.Total != "1" {
		return map[string]any{"commitment": f, "domain": route, "occurrences": page.Items, "total": page.Total, "nextCursor": page.NextCursor, "snapshot": page.Snapshot, "indexAnomaly": true, "recognition": []any{}}, nil
	}
	out := copyMap(page.Items[0])
	out["occurrences"] = page.Items
	out["snapshot"] = page.Snapshot
	return out, nil
}
func (e *Explorer) commitment(ctx context.Context, q query, hash string) (any, error) {
	if q.namespace == "credit" || q.namespace == "creditState" {
		return e.object(ctx, "credit-states", q, hash, true)
	}
	if q.namespace == "note" {
		return e.object(ctx, "notes", q, hash, true)
	}
	if q.namespace == "" {
		return e.object(ctx, "commitments", q, hash, true)
	}
	return nil, publicError("INVALID_PARAMS")
}

func normalizeRecognition(value any) any {
	b, _ := json.Marshal(value)
	var raw any
	if json.Unmarshal(b, &raw) != nil {
		return nil
	}
	var visit func(any) any
	visit = func(v any) any {
		switch x := v.(type) {
		case []any:
			for i, n := range x {
				x[i] = visit(n)
			}
			return x
		case map[string]any:
			if items, ok := x["items"]; ok {
				return visit(items)
			}
			if status, ok := x["recognitionState"]; ok {
				x["status"] = status
			}
			if reason, ok := x["reasonCode"]; ok {
				x["reason"] = reason
			}
			return x
		default:
			return x
		}
	}
	return visit(raw)
}
