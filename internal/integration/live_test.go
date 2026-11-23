package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/readmodel"
	"github.com/invisi-labs/invisi-explorer/internal/upstream"
)

// This opt-in check creates a fresh Explorer index. It never opens the configured
// query database or any upstream service database, and never submits a proof/payment.
func TestLiveReadOnlyUpstreamContracts(t *testing.T) {
	dir := os.Getenv("EXPLORER_LIVE_CONFIG_DIR")
	if dir == "" {
		t.Skip("set EXPLORER_LIVE_CONFIG_DIR for explicit live read-only checks")
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Chains) == 0 {
		t.Fatal("live configuration has no chain scopes")
	}
	db, err := readmodel.Open(filepath.Join(t.TempDir(), "explorer.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client := upstream.Client()
	defer client.CloseIdleConnections()
	syncer := &upstream.Syncer{Store: db, Client: client}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	for _, chain := range cfg.Chains {
		if _, err := syncer.Deployment(ctx, chain); err != nil {
			t.Fatalf("chain %s deployment: %v", chain.ChainID, err)
		}
		if err := syncer.Tick(ctx, chain); err != nil {
			t.Fatalf("chain %s public fact ingestion: %v", chain.ChainID, err)
		}
		if chain.ProvenanceURL != "" {
			if err := syncer.SyncRoots(ctx, chain); err != nil {
				t.Fatalf("chain %s published roots: %v", chain.ChainID, err)
			}
		}
		if chain.BroadcasterURL != "" {
			if _, err := upstream.ProbeBroadcaster(ctx, client, chain); err != nil {
				t.Fatalf("chain %s public broadcaster status: %v", chain.ChainID, err)
			}
		}
	}
}
