package upstream

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/wire"
)

// BroadcasterStatus contains only public metadata. It is a service observation,
// separate from registry membership, transaction inclusion and source recognition.
type BroadcasterStatus struct {
	Status           string `json:"status"`
	APIVersion       string `json:"apiVersion,omitempty"`
	ProtocolVersion  string `json:"protocolVersion,omitempty"`
	ChainID          string `json:"chainId,omitempty"`
	Protocol         string `json:"protocol,omitempty"`
	RuntimeCodeHash  string `json:"runtimeCodeHash,omitempty"`
	Sender           string `json:"sender,omitempty"`
	FeeRecipient     string `json:"feeRecipient,omitempty"`
	SubmissionHalted bool   `json:"submissionHalted"`
}

// ProbeBroadcaster queries only an explicitly configured endpoint. Call it in an
// optional background worker, never on the protocol-indexing or page-request path.
func ProbeBroadcaster(ctx context.Context, client *http.Client, chain config.Chain) (BroadcasterStatus, error) {
	result := BroadcasterStatus{Status: "not_configured"}
	if chain.BroadcasterURL == "" {
		return result, nil
	}
	result.Status = "unavailable"
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := Call(ctx, client, chain.BroadcasterURL, "broadcaster_getStatus", []any{}, &result); err != nil {
		return BroadcasterStatus{Status: "unavailable"}, err
	}
	if result.APIVersion != "v1" || result.ProtocolVersion != "1" || result.ChainID != chain.ChainID || !strings.EqualFold(result.Protocol, chain.ProtocolAddress) || (chain.CodeHash != "" && !strings.EqualFold(result.RuntimeCodeHash, chain.CodeHash)) {
		return BroadcasterStatus{Status: "unavailable"}, errors.New("broadcaster deployment mismatch")
	}
	for _, field := range []string{result.Protocol, result.Sender, result.FeeRecipient} {
		if _, err := wire.DecodeHex(field, 20); err != nil {
			return BroadcasterStatus{Status: "unavailable"}, errors.New("invalid broadcaster identity")
		}
	}
	if _, err := wire.DecodeHex(result.RuntimeCodeHash, 32); err != nil {
		return BroadcasterStatus{Status: "unavailable"}, errors.New("invalid broadcaster deployment code hash")
	}
	result.Status = "available"
	if result.SubmissionHalted {
		result.Status = "halted"
	}
	return result, nil
}
