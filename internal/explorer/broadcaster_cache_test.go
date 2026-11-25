package explorer

import (
	"context"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	"github.com/invisi-labs/invisi-explorer/internal/publicchain"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestCachedBroadcasterDirectoryPagesAndIdentityChanges(t *testing.T) {
	e, _, scope, _ := fixture(t)
	address1, address2 := "0x"+strings.Repeat("1", 40), "0x"+strings.Repeat("2", 40)
	directory := model.BroadcasterDirectory{ChainID: scope.ChainID, ProtocolAddress: scope.ProtocolAddress, Status: "verified", Complete: true, CheckedAt: time.Now().Unix(), Items: []model.BroadcasterEntry{}}
	for _, operator := range []string{address1, address2} {
		directory.Items = append(directory.Items, model.BroadcasterEntry{Broadcaster: publicchain.Broadcaster{Operator: operator, Endpoint: "https://public.example", Enabled: true}, ChainID: scope.ChainID, ProtocolAddress: scope.ProtocolAddress, ID: operator, Observation: model.BroadcasterObservation{Status: "available", CheckedAt: time.Now().Unix()}})
	}
	e.options.BroadcasterDirectory = func(context.Context) ([]model.BroadcasterDirectory, error) {
		return []model.BroadcasterDirectory{directory}, nil
	}
	value, err := e.Public(context.Background(), "/broadcasters", url.Values{"limit": {"1"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	page := value.(map[string]any)
	cursor := page["nextCursor"].(string)
	directory.Items[0].Observation.Status = "unavailable"
	value, err = e.Public(context.Background(), "/broadcasters", url.Values{"limit": {"1"}, "cursor": {cursor}}, nil)
	if err != nil {
		t.Fatal("runtime poll invalidated registration cursor", err)
	}
	if value.(map[string]any)["items"].([]model.BroadcasterEntry)[0].Operator != address2 {
		t.Fatal(value)
	}
	directory.Items[0].Endpoint = "https://changed.example"
	if _, err = e.Public(context.Background(), "/broadcasters", url.Values{"limit": {"1"}, "cursor": {cursor}}, nil); err == nil {
		t.Fatal("registration change accepted with old cursor")
	}
	directory.Items[0].Observation.CheckedAt = 1
	value, err = e.Public(context.Background(), "/broadcasters/"+address1, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !value.(map[string]any)["observation"].(map[string]any)["stale"].(bool) {
		t.Fatal(value)
	}
}

func TestBroadcasterServiceDetailKeepsAllNetworkAndDeploymentRegistrations(t *testing.T) {
	e, _, scope, _ := fixture(t)
	operator := "0x" + strings.Repeat("1", 40)
	secondCore := "0x" + strings.Repeat("2", 40)
	directories := []model.BroadcasterDirectory{}
	for _, pair := range [][2]string{{scope.ChainID, scope.ProtocolAddress}, {"1", scope.ProtocolAddress}, {scope.ChainID, secondCore}} {
		directories = append(directories, model.BroadcasterDirectory{ChainID: pair[0], ProtocolAddress: pair[1], Status: "verified", Complete: true, CheckedAt: time.Now().Unix(), Items: []model.BroadcasterEntry{{Broadcaster: publicchain.Broadcaster{Operator: operator, Endpoint: "https://relay.example/v1/" + pair[0], Enabled: true}, ChainID: pair[0], ProtocolAddress: pair[1], ID: operator}}})
	}
	e.options.BroadcasterDirectory = func(context.Context) ([]model.BroadcasterDirectory, error) { return directories, nil }
	value, err := e.Public(context.Background(), "/broadcasters/"+operator, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	entries := value.(map[string]any)["occurrences"].([]model.BroadcasterEntry)
	if len(entries) != 3 {
		t.Fatalf("expected all three registrations, got %d", len(entries))
	}
	scoped, err := e.Public(context.Background(), "/broadcasters/"+operator, url.Values{"chainId": {scope.ChainID}, "protocolAddress": {scope.ProtocolAddress}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := scoped.(map[string]any)["occurrences"]; exists {
		t.Fatal("scoped API query unexpectedly mixed registrations")
	}
}
