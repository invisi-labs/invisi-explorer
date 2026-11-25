package readmodel

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/invisi-labs/invisi-explorer/internal/model"
	"github.com/invisi-labs/invisi-explorer/internal/wire"
)

// RecognitionContext fences asynchronous responses across source and chain changes.
// Epoch also changes when an endpoint/issuer is changed away and then back.
type RecognitionContext struct {
	Epoch           int64
	Generation      string
	Provider        string
	ChainGeneration string
}

func bumpRecognitionVersion(ctx context.Context, tx *sql.Tx, scope wire.Scope) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO readmodel_versions VALUES(?,1) ON CONFLICT(id) DO UPDATE SET version=version+1", scope.Key())
	return err
}

// ConfigureRecognition invalidates bindings on configuration changes, including
// removal of an optional source. It performs no network operations.
func (s *Store) ConfigureRecognition(ctx context.Context, scope wire.Scope, url, issuer string) error {
	var oldURL, oldIssuer string
	err := s.DB.QueryRowContext(ctx, "SELECT url,issuer FROM recognition_sources WHERE scope=?", scope.Key()).Scan(&oldURL, &oldIssuer)
	if err == nil && oldURL == url && oldIssuer == issuer {
		return nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, `INSERT INTO recognition_sources(scope,url,issuer,epoch) VALUES(?,?,?,1)
 ON CONFLICT(scope) DO UPDATE SET url=excluded.url,issuer=excluded.issuer,provider='',generation='',epoch=epoch+1
 WHERE url<>excluded.url OR issuer<>excluded.issuer`, scope.Key(), url, issuer)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n > 0 {
		if err = bumpRecognitionVersion(ctx, tx, scope); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) RecognitionContext(ctx context.Context, scope wire.Scope) (RecognitionContext, error) {
	var c RecognitionContext
	err := s.DB.QueryRowContext(ctx, `SELECT epoch,generation,provider,COALESCE((SELECT generation FROM chain_generations WHERE id=?),'00000000000000000001') FROM recognition_sources WHERE scope=?`, scope.Key(), scope.Key()).Scan(&c.Epoch, &c.Generation, &c.Provider, &c.ChainGeneration)
	return c, err
}

// ObserveRecognitionGeneration runs after a validated status response, before
// downloading roots. Failed root downloads must not keep the old epoch usable.
func (s *Store) ObserveRecognitionGeneration(ctx context.Context, scope wire.Scope, expected RecognitionContext, provider, generation string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var epoch int64
	var oldProvider, oldGeneration string
	if err = tx.QueryRowContext(ctx, "SELECT epoch,provider,generation FROM recognition_sources WHERE scope=?", scope.Key()).Scan(&epoch, &oldProvider, &oldGeneration); err != nil {
		return err
	}
	if epoch != expected.Epoch {
		return errors.New("recognition source changed")
	}
	if oldProvider == provider && oldGeneration == generation {
		return tx.Commit()
	}
	// Within an issuer, a late response cannot roll the observed generation back.
	if oldProvider == provider && oldGeneration != "" && pad(generation) < pad(oldGeneration) {
		return errors.New("recognition generation regressed")
	}
	if _, err = tx.ExecContext(ctx, "UPDATE recognition_sources SET provider=?,generation=?,epoch=epoch+1 WHERE scope=?", provider, generation, scope.Key()); err != nil {
		return err
	}
	if err = bumpRecognitionVersion(ctx, tx, scope); err != nil {
		return err
	}
	return tx.Commit()
}

// SaveBoundRecognition checks the epochs and canonical locator in the same
// transaction as the cache write. An in-flight pre-reorg response cannot revive
// invalidated evidence. On failed queries callers do not call this method.
func (s *Store) SaveBoundRecognition(ctx context.Context, op model.Operation, value any, now int64, expected RecognitionContext) error {
	scope := wire.Scope{ChainID: op.ChainID, ProtocolAddress: op.ProtocolAddress}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var epoch int64
	var chain string
	var canonical bool
	err = tx.QueryRowContext(ctx, `SELECT s.epoch,COALESCE(g.generation,'00000000000000000001'),o.canonical
 FROM recognition_sources s LEFT JOIN chain_generations g ON g.id=s.scope JOIN operations o ON o.id=? WHERE s.scope=?`, op.ID(), scope.Key()).Scan(&epoch, &chain, &canonical)
	if err != nil {
		return err
	}
	if !canonical || epoch != expected.Epoch || chain != expected.ChainGeneration {
		return errors.New("recognition context changed")
	}
	// A transient or typed absence cannot replace a previously successful decision.
	var prior string
	if err = tx.QueryRowContext(ctx, "SELECT payload FROM current_recognition WHERE operation_id=?", op.ID()).Scan(&prior); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if prior != "" && successfulRecognition(prior) && !successfulRecognitionJSON(value) {
		return tx.Commit()
	}
	if err = saveRecognition(ctx, tx, op, value, now); err != nil {
		return err
	}
	if prior == "" {
		if err = bumpRecognitionVersion(ctx, tx, scope); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO recognition_bindings VALUES(?,?,?,?) ON CONFLICT(operation_id) DO UPDATE SET scope=excluded.scope,epoch=excluded.epoch,chain_generation=excluded.chain_generation`, op.ID(), scope.Key(), epoch, chain); err != nil {
		return err
	}
	return tx.Commit()
}
func successfulRecognitionJSON(value any) bool {
	b, err := json.Marshal(value)
	return err == nil && successfulRecognition(string(b))
}
func successfulRecognition(payload string) bool {
	var rows []struct {
		Status string `json:"status"`
	}
	if json.Unmarshal([]byte(payload), &rows) != nil || len(rows) == 0 {
		return false
	}
	for _, r := range rows {
		if r.Status == "" || r.Status == "provider_unavailable" || r.Status == "unavailable" {
			return false
		}
	}
	return true
}

// RecordRecognitionAttempt schedules retries independently of successful evidence.
// Invalidated attempts never delay work in a new source or local chain epoch.
func (s *Store) RecordRecognitionAttempt(ctx context.Context, op model.Operation, expected RecognitionContext, now int64) error {
	scope := wire.Scope{ChainID: op.ChainID, ProtocolAddress: op.ProtocolAddress}
	result, err := s.DB.ExecContext(ctx, `INSERT INTO recognition_attempts(operation_id,epoch,chain_generation,attempted_at)
 SELECT o.id,s.epoch,COALESCE(g.generation,'00000000000000000001'),?
 FROM recognition_sources s LEFT JOIN chain_generations g ON g.id=s.scope JOIN operations o ON o.id=?
 WHERE s.scope=? AND s.epoch=? AND COALESCE(g.generation,'00000000000000000001')=? AND o.canonical=1
 ON CONFLICT(operation_id) DO UPDATE SET epoch=excluded.epoch,chain_generation=excluded.chain_generation,attempted_at=excluded.attempted_at`, now, op.ID(), scope.Key(), expected.Epoch, expected.ChainGeneration)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("recognition context changed")
	}
	return nil
}
