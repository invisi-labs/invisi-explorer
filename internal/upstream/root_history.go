package upstream

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/wire"
	"strconv"
)

// syncHistoricalRoots processes one page from one prior generation. Explicit
// generation/after queries survive remote cursor epochs, while local progress
// survives restart. A new current generation queues its predecessor first.
func (s *Syncer) syncHistoricalRoots(ctx context.Context, c config.Chain, current, provider string) error {
	epoch, e := wire.Uint64(current)
	if e != nil {
		return e
	}
	if epoch <= 1 {
		return nil
	}
	scope := Scope(c)
	_, e = s.Store.DB.ExecContext(ctx, "INSERT INTO root_backfill VALUES(?,?,?,'0') ON CONFLICT(scope) DO UPDATE SET epoch=excluded.epoch,next_generation=excluded.next_generation,after_sequence='0' WHERE root_backfill.epoch<>excluded.epoch", scope.Key(), current, strconv.FormatUint(epoch-1, 10))
	if e != nil {
		return e
	}
	var generation, after string
	if e = s.Store.DB.QueryRowContext(ctx, "SELECT next_generation,after_sequence FROM root_backfill WHERE scope=?", scope.Key()).Scan(&generation, &after); e != nil {
		return e
	}
	gen, e := wire.Uint64(generation)
	if e != nil {
		return e
	}
	if gen == 0 {
		return nil
	}
	if gen >= epoch {
		return fmt.Errorf("historical generation exceeds current epoch")
	}
	if after == "0" {
		var highest sql.NullString
		if e = s.Store.DB.QueryRowContext(ctx, "SELECT MAX(sequence) FROM checkpoints WHERE scope=? AND generation=?", scope.Key(), pad(generation)).Scan(&highest); e != nil {
			return e
		}
		if highest.Valid {
			v, e := strconv.ParseUint(highest.String, 10, 64)
			if e != nil {
				return e
			}
			after = strconv.FormatUint(v, 10)
		}
	}
	var page struct {
		Items      []wire.Certificate `json:"items"`
		NextCursor string             `json:"nextCursor"`
	}
	if e = s.callProvenance(ctx, c.ProvenanceURL, "provenance_getRoots", []any{map[string]any{"generation": generation, "after": after, "limit": 200}}, &page); e != nil {
		return e
	}
	if len(page.Items) > 200 {
		return fmt.Errorf("historical root response bound")
	}
	last := after
	for _, cert := range page.Items {
		if cert.Scope().Key() != scope.Key() || cert.ChainGeneration != generation || provider != "" && cert.ProviderID != provider || c.IssuerID != "" && cert.ProviderID != c.IssuerID {
			return fmt.Errorf("historical certificate scope mismatch")
		}
		seq, e := wire.Uint64(cert.RootSequence)
		prev, _ := wire.Uint64(last)
		if e != nil || seq <= prev {
			return fmt.Errorf("historical certificate order mismatch")
		}
		last = cert.RootSequence
	}
	if page.NextCursor != "" && last == after {
		return fmt.Errorf("historical root continuation did not advance")
	}
	nextGeneration, nextAfter := generation, last
	if page.NextCursor == "" {
		nextGeneration = strconv.FormatUint(gen-1, 10)
		nextAfter = "0"
	}
	tx, e := s.Store.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	for _, cert := range page.Items {
		b, e := json.Marshal(cert)
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO checkpoints VALUES(?,?,?,?,?,1) ON CONFLICT(scope,generation,sequence) DO UPDATE SET payload=excluded.payload,signature=excluded.signature", scope.Key(), pad(generation), pad(cert.RootSequence), string(b), cert.Signature); e != nil {
			return e
		}
	}
	if _, e = tx.ExecContext(ctx, "UPDATE root_backfill SET next_generation=?,after_sequence=? WHERE scope=? AND epoch=?", nextGeneration, nextAfter, scope.Key(), current); e != nil {
		return e
	}
	if len(page.Items) > 0 {
		if _, e = tx.ExecContext(ctx, "INSERT INTO readmodel_versions VALUES(?,1) ON CONFLICT(id) DO UPDATE SET version=version+1", scope.Key()); e != nil {
			return e
		}
	}
	return tx.Commit()
}
