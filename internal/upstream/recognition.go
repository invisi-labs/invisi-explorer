package upstream

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	"github.com/invisi-labs/invisi-explorer/internal/wire"
	"strconv"
	"strings"
	"time"
)

func unavailable() any {
	return []any{map[string]any{"status": "provider_unavailable", "reason": "PROVENANCE_UNAVAILABLE", "source": "provenance"}}
}

// PublicRecognition recursively copies only documented public evidence fields.
// Unrecognized keys are excluded, including job identifiers and private material.
func PublicRecognition(value any) any {
	allowed := map[string]bool{}
	for _, k := range []string{"status", "reason", "recognitionState", "reasonCode", "recognition", "providerDecisions", "fundingChecks", "assessmentHistory", "chainGeneration", "generation", "checkpointSequence", "scope", "chainId", "protocolAddress", "providerId", "providerID", "issuerId", "operationId", "operationIndex", "memberIndex", "firstRecognizedAt", "recognizedAt", "source", "position", "payer", "tokenAddress", "amount", "decision", "dataVersion", "assessedAt", "final", "ruleId", "policyId", "leaf", "treeId", "leafIndex", "noteCommitment", "commitment", "note", "items", "checks", "reasonCodes", "evidenceDigest", "coverage", "version", "sources", "screening", "funding", "updatedAt"} {
		allowed[k] = true
	}
	var walk func(any) any
	walk = func(v any) any {
		switch x := v.(type) {
		case map[string]any:
			out := map[string]any{}
			for k, n := range x {
				if allowed[k] {
					out[k] = walk(n)
				}
			}
			return out
		case []any:
			out := make([]any, len(x))
			for i, n := range x {
				out[i] = walk(n)
			}
			return out
		case string, bool, json.Number, float64, nil:
			return x
		default:
			b, _ := json.Marshal(v)
			var normalized any
			_ = json.Unmarshal(b, &normalized)
			return walk(normalized)
		}
	}
	return walk(value)
}
func (s *Syncer) Recognition(ctx context.Context, c config.Chain, op model.Operation) (any, error) {
	// Credit authority changes without ordinary Note outputs have no source
	// recognition result. This applicability comes from the public operation,
	// independently of whether an optional recognition issuer is reachable.
	if len(op.NoteOutputs) == 0 && (op.Kind == "createCredit" || op.Kind == "updateCredit" || op.Kind == "revokeCredit") {
		return []any{map[string]any{"status": "not_applicable"}}, nil
	}
	var payload string
	var updated int64
	if s.Store.DB.QueryRowContext(ctx, "SELECT payload,updated_at FROM remote_recognition WHERE operation_id=?", op.ID()).Scan(&payload, &updated) != nil || time.Now().Unix()-updated > 60 {
		_, _ = s.Store.DB.ExecContext(ctx, "INSERT INTO recognition_requests VALUES(?,?) ON CONFLICT(operation_id) DO UPDATE SET requested_at=excluded.requested_at", op.ID(), time.Now().Unix())
		return unavailable(), nil
	}
	var out any
	if json.Unmarshal([]byte(payload), &out) != nil {
		return unavailable(), nil
	}
	return out, nil
}
func (s *Syncer) NoteRecognition(ctx context.Context, c config.Chain, commitment wire.Field) (any, error) {
	scope := Scope(c)
	var payload string
	var updated int64
	e := s.Store.DB.QueryRowContext(ctx, "SELECT r.payload,r.updated_at FROM public_notes n JOIN operations o ON o.id=n.operation_id JOIN remote_recognition r ON r.operation_id=o.id WHERE n.chain_id=? AND n.protocol_address=? AND n.commitment=? AND o.canonical=1 ORDER BY r.updated_at DESC LIMIT 1", scope.ChainID, scope.ProtocolAddress, string(commitment)).Scan(&payload, &updated)
	if e != nil || time.Now().Unix()-updated > 60 {
		return unavailable(), nil
	}
	var out any
	if json.Unmarshal([]byte(payload), &out) != nil {
		return unavailable(), nil
	}
	return out, nil
}

