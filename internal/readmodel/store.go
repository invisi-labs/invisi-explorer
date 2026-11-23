// Package readmodel owns the Explorer query database. It contains public facts only.
package readmodel

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	"github.com/invisi-labs/invisi-explorer/internal/wire"
	_ "modernc.org/sqlite"
	"strconv"
	"strings"
	"time"
)

//go:embed schema.sql
var schema string

type Store struct{ DB *sql.DB }

func Open(path string) (*Store, error) {
	db, e := sql.Open("sqlite", path)
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	var applicationID, tables int
	if e = db.QueryRow("PRAGMA application_id").Scan(&applicationID); e != nil {
		db.Close()
		return nil, e
	}
	if e = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&tables); e != nil {
		db.Close()
		return nil, e
	}
	const explorerApplicationID = 0x49564558
	if (applicationID != 0 && applicationID != explorerApplicationID) || (applicationID == 0 && tables != 0) {
		db.Close()
		return nil, fmt.Errorf("database is not an Explorer-owned read model; select a dedicated empty path")
	}

	if _, e = db.Exec("PRAGMA application_id=1230390616; PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;" + schema); e != nil {
		db.Close()
		return nil, e
	}
	return &Store{db}, nil
}
func (s *Store) Close() error { return s.DB.Close() }
func pad(v string) string     { n, _ := strconv.ParseUint(v, 10, 64); return fmt.Sprintf("%020d", n) }
func (s *Store) Stage(ctx context.Context, scope wire.Scope, ops []model.Operation, cursor, target string) error {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	for _, op := range ops {
		if op.ChainID != scope.ChainID || !strings.EqualFold(op.ProtocolAddress, scope.ProtocolAddress) {
			return fmt.Errorf("operation scope mismatch")
		}
		b, e := json.Marshal(op)
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO staged_operations(scope,id,payload) VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload", scope.Key(), op.ID(), string(b)); e != nil {
			return e
		}
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO sync_windows(scope,cursor,target) VALUES(?,?,?) ON CONFLICT(scope) DO UPDATE SET cursor=excluded.cursor,target=excluded.target", scope.Key(), cursor, target); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) ResetStage(ctx context.Context, scope wire.Scope) error {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, "DELETE FROM staged_operations WHERE scope=?", scope.Key()); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM sync_windows WHERE scope=?", scope.Key()); e != nil {
		return e
	}
	return tx.Commit()
}
func project(ctx context.Context, tx *sql.Tx, op model.Operation) error {
	op.ProtocolAddress = strings.ToLower(op.ProtocolAddress)
	op.Canonical = true
	b, _ := json.Marshal(op)
	id := op.ID()
	_, e := tx.ExecContext(ctx, "INSERT INTO operations(id,chain_id,protocol_address,block_number,block_hash,tx_hash,operation_index,canonical,payload) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET canonical=1,payload=excluded.payload", id, op.ChainID, op.ProtocolAddress, pad(op.BlockNumber), op.BlockHash, op.TxHash, pad(op.OperationIndex), 1, string(b))
	if e != nil {
		return e
	}
	for _, table := range []string{"public_notes", "public_credit_states", "public_nullifiers", "public_grant_ids"} {
		if _, e = tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE operation_id=?", id); e != nil {
			return e
		}
	}
	for i, n := range op.NoteOutputs {
		b, _ := json.Marshal(n)
		_, e = tx.ExecContext(ctx, "INSERT INTO public_notes VALUES(?,?,?,?,?,?,?,?,?,?,?)", op.ChainID, op.ProtocolAddress, string(n.NoteCommitment), id, n.TreeID, n.LeafIndex, op.TokenAddress, n.Amount, pad(op.BlockNumber), i, string(b))
		if e != nil {
			return e
		}
	}
	for i, n := range op.CreditOutputs {
		b, _ := json.Marshal(n)
		_, e = tx.ExecContext(ctx, "INSERT INTO public_credit_states VALUES(?,?,?,?,?,?,?,?,?,?,?)", op.ChainID, op.ProtocolAddress, string(n.CreditCommitment), id, n.TreeID, n.LeafIndex, op.TokenAddress, "", pad(op.BlockNumber), i, string(b))
		if e != nil {
			return e
		}
	}
	for kind, list := range map[string][]wire.Field{"note": op.NoteNullifiers, "credit": op.CreditNullifiers} {
		for i, n := range list {
			_, e = tx.ExecContext(ctx, "INSERT INTO public_nullifiers VALUES(?,?,?,?,?,?,?)", op.ChainID, op.ProtocolAddress, string(n), kind, id, pad(op.BlockNumber), i)
			if e != nil {
				return e
			}
		}
	}
	for i, g := range op.GrantRefs {
		payer := g.Payer
		if payer == "" {
			n, err := strconv.Atoi(g.PayerIndex)
			if err == nil && n >= 0 && n < len(op.Funding) {
				payer = op.Funding[n].Payer
			}
		}
		g.Payer = payer
		b, _ := json.Marshal(g)
		_, e = tx.ExecContext(ctx, "INSERT OR REPLACE INTO public_grant_ids VALUES(?,?,?,?,?,?,?,?,?)", op.ChainID, op.ProtocolAddress, op.TokenAddress, strings.ToLower(payer), g.GrantID, id, i, pad(op.BlockNumber), string(b))
		if e != nil {
			return e
		}
	}
	return nil
}
func (s *Store) Publish(ctx context.Context, scope wire.Scope, a model.Anchor) error {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, "SELECT payload FROM staged_operations WHERE scope=? ORDER BY id", scope.Key())
	if e != nil {
		return e
	}
	var ops []model.Operation
	for rows.Next() {
		var b string
		if e = rows.Scan(&b); e != nil {
			rows.Close()
			return e
		}
		var op model.Operation
		if e = json.Unmarshal([]byte(b), &op); e != nil {
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
	if e = publishWindow(ctx, tx, scope, a, ops); e != nil {
		return e
	}
	return tx.Commit()
}

// PublishWindow atomically projects an already verified complete window. It
// checks the saved boundary inside the writer transaction; no restart can leave
// a terminal staging record whose operations could mix with a later fork.
func (s *Store) PublishWindow(ctx context.Context, scope wire.Scope, expected, a model.Anchor, ops []model.Operation) error {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var number, hash string
	if e = tx.QueryRowContext(ctx, "SELECT block_number,block_hash FROM chain_checkpoints WHERE id=?", scope.Key()).Scan(&number, &hash); e != nil {
		return e
	}
	if number != pad(expected.BlockNumber) || !strings.EqualFold(hash, expected.BlockHash) {
		return fmt.Errorf("checkpoint changed during window publication")
	}
	var pending int
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM sync_windows WHERE scope=?", scope.Key()).Scan(&pending); e != nil {
		return e
	}
	if pending != 0 {
		return fmt.Errorf("cannot replace a staged sync window")
	}
	if e = publishWindow(ctx, tx, scope, a, ops); e != nil {
		return e
	}
	return tx.Commit()
}
func publishWindow(ctx context.Context, tx *sql.Tx, scope wire.Scope, a model.Anchor, ops []model.Operation) error {
	for _, op := range ops {
		if op.ChainID != scope.ChainID || !strings.EqualFold(op.ProtocolAddress, scope.ProtocolAddress) {
			return fmt.Errorf("operation scope mismatch")
		}
		if e := project(ctx, tx, op); e != nil {
			return e
		}
	}
	for _, id := range []string{scope.Key(), scope.Key() + "/block/" + pad(a.BlockNumber)} {
		if _, e := tx.ExecContext(ctx, "INSERT INTO chain_checkpoints VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET block_number=excluded.block_number,block_hash=excluded.block_hash", id, pad(a.BlockNumber), strings.ToLower(a.BlockHash)); e != nil {
			return e
		}
	}
	if _, e := tx.ExecContext(ctx, "INSERT INTO chain_generations VALUES(?,?) ON CONFLICT(id) DO NOTHING", scope.Key(), pad("1")); e != nil {
		return e
	}
	if _, e := tx.ExecContext(ctx, "DELETE FROM staged_operations WHERE scope=?", scope.Key()); e != nil {
		return e
	}
	if _, e := tx.ExecContext(ctx, "DELETE FROM sync_windows WHERE scope=?", scope.Key()); e != nil {
		return e
	}
	return nil
}

