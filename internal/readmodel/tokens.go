package readmodel

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/invisi-labs/invisi-explorer/internal/model"
)

// TokenMetadata may contain either public ERC-20 field independently. Nil
// decimals means unknown, which is different from a verified zero precision.
type TokenMetadata struct {
	ChainID  string `json:"chainId"`
	Address  string `json:"address"`
	Symbol   string `json:"symbol,omitempty"`
	Decimals *uint8 `json:"decimals,omitempty"`
}

func discoverTokens(ctx context.Context, tx *sql.Tx, op model.Operation) error {
	addresses := []string{op.TokenAddress}
	for _, f := range op.Funding {
		addresses = append(addresses, f.TokenAddress)
	}
	if op.FundingEvidence != nil {
		for _, f := range op.FundingEvidence.Transfers {
			addresses = append(addresses, f.TokenAddress)
		}
	}
	seen := map[string]bool{}
	for _, address := range addresses {
		address = strings.ToLower(address)
		if len(address) != 42 || !strings.HasPrefix(address, "0x") || address == "0x0000000000000000000000000000000000000000" || seen[address] {
			continue
		}
		if _, err := hex.DecodeString(address[2:]); err != nil {
			continue
		}
		seen[address] = true
		if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO token_metadata(chain_id,address) VALUES(?,?)", op.ChainID, address); err != nil {
			return err
		}
	}
	return nil
}

// Seed the discovery queue once when upgrading an existing Explorer database.
// Keyset batches keep memory bounded; metadata never changes canonical facts.
func (s *Store) backfillTokenDiscovery(ctx context.Context) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM readmodel_versions WHERE id='token_metadata_discovery'").Scan(&exists); err != nil {
		return err
	}
	if exists > 0 {
		return nil
	}
	after := ""
	for {
		rows, err := tx.QueryContext(ctx, "SELECT id,payload FROM operations WHERE canonical=1 AND id>? ORDER BY id LIMIT 256", after)
		if err != nil {
			return err
		}
		var ops []model.Operation
		for rows.Next() {
			var payload string
			if err = rows.Scan(&after, &payload); err != nil {
				rows.Close()
				return err
			}
			var op model.Operation
			if err = json.Unmarshal([]byte(payload), &op); err != nil {
				rows.Close()
				return err
			}
			ops = append(ops, op)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, op := range ops {
			if err = discoverTokens(ctx, tx, op); err != nil {
				return err
			}
		}
		if len(ops) < 256 {
			break
		}
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO readmodel_versions(id,version) VALUES('token_metadata_discovery',1)"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) DueTokens(ctx context.Context, chain string, now int64, limit int, excluded ...string) ([]TokenMetadata, error) {
	query := "SELECT chain_id,address FROM token_metadata WHERE chain_id=? AND retry_at<=?"
	args := []any{chain, now}
	if len(excluded) > 0 {
		query += " AND address NOT IN (" + strings.TrimSuffix(strings.Repeat("?,", len(excluded)), ",") + ")"
		for _, address := range excluded {
			args = append(args, strings.ToLower(address))
		}
	}
	query += " ORDER BY retry_at,address LIMIT ?"
	args = append(args, limit)
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []TokenMetadata{}
	for rows.Next() {
		var t TokenMetadata
		if err = rows.Scan(&t.ChainID, &t.Address); err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}
func (s *Store) SaveTokenMetadata(ctx context.Context, t TokenMetadata, retryAt int64) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO token_metadata(chain_id,address,symbol,decimals,retry_at) VALUES(?,?,?,?,?)
 ON CONFLICT(chain_id,address) DO UPDATE SET symbol=CASE WHEN excluded.symbol='' THEN token_metadata.symbol ELSE excluded.symbol END,decimals=COALESCE(excluded.decimals,token_metadata.decimals),retry_at=excluded.retry_at`, t.ChainID, strings.ToLower(t.Address), t.Symbol, t.Decimals, retryAt)
	return err
}
func (s *Store) TokenMetadata(ctx context.Context) ([]TokenMetadata, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT chain_id,address,symbol,decimals FROM token_metadata WHERE symbol<>'' OR decimals IS NOT NULL ORDER BY chain_id,address")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []TokenMetadata{}
	for rows.Next() {
		var t TokenMetadata
		var d sql.NullInt64
		if err = rows.Scan(&t.ChainID, &t.Address, &t.Symbol, &d); err != nil {
			return nil, err
		}
		if d.Valid {
			v := uint8(d.Int64)
			t.Decimals = &v
		}
		result = append(result, t)
	}
	return result, rows.Err()
}
