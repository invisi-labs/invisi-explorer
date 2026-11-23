package upstream

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/invisi-labs/invisi-explorer/internal/config"
)

func TestBroadcasterProbeUsesPublicMetadataOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request.Method != "broadcaster_getStatus" || string(request.Params) != "[]" {
			t.Errorf("unexpected request: %+v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"apiVersion":"v1","protocolVersion":"1","chainId":"97","protocol":"0x1111111111111111111111111111111111111111","runtimeCodeHash":"0x2222222222222222222222222222222222222222222222222222222222222222","sender":"0x3333333333333333333333333333333333333333","feeRecipient":"0x4444444444444444444444444444444444444444","submissionHalted":true,"trackingId":"private-tracking","journal":{"secret":"private-journal"}}}`))
	}))
	defer server.Close()
	chain := config.Chain{ChainID: "97", ProtocolAddress: "0x1111111111111111111111111111111111111111", CodeHash: "0x2222222222222222222222222222222222222222222222222222222222222222", BroadcasterURL: server.URL}
	result, err := ProbeBroadcaster(context.Background(), server.Client(), chain)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "halted" || !result.SubmissionHalted {
		t.Fatalf("wrong status: %+v", result)
	}
	raw, _ := json.Marshal(result)
	if strings.Contains(string(raw), "private-") || strings.Contains(string(raw), "trackingId") {
		t.Fatal("private metadata leaked")
	}
	chain.ProtocolAddress = "0x5555555555555555555555555555555555555555"
	if _, err := ProbeBroadcaster(context.Background(), server.Client(), chain); err == nil {
		t.Fatal("accepted wrong deployment")
	}
	chain.ProtocolAddress = "0x1111111111111111111111111111111111111111"
	chain.CodeHash = "0x6666666666666666666666666666666666666666666666666666666666666666"
	if _, err := ProbeBroadcaster(context.Background(), server.Client(), chain); err == nil {
		t.Fatal("accepted wrong code hash")
	}
}

func TestBroadcasterProbeOptionalAndUnavailable(t *testing.T) {
	result, err := ProbeBroadcaster(context.Background(), Client(), config.Chain{})
	if err != nil || result.Status != "not_configured" {
		t.Fatalf("optional dependency: %+v %v", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ProbeBroadcaster(ctx, Client(), config.Chain{BroadcasterURL: "http://127.0.0.1:1"}); err == nil {
		t.Fatal("accepted unavailable broadcaster")
	}
}
