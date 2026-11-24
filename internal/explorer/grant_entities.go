package explorer

import "strings"

// Only the public entity list collapses revisions. Grant detail continues to
// paginate every action. The anti-join uses the same frozen snapshot prefix.
func (e *Explorer) latestGrantCondition(snapshot Snapshot) (string, []any) {
	where, args := snapshotWhere(snapshot)
	where = strings.ReplaceAll(where, "o.", "newer.")
	success := strings.ReplaceAll(e.successSQL(), "o.", "newer.")
	newerLog := "CAST(" + e.extract("newer.payload", "logIndex") + " AS BIGINT)"
	currentLog := "CAST(" + e.extract("o.payload", "logIndex") + " AS BIGINT)"
	later := "newer.block_number>o.block_number OR (newer.block_number=o.block_number AND (" + newerLog + ">" + currentLog + " OR (" + newerLog + "=" + currentLog + " AND (newer.operation_index>o.operation_index OR (newer.operation_index=o.operation_index AND newer.id>o.id)))))"
	return "n.payer<>'' AND n.grant_id<>'0' AND NOT EXISTS (SELECT 1 FROM public_grant_ids next_grant JOIN operations newer ON newer.id=next_grant.operation_id WHERE newer.canonical=1 AND " + success + " AND " + where + " AND next_grant.chain_id=n.chain_id AND next_grant.protocol_address=n.protocol_address AND LOWER(next_grant.token_address)=LOWER(n.token_address) AND LOWER(next_grant.payer)=LOWER(n.payer) AND next_grant.grant_id=n.grant_id AND (" + later + "))", args
}
func grantAuthorizationStatus(kind string) string {
	switch kind {
	case "createCredit", "updateCredit", "creditDraw":
		return "active"
	case "revokeCredit":
		return "revoked"
	default:
		return "unknown"
	}
}
