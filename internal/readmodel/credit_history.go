package readmodel

import (
	"context"
	"encoding/json"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	"github.com/invisi-labs/invisi-explorer/internal/wire"
)

// DueCreditHistory includes pre-upgrade rows; chain ingestion never waits for enrichment.
func (s *Store) DueCreditHistory(ctx context.Context, scope wire.Scope, now int64, limit int) ([]model.Operation, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT o.payload FROM operations o LEFT JOIN credit_history_retry r ON r.operation_id=o.id
 WHERE o.chain_id=? AND o.protocol_address=? AND o.canonical=1
 AND json_extract(o.payload,'$.success')=1
 AND COALESCE(json_extract(o.payload,'$.unsupportedReason'),'')=''
 AND json_extract(o.payload,'$.kind') IN ('createCredit','updateCredit','revokeCredit')
 AND COALESCE(json_extract(o.payload,'$.creditSlotIndexed'),0)=0
 AND COALESCE(r.retry_at,0)<=? ORDER BY COALESCE(r.retry_at,0),o.block_number,o.id LIMIT ?`, scope.ChainID, scope.ProtocolAddress, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Operation{}
	for rows.Next() {
		var b string
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		var op model.Operation
		if err = json.Unmarshal([]byte(b), &op); err != nil {
			return nil, err
		}
		op.Canonical = true
		out = append(out, op)
	}
	return out, rows.Err()
}
func (s *Store) RetryCreditHistory(ctx context.Context, op model.Operation, at int64) error {
	_, err := s.DB.ExecContext(ctx, "INSERT INTO credit_history_retry VALUES(?,?) ON CONFLICT(operation_id) DO UPDATE SET retry_at=excluded.retry_at", op.ID(), at)
	return err
}
