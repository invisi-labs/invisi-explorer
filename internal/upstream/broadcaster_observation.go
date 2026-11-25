package upstream

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	"github.com/invisi-labs/invisi-explorer/internal/publicchain"
	"github.com/invisi-labs/invisi-explorer/internal/wire"
)

func ObserveBroadcaster(ctx context.Context, client *http.Client, chain config.Chain, record publicchain.Broadcaster) model.BroadcasterObservation {
	out := model.BroadcasterObservation{ExpectedCodeHash: chain.CodeHash, Status: "unavailable", CheckedAt: time.Now().Unix(), IdentityStatus: "unverified", Tokens: []model.BroadcasterToken{}, TokensStatus: "unavailable"}
	if !record.Enabled {
		out.Status = "not_probed"
		return out
	}
	if ValidateBroadcasterEndpoint(record.Endpoint) != nil {
		out.Status = "invalid_endpoint"
		return out
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	started := time.Now()
	var service model.BroadcasterStatus
	err := Call(ctx, client, record.Endpoint, "broadcaster_getStatus", []any{}, &service)
	out.ResponseMillis = time.Since(started).Milliseconds()
	if err != nil {
		return out
	}
	if !broadcasterIdentity(service, chain, record) {
		out.Status = "identity_mismatch"
		out.IdentityStatus = "mismatch"
		return out
	}
	out.Service = &service
	out.IdentityStatus = "matched"
	out.Status = "available"
	if *service.SubmissionHalted {
		out.Status = "halted"
	}
	out.TokensStatus = "complete"
	after := ""
	for page := 0; page < 20; page++ {
		var result struct {
			model.BroadcasterStatus
			Tokens    []model.BroadcasterToken `json:"tokens"`
			Next      string                   `json:"next"`
			CheckedAt string                   `json:"checkedAt"`
		}
		q := map[string]any{"limit": 100}
		if after != "" {
			q["after"] = after
		}
		err = Call(ctx, client, record.Endpoint, "broadcaster_getTokens", []any{q}, &result)
		if err != nil {
			var rpc *RPCError
			out.TokensStatus = "unavailable"
			if errors.As(err, &rpc) && rpc.Code == -32601 {
				out.TokensStatus = "not_provided"
			}
			break
		}
		if !broadcasterIdentity(result.BroadcasterStatus, chain, record) || len(result.Tokens) > 100 {
			out.TokensStatus = "invalid"
			break
		}
		previous := after
		for _, token := range result.Tokens {
			token.TokenAddress = strings.ToLower(token.TokenAddress)
			if !validProbeAddress(token.TokenAddress) || token.TokenAddress <= previous || (token.Status != "available" && token.Status != "paused" && token.Status != "pricing_unavailable") || token.Supported != (token.Status == "available") {
				out.TokensStatus = "invalid"
				break
			}
			if token.PriceValidUntil != "" {
				if _, e := time.Parse(time.RFC3339, token.PriceValidUntil); e != nil {
					out.TokensStatus = "invalid"
					break
				}
			}
			previous = token.TokenAddress
			out.Tokens = append(out.Tokens, token)
		}
		if out.TokensStatus == "invalid" {
			break
		}
		out.TokensCheckedAt = time.Now().Unix()
		if result.Next == "" {
			break
		}
		if len(result.Tokens) != 100 || strings.ToLower(result.Next) != previous {
			out.TokensStatus = "invalid"
			break
		}
		after = previous
		if page == 19 {
			out.TokensStatus = "partial"
		}
	}
	if out.TokensStatus != "complete" {
		out.Tokens = []model.BroadcasterToken{}
	}
	return out
}
func validProbeAddress(value string) bool {
	_, err := wire.DecodeHex(value, 20)
	return err == nil && value != "0x0000000000000000000000000000000000000000"
}
func broadcasterIdentity(s model.BroadcasterStatus, c config.Chain, r publicchain.Broadcaster) bool {
	if s.APIVersion != "v1" || s.ProtocolVersion != "1" || s.ChainID != c.ChainID || !strings.EqualFold(s.Protocol, c.ProtocolAddress) || s.SubmissionHalted == nil || !strings.EqualFold(s.FeeRecipient, r.FeeRecipient) || !validProbeAddress(s.Sender) || !validProbeAddress(s.Protocol) {
		return false
	}
	if _, err := wire.DecodeHex(s.RuntimeCodeHash, 32); err != nil || s.RuntimeCodeHash == wire.ZeroHash || c.CodeHash != "" && !strings.EqualFold(s.RuntimeCodeHash, c.CodeHash) {
		return false
	}
	if s.PaymentRouters != nil {
		if len(*s.PaymentRouters) > 16 {
			return false
		}
		for _, p := range *s.PaymentRouters {
			if !validProbeAddress(p.Router) || !strings.EqualFold(p.Core, c.ProtocolAddress) {
				return false
			}
		}
	}
	return true
}