func (s *Store) Rollback(ctx context.Context, scope wire.Scope, height uint64) error {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, "UPDATE operations SET canonical=0 WHERE chain_id=? AND protocol_address=? AND block_number>?", scope.ChainID, strings.ToLower(scope.ProtocolAddress), pad(strconv.FormatUint(height, 10))); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM chain_checkpoints WHERE (id=? OR id LIKE ?) AND block_number>?", scope.Key(), scope.Key()+"/block/%", pad(strconv.FormatUint(height, 10))); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "INSERT OR REPLACE INTO chain_checkpoints SELECT ?,block_number,block_hash FROM chain_checkpoints WHERE id LIKE ? ORDER BY block_number DESC LIMIT 1", scope.Key(), scope.Key()+"/block/%"); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO chain_generations VALUES(?,?) ON CONFLICT(id) DO UPDATE SET generation=printf('%020d',CAST(generation AS INTEGER)+1)", scope.Key(), pad("2")); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM staged_operations WHERE scope=?", scope.Key()); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM sync_windows WHERE scope=?", scope.Key()); e != nil {
		return e
	}
	return tx.Commit()
}

// Enrich replaces an existing canonical locator only; it cannot insert chain history.
func (s *Store) Enrich(ctx context.Context, op model.Operation) error {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var canonical int
	var existing string
	if e = tx.QueryRowContext(ctx, "SELECT canonical,payload FROM operations WHERE id=?", op.ID()).Scan(&canonical, &existing); e != nil {
		return e
	}
	if canonical != 1 {
		return nil
	}
	b, _ := json.Marshal(op)
	if existing == string(b) {
		return nil
	}
	if e = project(ctx, tx, op); e != nil {
		return e
	}
	scope := wire.Scope{ChainID: op.ChainID, ProtocolAddress: op.ProtocolAddress}
	if _, e = tx.ExecContext(ctx, "INSERT INTO readmodel_versions VALUES(?,1) ON CONFLICT(id) DO UPDATE SET version=version+1", scope.Key()); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) SaveRecognition(ctx context.Context, op model.Operation, value any, now int64) error {
	b, e := json.Marshal(value)
	if e != nil {
		return e
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var old string
	_ = tx.QueryRowContext(ctx, "SELECT payload FROM remote_recognition WHERE operation_id=?", op.ID()).Scan(&old)
	if _, e = tx.ExecContext(ctx, "INSERT INTO remote_recognition VALUES(?,?,?) ON CONFLICT(operation_id) DO UPDATE SET payload=excluded.payload,updated_at=excluded.updated_at", op.ID(), string(b), now); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM recognition_requests WHERE operation_id=?", op.ID()); e != nil {
		return e
	}
	if old != string(b) {
		scope := wire.Scope{ChainID: op.ChainID, ProtocolAddress: op.ProtocolAddress}
		if _, e = tx.ExecContext(ctx, "INSERT INTO readmodel_versions VALUES(?,1) ON CONFLICT(id) DO UPDATE SET version=version+1", scope.Key()); e != nil {
			return e
		}
	}
	return tx.Commit()
}
func (s *Store) SourceStatus(ctx context.Context, scope wire.Scope, source, status string, value any) error {
	b, _ := json.Marshal(value)
	_, e := s.DB.ExecContext(ctx, "INSERT INTO source_status VALUES(?,?,?,?,?) ON CONFLICT(scope,source) DO UPDATE SET status=excluded.status,updated_at=excluded.updated_at,payload=excluded.payload", scope.Key(), source, status, time.Now().Unix(), string(b))
	return e
}
