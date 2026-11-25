package upstream

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"
)

// All Provenance methods and chain paths share the provider's per-client quota.
// 1.5 reads/second with a burst of 10 permits at most 100 reads in any minute,
// below the provider's default 120. Background hydration leaves two tokens for
// status + roots; it skips a pass instead of blocking synchronization.
const provenanceReadRate = 1.5
const provenanceReadBurst = 10.0

type provenanceAllowance struct {
	tokens                float64
	updated, blockedUntil time.Time
}
type provenanceQuota struct {
	mu      sync.Mutex
	origins map[string]*provenanceAllowance
	now     func() time.Time
}

func (q *provenanceQuota) clock() time.Time {
	if q.now != nil {
		return q.now()
	}
	return time.Now()
}
func provenanceOrigin(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return endpoint
	}
	scheme, host := strings.ToLower(u.Scheme), strings.ToLower(u.Hostname())
	port := u.Port()
	if port == "" {
		if scheme == "https" {
			port = "443"
		} else if scheme == "http" {
			port = "80"
		}
	}
	return scheme + "://" + host + ":" + port
}

// acquire returns zero only when one request has been charged to the budget.
func (q *provenanceQuota) acquire(endpoint string, background bool, now time.Time) time.Duration {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.origins == nil {
		q.origins = map[string]*provenanceAllowance{}
	}
	key := provenanceOrigin(endpoint)
	a := q.origins[key]
	if a == nil {
		a = &provenanceAllowance{tokens: provenanceReadBurst, updated: now}
		q.origins[key] = a
	}
	if now.After(a.updated) {
		a.tokens = min(provenanceReadBurst, a.tokens+now.Sub(a.updated).Seconds()*provenanceReadRate)
		a.updated = now
	}
	if now.Before(a.blockedUntil) {
		return a.blockedUntil.Sub(now)
	}
	required := 1.0
	if background {
		required = 3
	}
	if a.tokens >= required {
		a.tokens--
		return 0
	}
	return time.Duration((required-a.tokens)/provenanceReadRate*float64(time.Second)) + time.Nanosecond
}
func (q *provenanceQuota) block(endpoint string, until time.Time) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.origins == nil {
		q.origins = map[string]*provenanceAllowance{}
	}
	key := provenanceOrigin(endpoint)
	a := q.origins[key]
	if a == nil {
		a = &provenanceAllowance{updated: q.clock()}
		q.origins[key] = a
	}
	if until.After(a.blockedUntil) {
		a.blockedUntil = until
	}
}
func (s *Syncer) callProvenance(ctx context.Context, endpoint, method string, params, out any) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		delay := s.provenanceReads.acquire(endpoint, false, s.provenanceReads.clock())
		if delay == 0 {
			break
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return s.provenanceRPC(ctx, endpoint, method, params, out)
}

// provenanceRPC is called only after admission, including background hydration.
func (s *Syncer) provenanceRPC(ctx context.Context, endpoint, method string, params, out any) error {
	err := Call(ctx, s.Client, endpoint, method, params, out)
	var rpc *RPCError
	if errors.As(err, &rpc) && (rpc.HTTPStatus == 429 || rpc.Code == -32013) {
		seconds := rpc.RetryAfter
		if seconds <= 0 {
			seconds = 60
		}
		s.provenanceReads.block(endpoint, s.provenanceReads.clock().Add(time.Duration(seconds)*time.Second))
	}
	return err
}
