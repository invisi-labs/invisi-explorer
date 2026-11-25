package upstream

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	"github.com/invisi-labs/invisi-explorer/internal/publicchain"
)

type observationTransport func(*http.Request) (*http.Response, error)

func (f observationTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestRegisteredBroadcasterPublicObservations(t *testing.T) {
	address := "0x" + strings.Repeat("1", 40)
	hash := "0x" + strings.Repeat("2", 64)
	c := config.Chain{ChainID: "97", ProtocolAddress: address, CodeHash: hash}
	record := publicchain.Broadcaster{Operator: address, Endpoint: "https://public.example/v1/chains/97", QuoteSigner: address, FeeRecipient: address, Enabled: true}
	mode := "valid"
	calls := 0
	status := map[string]any{"apiVersion": "v1", "protocolVersion": "1", "chainId": "97", "protocol": address, "runtimeCodeHash": hash, "sender": address, "feeRecipient": address, "submissionHalted": false}
	client := &http.Client{Transport: observationTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		var request struct {
			Method string `json:"method"`
		}
		json.NewDecoder(r.Body).Decode(&request)
		var result any
		switch request.Method {
		case "broadcaster_getStatus":
			result = status
		case "broadcaster_getTokens":
			if mode == "legacy" {
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"jsonrpc":"2.0","id":1,"error":{"code":-32601}}`))}, nil
			}
			tokenPage := map[string]any{}
			for k, v := range status {
				tokenPage[k] = v
			}
			tokenPage["tokens"] = []any{map[string]any{"tokenAddress": "0x" + strings.Repeat("3", 40), "supported": true, "status": "available"}}
			if mode == "wrong_token_scope" {
				tokenPage["chainId"] = "1"
			}
			result = tokenPage
		default:
			t.Fatalf("unexpected non-discovery call %s", request.Method)
		}
		data, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(data)))}, nil
	})}
	out := ObserveBroadcaster(context.Background(), client, c, record)
	if out.Status != "available" || out.TokensStatus != "complete" || len(out.Tokens) != 1 {
		t.Fatal(out)
	}
	if calls != 2 {
		t.Fatalf("public observation made %d calls, want status and token discovery only", calls)
	}
	mode = "legacy"
	out = ObserveBroadcaster(context.Background(), client, c, record)
	if out.Status != "available" || out.TokensStatus != "not_provided" {
		t.Fatal(out)
	}
	mode = "wrong_token_scope"
	out = ObserveBroadcaster(context.Background(), client, c, record)
	if out.TokensStatus != "invalid" || len(out.Tokens) != 0 {
		t.Fatal(out)
	}
	status["feeRecipient"] = "0x" + strings.Repeat("4", 40)
	out = ObserveBroadcaster(context.Background(), client, c, record)
	if out.Status != "identity_mismatch" || out.Service != nil {
		t.Fatal(out)
	}
	before := calls
	record.Enabled = false
	out = ObserveBroadcaster(context.Background(), client, c, record)
	if calls != before || out.Status != "not_probed" {
		t.Fatal(out)
	}
}
func TestDiscoveredBroadcasterRejectsPrivateAndRedirectTargets(t *testing.T) {
	for _, endpoint := range []string{"http://public.example", "https://user:password@public.example", "https://127.0.0.1", "https://169.254.169.254/metadata", "https://[::1]", "https://10.0.0.1", "https://public.example/#fragment"} {
		if ValidateBroadcasterEndpoint(endpoint) == nil {
			t.Fatal(endpoint)
		}
	}
	for _, ip := range []string{"127.0.0.1", "::1", "::ffff:127.0.0.1", "10.0.0.1", "100.64.0.1", "198.18.0.1", "fe80::1", "169.254.169.254", "fc00::1"} {
		if publicProbeIP(net.ParseIP(ip)) {
			t.Fatal(ip)
		}
	}
	if ValidateBroadcasterEndpoint("https://public.example/v1/chains/97") != nil || !publicProbeIP(net.ParseIP("1.1.1.1")) {
		t.Fatal("public endpoint rejected")
	}
	if BroadcasterClient().CheckRedirect(&http.Request{}, nil) == nil {
		t.Fatal("redirect accepted")
	}
}
func TestSyntheticDNSFallbackNeverAllowsPrivateIP(t *testing.T) {
	resolver := func(ip string) probeResolver {
		return func(context.Context, string) ([]net.IPAddr, error) { return []net.IPAddr{{IP: net.ParseIP(ip)}}, nil }
	}
	ips, err := validatedProbeIPs(context.Background(), "public.example", resolver("198.18.0.1"), resolver("1.1.1.1"))
	if err != nil || len(ips) != 1 || !ips[0].IP.Equal(net.ParseIP("1.1.1.1")) {
		t.Fatal(ips, err)
	}
	if _, err = validatedProbeIPs(context.Background(), "public.example", resolver("198.18.0.1"), resolver("127.0.0.1")); err == nil {
		t.Fatal("private DNS fallback accepted")
	}
	called := false
	if _, err = validatedProbeIPs(context.Background(), "private.example", resolver("10.0.0.1"), func(context.Context, string) ([]net.IPAddr, error) { called = true; return nil, nil }); err == nil || called {
		t.Fatal("private endpoint escaped via public resolver")
	}
}
func TestMissingCapabilityFieldsStayUnknown(t *testing.T) {
	var value struct {
		Service struct {
			Scheduling *struct {
				Enabled *bool `json:"enabled"`
			} `json:"scheduling"`
		} `json:"service"`
	}
	// Check the real public model round-trip, not a false default from an empty object.
	var observation model.BroadcasterObservation
	if err := json.Unmarshal([]byte(`{"service":{"scheduling":{},"provenance":{},"feeLock":{"router":true}}}`), &observation); err != nil {
		t.Fatal(err)
	}
	if observation.Service.Scheduling.Enabled != nil || observation.Service.Provenance.Enabled != nil || observation.Service.FeeLock.Core != nil {
		t.Fatal(observation)
	}
	raw, _ := json.Marshal(observation)
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	if value.Service.Scheduling.Enabled != nil {
		t.Fatal("unknown became false")
	}
}

func TestPublicBroadcasterObservationDropsLegacyStatistics(t *testing.T) {
	var observation model.BroadcasterObservation
	if err := json.Unmarshal([]byte(`{"status":"available","statistics":{"submitted":12345,"knownGasCostWei":"999"},"statisticsStatus":"available"}`), &observation); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(observation)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "statistics") || strings.Contains(string(raw), "12345") {
		t.Fatalf("legacy operating statistics leaked into public DTO: %s", raw)
	}
}
