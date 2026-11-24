package service

import (
	"context"
	"strings"

	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/readmodel"
)

func cachedMetadata(ctx context.Context, store *readmodel.Store, c config.Config) (any, error) {
	cached, err := store.TokenMetadata(ctx)
	if err != nil {
		return nil, err
	}
	result := metadata(c).(map[string]any)
	tokens := []any{}
	seen := map[string]bool{}
	chains := map[string]bool{}
	for _, chain := range c.Chains {
		chains[chain.ChainID] = true
	}
	for _, token := range c.Tokens {
		tokens = append(tokens, token)
		seen[token.ChainID+"/"+strings.ToLower(token.Address)] = true
	}
	for _, token := range cached {
		if chains[token.ChainID] && !seen[token.ChainID+"/"+strings.ToLower(token.Address)] {
			tokens = append(tokens, token)
		}
	}
	result["tokens"] = tokens
	return result, nil
}
