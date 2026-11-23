package upstream

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	"github.com/invisi-labs/invisi-explorer/internal/readmodel"
	"github.com/invisi-labs/invisi-explorer/internal/wire"
)

func TestUnfundedCreditManagementDoesNotNeedRecognitionService(t *testing.T) {
	store, err := readmodel.Open(filepath.Join(t.TempDir(), "explorer.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	syncer := &Syncer{Store: store}
	for _, kind := range []string{"createCredit", "updateCredit", "revokeCredit"} {
		value, err := syncer.Recognition(context.Background(), config.Chain{}, model.Operation{Kind: kind})
		raw, _ := json.Marshal(value)
		if err != nil || !strings.Contains(string(raw), `"status":"not_applicable"`) {
			t.Errorf("%s: %s %v", kind, raw, err)
		}
	}
	value, err := syncer.Recognition(context.Background(), config.Chain{}, model.Operation{Kind: "createCredit", NoteOutputs: []model.NoteOutput{{NoteCommitment: wire.UintField(1)}}})
	raw, _ := json.Marshal(value)
	if err != nil || strings.Contains(string(raw), "not_applicable") || !strings.Contains(string(raw), "provider_unavailable") {
		t.Fatalf("funded creation lost recognition boundary: %s %v", raw, err)
	}
}
