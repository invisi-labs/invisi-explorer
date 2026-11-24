package explorer

import (
	"context"
	"errors"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	"net/url"
	"testing"
)

func TestTransactionObservationFailureIsReadOnceForMultipleOperations(t *testing.T) {
	e, s, scope, ops := fixture(t)
	ctx := context.Background()
	extra := ops[1]
	extra.OperationIndex = "1"
	if err := s.Stage(ctx, scope, []model.Operation{extra}, "", "2"); err != nil {
		t.Fatal(err)
	}
	if err := s.Publish(ctx, scope, model.Anchor{BlockNumber: "2", BlockHash: extra.BlockHash}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	e.options.TransactionObservation = func(context.Context, model.Operation) (*model.TransactionObservation, error) {
		calls++
		return nil, errors.New("unavailable")
	}
	if _, err := e.Public(ctx, "/transactions/"+extra.TxHash, url.Values{}, nil); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("repeated failed calls: %d", calls)
	}
}
