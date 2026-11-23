package readmodel

import (
	"context"
	"database/sql"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	"github.com/invisi-labs/invisi-explorer/internal/wire"
	"path/filepath"
	"testing"
)

func TestStagePublicationAndRollback(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "explorer.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	scope := wire.Scope{ChainID: "97", ProtocolAddress: "0x0000000000000000000000000000000000000001"}
	op := model.Operation{ChainID: scope.ChainID, ProtocolAddress: scope.ProtocolAddress, BlockNumber: "1", BlockHash: wire.ZeroHash, TxHash: wire.ZeroHash, OperationIndex: "0", Timestamp: "1", Success: true, NoteOutputs: []model.NoteOutput{{NoteCommitment: wire.UintField(3), TreeID: "1", LeafIndex: "0"}}}
	if err = s.Stage(ctx, scope, []model.Operation{op}, "next", "1"); err != nil {
		t.Fatal(err)
	}
	var n int
	s.DB.QueryRow("SELECT COUNT(*) FROM operations").Scan(&n)
	if n != 0 {
		t.Fatal("partial page leaked")
	}
	if err = s.Publish(ctx, scope, model.Anchor{BlockNumber: "1", BlockHash: wire.ZeroHash}); err != nil {
		t.Fatal(err)
	}
	s.DB.QueryRow("SELECT COUNT(*) FROM public_notes").Scan(&n)
	if n != 1 {
		t.Fatal("missing public projection")
	}
	if err = s.Rollback(ctx, scope, 0); err != nil {
		t.Fatal(err)
	}
	s.DB.QueryRow("SELECT canonical FROM operations").Scan(&n)
	if n != 0 {
		t.Fatal("orphan deleted or canonical")
	}
	s.DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE name='proof_jobs'").Scan(&n)
	if n != 0 {
		t.Fatal("private service schema")
	}
}

func TestRefusesForeignDatabaseAndKeepsOwnershipOnReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "foreign.db")
	db, e := sql.Open("sqlite", path)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("CREATE TABLE proof_jobs(id TEXT)"); e != nil {
		t.Fatal(e)
	}
	db.Close()
	if s, e := Open(path); e == nil {
		s.Close()
		t.Fatal("accepted another service database")
	}
	path = filepath.Join(t.TempDir(), "own.db")
	s, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	s.Close()
	s, e = Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
}

func TestCompleteWindowRejectsForeignScopeAtomically(t *testing.T) {
	ctx := context.Background()
	s, e := Open(filepath.Join(t.TempDir(), "window.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	scope := wire.Scope{ChainID: "97", ProtocolAddress: "0x0000000000000000000000000000000000000001"}
	before := model.Anchor{BlockNumber: "1", BlockHash: wire.ZeroHash}
	if e = s.Stage(ctx, scope, nil, "", "1"); e != nil {
		t.Fatal(e)
	}
	if e = s.Publish(ctx, scope, before); e != nil {
		t.Fatal(e)
	}
	op := model.Operation{ChainID: "1", ProtocolAddress: scope.ProtocolAddress, BlockNumber: "2", BlockHash: wire.ZeroHash, TxHash: wire.ZeroHash, OperationIndex: "0"}
	if e = s.PublishWindow(ctx, scope, before, model.Anchor{BlockNumber: "2", BlockHash: wire.ZeroHash}, []model.Operation{op}); e == nil {
		t.Fatal("foreign scope was published")
	}
	var count int
	s.DB.QueryRow("SELECT COUNT(*) FROM operations").Scan(&count)
	if count != 0 {
		t.Fatal("partial projection survived rejection")
	}
	var height string
	s.DB.QueryRow("SELECT block_number FROM chain_checkpoints WHERE id=?", scope.Key()).Scan(&height)
	if height != pad("1") {
		t.Fatal("rejected window advanced checkpoint")
	}
}
