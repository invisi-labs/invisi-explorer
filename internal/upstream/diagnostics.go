package upstream

import (
	"context"
	"encoding/json"
	"github.com/invisi-labs/invisi-explorer/internal/config"
	"strconv"
	"time"
)

// Diagnostics reads actual recorded attempts; HTTP requests never refresh timestamps
// or contact services merely to paint a healthy status.
func (s *Syncer) Diagnostics(ctx context.Context, chains []config.Chain) (any, error) {
	items := []any{}
	for _, c := range chains {
		k := Scope(c).Key()
		record := map[string]any{"id": k, "chainId": c.ChainID, "protocolAddress": c.ProtocolAddress, "status": "not_checked", "rpcConfigured": c.RPCURL != "", "provenanceConfigured": c.ProvenanceURL != "", "broadcasterConfigured": c.BroadcasterURL != ""}
		sources := map[string]any{}
		for _, source := range []string{"protocol", "provenance", "broadcaster"} {
			state := "not_checked"
			var payload string
			var updated int64
			if s.Store.DB.QueryRowContext(ctx, "SELECT status,updated_at,payload FROM source_status WHERE scope=? AND source=?", k, source).Scan(&state, &updated, &payload) == nil {
				if time.Now().Unix()-updated > 60 {
					state = "unavailable"
				}
			}
			var value map[string]any
			_ = json.Unmarshal([]byte(payload), &value)
			if source == "protocol" {
				record["status"] = state
				record["updatedAt"] = strconv.FormatInt(updated, 10)
				for _, field := range []string{"indexedHead", "rpcIndexedHead", "chainHead", "finality", "lastSuccessfulAt"} {
					if v, ok := value[field]; ok {
						record[field] = v
					}
				}
			} else {
				if source == "provenance" && c.ProvenanceURL == "" || source == "broadcaster" && c.BroadcasterURL == "" {
					state = "not_configured"
				}
				sources[source] = map[string]any{"status": state, "updatedAt": strconv.FormatInt(updated, 10), "observation": value}
			}
		}
		record["sources"] = sources
		items = append(items, record)
	}
	return map[string]any{"items": items, "source": "explorer", "secretsAccepted": false}, nil
}
