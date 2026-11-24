package explorer

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/invisi-labs/invisi-explorer/internal/model"
)

func (e *Explorer) grantHistoryStatus(ctx context.Context, snapshot Snapshot, chain, protocol, token string) (string, error) {
	where, args := snapshotWhere(snapshot)
	args = append(args, chain, strings.ToLower(protocol), strings.ToLower(token))
	query := "SELECT COUNT(*) FROM operations o WHERE o.canonical=1 AND " + where + " AND o.chain_id=? AND o.protocol_address=? AND LOWER(" + e.extract("o.payload", "tokenAddress") + ")=? AND " + e.successSQL() + " AND " + e.extract("o.payload", "kind") + " IN ('createCredit','updateCredit','revokeCredit') AND COALESCE(" + e.extract("o.payload", "unsupportedReason") + ", '')='' AND COALESCE(CAST(" + e.extract("o.payload", "creditSlotIndexed") + " AS TEXT), 'false') NOT IN ('true','1')"
	var pending int64
	if err := e.db.QueryRowContext(ctx, e.sql(query), args...).Scan(&pending); err != nil {
		return "", publicError("UNAVAILABLE")
	}
	if pending > 0 {
		return "backfilling", nil
	}
	return "complete_at_indexed_head", nil
}

// Creation evidence is queried over the complete frozen prefix, never just the
// visible action page. Absence is not classified while identities are pending
// or when a successful draw/update contradicts an uncreated slot.
func (e *Explorer) enrichGrantLifecycle(ctx context.Context, out map[string]any, snapshot Snapshot) error {
	chain, protocol, token, owner, id := stringValue(out["chainId"]), stringValue(out["protocolAddress"]), stringValue(out["tokenAddress"]), stringValue(out["payer"]), stringValue(out["grantId"])
	if owner == "" {
		owner = stringValue(out["owner"])
	}
	history, err := e.grantHistoryStatus(ctx, snapshot, chain, protocol, token)
	if err != nil {
		return err
	}
	out["historyStatus"] = history
	where, args := snapshotWhere(snapshot)
	args = append(args, chain, strings.ToLower(protocol), strings.ToLower(token), strings.ToLower(owner), id)
	from := " FROM public_grant_ids n JOIN operations o ON o.id=n.operation_id WHERE o.canonical=1 AND " + e.successSQL() + " AND " + where + " AND n.chain_id=? AND n.protocol_address=? AND LOWER(n.token_address)=? AND LOWER(n.payer)=? AND n.grant_id=?"
	var count int64
	if err = e.db.QueryRowContext(ctx, e.sql("SELECT COUNT(DISTINCT o.id)"+from), args...).Scan(&count); err != nil {
		return publicError("UNAVAILABLE")
	}
	out["historyCount"] = count
	log := "CAST(" + e.extract("o.payload", "logIndex") + " AS BIGINT)"
	order := " ORDER BY o.block_number DESC," + log + " DESC,o.operation_index DESC,o.id DESC LIMIT 1"
	var latest string
	if err = e.db.QueryRowContext(ctx, e.sql("SELECT "+e.extract("o.payload", "kind")+from+order), args...).Scan(&latest); err != nil {
		return publicError("CONFLICT")
	}
	out["authorizationStatus"] = grantAuthorizationStatus(latest)
	out["creationStatus"] = "unconfirmed"
	out["lifecycleStatus"] = grantAuthorizationStatus(latest)
	if latest == "revokeCredit" {
		out["lifecycleStatus"] = "revocation_unconfirmed"
	}
	var body string
	err = e.db.QueryRowContext(ctx, e.sql("SELECT o.payload"+from+" AND "+e.extract("o.payload", "kind")+"='createCredit' ORDER BY o.block_number,o.operation_index,o.id LIMIT 1"), args...).Scan(&body)
	if err == nil {
		var op model.Operation
		if json.Unmarshal([]byte(body), &op) != nil {
			return publicError("UNAVAILABLE")
		}
		out["creationStatus"] = "created"
		out["creationTransaction"] = onlyPublic(copyMap(op), "txHash", "operationIndex", "chainId", "protocolAddress", "tokenAddress", "timestamp", "blockNumber", "blockHash")
		if latest == "revokeCredit" {
			out["lifecycleStatus"] = "revoked_after_creation"
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return publicError("UNAVAILABLE")
	}
	if latest != "revokeCredit" || history != "complete_at_indexed_head" {
		return nil
	}
	var active int64
	if err = e.db.QueryRowContext(ctx, e.sql("SELECT COUNT(*)"+from+" AND "+e.extract("o.payload", "kind")+" IN ('creditDraw','updateCredit')"), args...).Scan(&active); err != nil {
		return publicError("UNAVAILABLE")
	}
	if active == 0 {
		out["creationStatus"] = "not_created"
		out["lifecycleStatus"] = "pre_revoked"
	}
	return nil
}
