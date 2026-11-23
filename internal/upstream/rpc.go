// Package upstream implements bounded read-only JSON-RPC clients.
package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const MaxResponseBytes = 8 << 20

// RPCError retains only numeric classification from an envelope bound to the
// exact request ID. Untrusted upstream messages and error data are discarded.
type RPCError struct {
	Code       int
	HTTPStatus int
}

func (e *RPCError) Error() string { return "upstream RPC rejected" }

func Client() *http.Client {
	return &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("upstream redirect rejected") }}
}
func Call(ctx context.Context, c *http.Client, url, method string, params any, out any) error {
	if url == "" {
		return errors.New("upstream not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	b, e := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if e != nil {
		return e
	}
	req, e := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/json")
	r, e := c.Do(req)
	if e != nil {
		return errors.New("upstream unavailable")
	}
	defer r.Body.Close()
	b, e = io.ReadAll(io.LimitReader(r.Body, MaxResponseBytes+1))
	if e != nil || len(b) > MaxResponseBytes {
		return errors.New("upstream response bound")
	}
	var env struct {
		Version string          `json:"jsonrpc"`
		ID      int             `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
	}
	if json.Unmarshal(b, &env) != nil || env.Version != "2.0" || env.ID != 1 {
		return errors.New("upstream RPC rejected")
	}
	if len(env.Error) > 0 {
		var classification struct {
			Code *int `json:"code"`
		}
		if len(env.Result) > 0 || json.Unmarshal(env.Error, &classification) != nil || classification.Code == nil {
			return errors.New("upstream RPC rejected")
		}
		return &RPCError{Code: *classification.Code, HTTPStatus: r.StatusCode}
	}
	if r.StatusCode != http.StatusOK {
		return errors.New("upstream HTTP error")
	}
	if len(env.Result) == 0 || bytes.Equal(env.Result, []byte("null")) {
		return errors.New("upstream RPC rejected")
	}
	if e = json.Unmarshal(env.Result, out); e != nil {
		return fmt.Errorf("invalid upstream result: %w", e)
	}
	return nil
}
