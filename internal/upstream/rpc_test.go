package upstream

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRPCRejectsMismatchedEnvelopeAndOversize(t *testing.T) {
	for _, body := range []string{`{"jsonrpc":"2.0","id":2,"result":{}}`, `{"jsonrpc":"2.0","id":1,"result":{},"error":{"code":1}}`, `{"jsonrpc":"2.0","id":1,"result":null}`} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
		var out map[string]any
		if Call(context.Background(), s.Client(), s.URL, "test", []any{}, &out) == nil {
			t.Errorf("accepted %s", body)
		}
		s.Close()
	}
}

func TestRPCPreservesOnlyBoundNumericBusinessError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32021,"message":"PRIVATE provider diagnostics","data":{"secret":"PRIVATE"}}}`))
	}))
	defer server.Close()
	var out any
	err := Call(context.Background(), server.Client(), server.URL, "test", []any{}, &out)
	var typed *RPCError
	if !errors.As(err, &typed) || typed.Code != -32021 || typed.HTTPStatus != 503 {
		t.Fatalf("lost bounded error classification: %v", err)
	}
	if strings.Contains(err.Error(), "PRIVATE") {
		t.Fatalf("exposed upstream message %v", err)
	}
}

func TestRPCBoundedRateLimitDelay(t *testing.T) {
	for _, tc := range []struct {
		header string
		want   int
	}{{"7", 7}, {"", 60}, {"0", 60}, {"-1", 60}, {"3601", 60}, {"PRIVATE", 60}} {
		t.Run(tc.header, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Retry-After", tc.header)
				w.WriteHeader(429)
				w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32013,"message":"PRIVATE"}}`))
			}))
			defer server.Close()
			var out any
			err := Call(context.Background(), server.Client(), server.URL, "test", []any{}, &out)
			var rpc *RPCError
			if !errors.As(err, &rpc) || rpc.RetryAfter != tc.want {
				t.Fatalf("unbounded retry delay: %+v", rpc)
			}
		})
	}
}
