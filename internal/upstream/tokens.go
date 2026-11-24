package upstream

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/readmodel"
	"github.com/invisi-labs/invisi-explorer/internal/wire"
)

func tokenBytes(encoded string) []byte {
	if !strings.HasPrefix(encoded, "0x") || len(encoded) > 386 {
		return nil
	}
	b, err := hex.DecodeString(encoded[2:])
	if err != nil {
		return nil
	}
	return b
}
func decodeTokenDecimals(encoded string) *uint8 {
	b := tokenBytes(encoded)
	if len(b) != 32 || !bytes.Equal(b[:31], make([]byte, 31)) {
		return nil
	}
	d := b[31]
	return &d
}
func decodeTokenSymbol(encoded string) string {
	b := tokenBytes(encoded)
	var text []byte
	if len(b) == 32 {
		text = bytes.TrimRight(b, "\x00")
	} else {
		if len(b) < 96 || len(b)%32 != 0 || !bytes.Equal(b[:31], make([]byte, 31)) || b[31] != 32 || !bytes.Equal(b[32:63], make([]byte, 31)) {
			return ""
		}
		n := int(b[63])
		if n < 1 || n > 64 || len(b) != 64+(n+31)/32*32 {
			return ""
		}
		text = b[64 : 64+n]
		if !bytes.Equal(b[64+n:], make([]byte, len(b)-64-n)) {
			return ""
		}
	}
	if len(text) == 0 || len(text) > 64 || !utf8.Valid(text) {
		return ""
	}
	for _, r := range string(text) {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return ""
		}
	}
	return strings.TrimSpace(string(text))
}

func tokenDeploymentMatches(d Deployment, c config.Chain) bool {
	return d.ChainID == c.ChainID && strings.EqualFold(d.Protocol.Address, c.ProtocolAddress) && (c.CodeHash == "" || strings.EqualFold(d.Protocol.RuntimeCodeHash, c.CodeHash))
}

// ReadTokenMetadata uses only existing allowlisted Protocol RPC reads. Metadata
// is display information; a failed field stays unknown rather than becoming 18.
func (s *Syncer) ReadTokenMetadata(ctx context.Context, c config.Chain, address string) (readmodel.TokenMetadata, error) {
	out := readmodel.TokenMetadata{ChainID: c.ChainID, Address: strings.ToLower(address)}
	if _, err := wire.DecodeHex(address, 20); err != nil {
		return out, err
	}
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	var anchor struct {
		Deployment Deployment `json:"deployment"`
		HeaderHash string     `json:"headerHash"`
		Header     struct {
			Number string `json:"number"`
		} `json:"header"`
	}
	if err := Call(ctx, s.Client, c.RPCURL, "invisi_readChainState", []any{map[string]any{"kind": "header", "block": "finalized"}}, &anchor); err != nil {
		return out, err
	}
	if !tokenDeploymentMatches(anchor.Deployment, c) || !strings.HasPrefix(anchor.Header.Number, "0x") {
		return out, errors.New("metadata chain binding mismatch")
	}
	n, err := strconv.ParseUint(anchor.Header.Number[2:], 16, 64)
	if err != nil {
		return out, err
	}
	if _, err = wire.DecodeHex(anchor.HeaderHash, 32); err != nil {
		return out, err
	}
	for _, field := range []struct {
		selector string
		symbol   bool
	}{{"0x95d89b41", true}, {"0x313ce567", false}} {
		var result struct {
			Deployment Deployment `json:"deployment"`
			Data       string     `json:"data"`
		}
		fieldCtx, stop := context.WithTimeout(ctx, 2*time.Second)
		err = Call(fieldCtx, s.Client, c.RPCURL, "invisi_readChainState", []any{map[string]any{"kind": "call", "block": anchor.Header.Number, "address": out.Address, "data": field.selector, "gas": 100000}}, &result)
		stop()
		if err != nil {
			continue
		}
		if !tokenDeploymentMatches(result.Deployment, c) {
			return readmodel.TokenMetadata{}, errors.New("metadata chain binding mismatch")
		}
		if field.symbol {
			out.Symbol = decodeTokenSymbol(result.Data)
		} else {
			out.Decimals = decodeTokenDecimals(result.Data)
		}
	}
	current, err := s.Header(ctx, c, n)
	if err != nil || !strings.EqualFold(current, anchor.HeaderHash) {
		return readmodel.TokenMetadata{}, errors.New("metadata anchor changed")
	}
	return out, nil
}

// RefreshTokens performs at most eight token lookups off the indexing path.
func (s *Syncer) RefreshTokens(ctx context.Context, c config.Chain, configured []config.Token, now time.Time) error {
	excluded := []string{}
	for _, manual := range configured {
		if manual.ChainID == c.ChainID {
			excluded = append(excluded, manual.Address)
		}
	}
	due, err := s.Store.DueTokens(ctx, c.ChainID, now.Unix(), 8, excluded...)
	if err != nil {
		return err
	}
	for _, token := range due {
		ttl := 5 * time.Minute
		resolved, readErr := s.ReadTokenMetadata(ctx, c, token.Address)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if readErr == nil {
			token = resolved
		}
		if token.Symbol != "" && token.Decimals != nil {
			ttl = 24 * time.Hour
		}
		if err = s.Store.SaveTokenMetadata(ctx, token, now.Add(ttl).Unix()); err != nil {
			return err
		}
	}
	return nil
}
