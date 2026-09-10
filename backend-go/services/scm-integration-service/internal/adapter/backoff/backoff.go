// Package backoff implements usecase.BackoffExecutor (BR-PI-03): jittered
// exponential retry around a provider call, keyed per (provider, tenant) so
// one tenant's repeated failures never throttle another tenant's calls to
// the same provider. Purely in-process — no persistence, unlike
// IssueListCache/RateLimitCache, since retry state only needs to live for
// the duration of one Do call.
package backoff

import (
	"context"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

// Executor retries fn up to maxAttempts times with jittered exponential
// backoff between attempts. A non-transient (4xx-shaped) error returns
// immediately without retrying — see isNonTransient's doc comment for the
// known limitation this heuristic accepts.
type Executor struct {
	maxAttempts int
	baseDelay   time.Duration
	maxDelay    time.Duration
}

// New constructs an Executor. maxAttempts<=0 defaults to 3 (BR-PI-03's
// "retry with backoff" — the same 3-total-attempts shape TASK-PI-03-07's
// issue-status-sync retry uses for consistency across this solution set).
func New(maxAttempts int, baseDelay, maxDelay time.Duration) *Executor {
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	if baseDelay <= 0 {
		baseDelay = 200 * time.Millisecond
	}
	if maxDelay <= 0 {
		maxDelay = 5 * time.Second
	}
	return &Executor{maxAttempts: maxAttempts, baseDelay: baseDelay, maxDelay: maxDelay}
}

var _ usecase.BackoffExecutor = (*Executor)(nil)

// Do runs fn, retrying on transient failure. provider is accepted (not just
// unused) so a future per-provider circuit breaker can key off it the same
// way — see ports.go's BackoffExecutor doc comment; this in-process
// implementation doesn't need it for the retry loop itself since each Do
// call already runs in isolation per request.
func (e *Executor) Do(ctx context.Context, provider domain.ScmProvider, fn func(context.Context) ([]domain.Issue, error)) ([]domain.Issue, error) {
	var lastErr error
	for attempt := 0; attempt < e.maxAttempts; attempt++ {
		if attempt > 0 {
			delay := e.delayFor(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
		issues, err := fn(ctx)
		if err == nil {
			return issues, nil
		}
		lastErr = err
		if isNonTransient(err) {
			return nil, err // 4xx-shaped — retrying won't help, per BR-PI-03
		}
	}
	return nil, lastErr
}

// delayFor computes attempt N's backoff: base * 2^(N-1), capped at
// maxDelay, plus up to ±25% jitter to avoid a thundering herd of retries
// all firing on the same tick.
func (e *Executor) delayFor(attempt int) time.Duration {
	d := e.baseDelay
	for i := 1; i < attempt; i++ {
		d *= 2
		if d > e.maxDelay {
			d = e.maxDelay
			break
		}
	}
	jitter := time.Duration(rand.Int64N(int64(d)/2+1)) - d/4
	d += jitter
	if d < 0 {
		d = 0
	}
	return d
}

// isNonTransient is a best-effort heuristic, not a structured status-code
// check — this codebase's provider adapters return plain fmt.Errorf-wrapped
// HTTP errors (e.g. "github: list issues: unexpected status %d") rather
// than a typed status-carrying error, so a 4xx is detected by substring.
// A false negative here just means one avoidable retry, never a correctness
// bug — the caller's own maxAttempts bound still applies either way.
func isNonTransient(err error) bool {
	msg := err.Error()
	for _, code := range []string{"status 400", "status 401", "status 403", "status 404", "status 422"} {
		if strings.Contains(msg, code) {
			return true
		}
	}
	return false
}
