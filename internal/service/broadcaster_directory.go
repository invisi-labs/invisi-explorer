package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	"github.com/invisi-labs/invisi-explorer/internal/publicchain"
	"github.com/invisi-labs/invisi-explorer/internal/readmodel"
	"github.com/invisi-labs/invisi-explorer/internal/upstream"
	"github.com/invisi-labs/invisi-explorer/internal/wire"
)

const broadcasterDirectorySource = "broadcaster_directory"

func cachedBroadcasters(ctx context.Context, store *readmodel.Store, chains []config.Chain) ([]model.BroadcasterDirectory, error) {
	result := []model.BroadcasterDirectory{}
	for _, chain := range chains {
		directory := model.BroadcasterDirectory{ChainID: chain.ChainID, ProtocolAddress: strings.ToLower(chain.ProtocolAddress), Status: "not_checked", Items: []model.BroadcasterEntry{}}
		var body string
		err := store.DB.QueryRowContext(ctx, "SELECT payload FROM source_status WHERE scope=? AND source=?", upstream.Scope(chain).Key(), broadcasterDirectorySource).Scan(&body)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if err == nil {
			if err = json.Unmarshal([]byte(body), &directory); err != nil {
				return nil, err
			}
		}
		result = append(result, directory)
	}
	return result, nil
}