// HydrateRecognition runs independently of chain synchronization and public reads.
// One source/transport failure trips this pass. A typed per-operation absence
// is cached and skipped so unavailable placeholders cannot starve known rows.
func (s *Syncer) HydrateRecognition(ctx context.Context, c config.Chain) error {
	scope := Scope(c)
	rows, e := s.Store.DB.QueryContext(ctx, "SELECT o.payload FROM operations o LEFT JOIN remote_recognition r ON r.operation_id=o.id LEFT JOIN recognition_requests q ON q.operation_id=o.id WHERE o.chain_id=? AND o.protocol_address=? AND o.canonical=1 AND (q.operation_id IS NOT NULL OR o.id IN (SELECT id FROM operations WHERE chain_id=? AND protocol_address=? AND canonical=1 ORDER BY block_number DESC LIMIT 200)) AND (r.updated_at IS NULL OR r.updated_at<?) ORDER BY q.requested_at DESC,COALESCE(r.updated_at,0),o.block_number DESC LIMIT 20", scope.ChainID, scope.ProtocolAddress, scope.ChainID, scope.ProtocolAddress, time.Now().Unix()-20)
	if e != nil {
		return e
	}
	var ops []model.Operation
	for rows.Next() {
		var payload string
		if e = rows.Scan(&payload); e != nil {
			rows.Close()
			return e
		}
		var op model.Operation
		if e = json.Unmarshal([]byte(payload), &op); e != nil {
			rows.Close()
			return e
		}
		ops = append(ops, op)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, op := range ops {
		requestCtx, stop := context.WithTimeout(ctx, 2*time.Second)
		var r map[string]any
		e = Call(requestCtx, s.Client, c.ProvenanceURL, "provenance_getOperationRecognition", []any{map[string]any{"blockHash": op.BlockHash, "txHash": op.TxHash, "operationIndex": op.OperationIndex}}, &r)
		stop()
		if e != nil {
			var business *RPCError
			if errors.As(e, &business) && (business.Code == -32021 || business.Code == -32002) {
				reason := "INDEX_PENDING"
				if business.Code == -32002 {
					reason = "NOT_FOUND"
				}
				pending := []any{map[string]any{"status": "provider_unavailable", "reason": reason, "source": "provenance"}}
				if cacheErr := s.Store.SaveRecognition(ctx, op, pending, time.Now().Unix()); cacheErr != nil {
					return cacheErr
				}
				continue
			}
			_ = s.Store.SaveRecognition(ctx, op, unavailable(), time.Now().Unix())
			return e
		}
		var returned model.Operation
		b, _ := json.Marshal(r["operation"])
		var responseScope wire.Scope
		scopeBytes, _ := json.Marshal(r["scope"])
		_ = json.Unmarshal(scopeBytes, &responseScope)
		if json.Unmarshal(b, &returned) != nil || returned.ID() != op.ID() || returned.BlockNumber != op.BlockNumber || returned.Success != op.Success || responseScope.Key() != scope.Key() || c.IssuerID != "" && r["providerId"] != c.IssuerID {
			_ = s.Store.SaveRecognition(ctx, op, unavailable(), time.Now().Unix())
			return fmt.Errorf("recognition identity mismatch")
		}
		if op.UnsupportedReason != "" && returned.UnsupportedReason == "" {
			returned.Canonical = true
			if e = s.Store.Enrich(ctx, returned); e != nil {
				return e
			}
		}
		out := map[string]any{}
		for _, k := range []string{"recognitionState", "recognition", "providerDecisions", "fundingChecks", "assessmentHistory", "chainGeneration", "checkpointSequence", "scope"} {
			if v, ok := r[k]; ok {
				out[k] = v
			}
		}
		out["source"] = "provenance"
		out["status"] = r["recognitionState"]
		if e = s.Store.SaveRecognition(ctx, op, PublicRecognition([]any{out}), time.Now().Unix()); e != nil {
			return e
		}
	}
	return nil
}

// SyncRoots copies published certificates, never signing or evaluating them locally.
func (s *Syncer) SyncRoots(ctx context.Context, c config.Chain) (returnErr error) {
	defer func() {
		state := "available"
		if returnErr != nil {
			state = "unavailable"
		}
		_ = s.Store.SourceStatus(context.WithoutCancel(ctx), Scope(c), "provenance", state, nil)
	}()

	if c.ProvenanceURL == "" {
		return nil
	}
	scope := Scope(c)
	var status struct {
		Scope      wire.Scope `json:"scope"`
		ProviderID string     `json:"providerId"`
		Generation string     `json:"chainGeneration"`
		Sequence   string     `json:"checkpointSequence"`
	}
	if e := Call(ctx, s.Client, c.ProvenanceURL, "provenance_getStatus", []any{}, &status); e != nil {
		return e
	}
	if status.Scope.Key() != scope.Key() || c.IssuerID != "" && status.ProviderID != c.IssuerID {
		return fmt.Errorf("recognition scope mismatch")
	}
	if _, e := wire.Uint64(status.Generation); e != nil {
		return e
	}
	if _, e := wire.Uint64(status.Sequence); e != nil {
		return e
	}
	var after sql.NullString
	_ = s.Store.DB.QueryRowContext(ctx, "SELECT MAX(sequence) FROM checkpoints WHERE scope=? AND generation=?", scope.Key(), pad(status.Generation)).Scan(&after)
	start := "0"
	if after.Valid {
		n, _ := strconv.ParseUint(after.String, 10, 64)
		start = strconv.FormatUint(n, 10)
	}
	var page struct {
		Items      []wire.Certificate `json:"items"`
		NextCursor string             `json:"nextCursor"`
	}
	if e := Call(ctx, s.Client, c.ProvenanceURL, "provenance_getRoots", []any{map[string]any{"limit": 200, "after": start, "generation": status.Generation}}, &page); e != nil {
		return e
	}
	if len(page.Items) > 200 {
		return fmt.Errorf("root response bound")
	}
	last := start
	for _, cert := range page.Items {
		if cert.Scope().Key() != scope.Key() || cert.ChainGeneration != status.Generation || c.IssuerID != "" && cert.ProviderID != c.IssuerID {
			return fmt.Errorf("certificate scope mismatch")
		}
		seq, e := wire.Uint64(cert.RootSequence)
		prev, _ := wire.Uint64(last)
		if e != nil || seq <= prev {
			return fmt.Errorf("certificate order mismatch")
		}
		last = cert.RootSequence
	}
	tx, e := s.Store.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	for _, cert := range page.Items {
		b, _ := json.Marshal(cert)
		if _, e = tx.ExecContext(ctx, "INSERT INTO checkpoints VALUES(?,?,?,?,?,1) ON CONFLICT(scope,generation,sequence) DO UPDATE SET payload=excluded.payload,signature=excluded.signature", scope.Key(), pad(cert.ChainGeneration), pad(cert.RootSequence), string(b), cert.Signature); e != nil {
			return e
		}
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO scopes VALUES(?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET generation=excluded.generation,published=excluded.published", scope.Key(), scope.ChainID, strings.ToLower(scope.ProtocolAddress), pad(status.Generation), pad(last)); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	// Current roots remain priority. Historical work starts only once this
	// current-generation page reaches its published snapshot.
	if page.NextCursor != "" {
		return nil
	}
	historyErr := s.syncHistoricalRoots(ctx, c, status.Generation, status.ProviderID)
	historyState := "available"
	if historyErr != nil {
		historyState = "unavailable"
	}
	_ = s.Store.SourceStatus(context.WithoutCancel(ctx), scope, "provenance_history", historyState, nil)
	// Historical failure does not invalidate the successfully refreshed current
	// generation or delay the independent Protocol synchronization worker.
	return nil
}
func pad(n string) string { v, _ := strconv.ParseUint(n, 10, 64); return fmt.Sprintf("%020d", v) }