// Refresh the authenticated registration independently of indexing and HTTP
// handlers. Failure retains the last observation with explicit discovery status.
func refreshBroadcasters(ctx context.Context, store *readmodel.Store, live *publicState, probeClient *http.Client, chain config.Chain) error {
	previous, _ := cachedBroadcasters(ctx, store, []config.Chain{chain})
	fail := func(err error) error {
		if len(previous) > 0 {
			old := previous[0]
			old.Status = "unavailable"
			_ = store.SourceStatus(ctx, upstream.Scope(chain), broadcasterDirectorySource, "unavailable", old)
		}
		return err
	}
	var number, hash string
	if err := store.DB.QueryRowContext(ctx, "SELECT block_number,block_hash FROM chain_checkpoints WHERE id=?", upstream.Scope(chain).Key()).Scan(&number, &hash); err != nil {
		return err
	}
	number = strings.TrimLeft(number, "0")
	if number == "" {
		number = "0"
	}
	anchor := publicchain.Anchor{BlockNumber: number, BlockHash: hash}
	directory := model.BroadcasterDirectory{ChainID: chain.ChainID, ProtocolAddress: strings.ToLower(chain.ProtocolAddress), Anchor: anchor, Status: "verified", Items: []model.BroadcasterEntry{}}
	deployment, err := live.syncer.Deployment(ctx, chain)
	if err != nil {
		return fail(err)
	}
	if chain.CodeHash == "" {
		chain.CodeHash = deployment.CodeHash
		if chain.CodeHash == "" {
			chain.CodeHash = deployment.Protocol.RuntimeCodeHash
		}
	}
	after := ""
	for page := 0; page < 10; page++ {
		result, err := live.Broadcasters(ctx, chain.ChainID, chain.ProtocolAddress, publicchain.DirectoryQuery{Anchor: anchor, After: after, Limit: 100})
		if err != nil {
			return fail(err)
		}
		if result.Status == "not_configured" {
			directory.Status = "not_configured"
			directory.Complete = true
			break
		}
		if result.Status != "verified" || result.Anchor != anchor || !probeAddress(result.RegistryAddress) || !probeHash(result.RegistryCodeHash) {
			return fail(errors.New("invalid registry observation"))
		}
		if page > 0 && (!strings.EqualFold(directory.RegistryAddress, result.RegistryAddress) || !strings.EqualFold(directory.RegistryCodeHash, result.RegistryCodeHash)) {
			return fail(errors.New("registry changed during discovery"))
		}
		directory.RegistryAddress = result.RegistryAddress
		directory.RegistryCodeHash = result.RegistryCodeHash
		prior := after
		for _, record := range result.Items {
			record.Operator = strings.ToLower(record.Operator)
			if !probeAddress(record.Operator) || record.Operator <= prior || !probeAddress(record.QuoteSigner) || !probeAddress(record.FeeRecipient) {
				return fail(errors.New("invalid registered identity"))
			}
			prior = record.Operator
			directory.Items = append(directory.Items, model.BroadcasterEntry{Broadcaster: record, ChainID: chain.ChainID, ProtocolAddress: strings.ToLower(chain.ProtocolAddress), ID: record.Operator, Observation: model.BroadcasterObservation{Status: "not_probed", Tokens: []model.BroadcasterToken{}, TokensStatus: "not_checked"}})
		}
		if result.NextOperator == "" {
			directory.Complete = true
			break
		}
		if len(result.Items) != 100 || strings.ToLower(result.NextOperator) != prior {
			return fail(errors.New("invalid registry pagination"))
		}
		after = prior
	}
	if !directory.Complete {
		directory.Status = "partial"
	}
	if err := live.verify(ctx, chain, anchor); err != nil {
		return fail(err)
	}
	directory.CheckedAt = time.Now().Unix()
	// Reuse observations only while all endpoint/deployment/registration bindings
	// remain unchanged. This avoids a polling flash without trusting old identities.
	if len(previous) > 0 {
		for i := range directory.Items {
			current := &directory.Items[i]
			for _, old := range previous[0].Items {
				if current.Enabled && strings.EqualFold(current.Operator, old.Operator) && current.Endpoint == old.Endpoint && strings.EqualFold(current.QuoteSigner, old.QuoteSigner) && strings.EqualFold(current.FeeRecipient, old.FeeRecipient) && strings.EqualFold(old.Observation.ExpectedCodeHash, chain.CodeHash) {
					current.Observation = old.Observation
					break
				}
			}
		}
	}
	// Publish discovery before probing so slow endpoints never hide registration.
	if err := store.SourceStatus(ctx, upstream.Scope(chain), broadcasterDirectorySource, directory.Status, directory); err != nil {
		return err
	}
	jobs := make(chan int)
	done := make(chan struct{}, 4)
	for range 4 {
		go func() {
			defer func() { done <- struct{}{} }()
			for i := range jobs {
				record := directory.Items[i].Broadcaster
				if ctx.Err() != nil {
					continue
				}
				directory.Items[i].Observation = upstream.ObserveBroadcaster(ctx, probeClient, chain, record)
			}
		}()
	}
	pending := []int{}
	for i := range directory.Items {
		if directory.Items[i].Enabled {
			pending = append(pending, i)
		}
	}
	sort.SliceStable(pending, func(i, j int) bool {
		return directory.Items[pending[i]].Observation.CheckedAt < directory.Items[pending[j]].Observation.CheckedAt
	})
	for _, i := range pending {
		select {
		case jobs <- i:
		case <-ctx.Done():
		}
	}
	close(jobs)
	for range 4 {
		<-done
	}
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	// Tokens discovered through service policy join the ordinary bounded RPC
	// metadata queue; symbol/decimals never come from an unauthenticated endpoint.
	discovered := map[string]bool{}
	for _, entry := range directory.Items {
		for _, token := range entry.Observation.Tokens {
			if len(discovered) >= 2000 {
				break
			}
			if discovered[token.TokenAddress] {
				continue
			}
			discovered[token.TokenAddress] = true
			_, _ = store.DB.ExecContext(saveCtx, "INSERT OR IGNORE INTO token_metadata(chain_id,address) VALUES(?,?)", chain.ChainID, strings.ToLower(token.TokenAddress))
		}
	}
	return store.SourceStatus(saveCtx, upstream.Scope(chain), broadcasterDirectorySource, directory.Status, directory)
}
func probeAddress(v string) bool {
	return config.ValidateAddress(v) == nil && strings.ToLower(v) != "0x0000000000000000000000000000000000000000"
}
func probeHash(v string) bool {
	_, err := wire.DecodeHex(v, 32)
	return err == nil && strings.ToLower(v) != "0x"+strings.Repeat("0", 64)
}
