package usecase

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// fakeIssueListCache is an in-memory IssueListCache — stands in for the
// real postgres-backed one in internal/adapter/postgres.
type fakeIssueListCache struct {
	entry  CachedIssueList
	hit    bool
	getErr error
	putErr error

	getCalls   int
	putCalls   int
	lastPutKey IssueCacheKey
}

func (f *fakeIssueListCache) Get(_ context.Context, key IssueCacheKey) (CachedIssueList, bool, error) {
	f.getCalls++
	if f.getErr != nil {
		return CachedIssueList{}, false, f.getErr
	}
	return f.entry, f.hit, nil
}

func (f *fakeIssueListCache) Put(_ context.Context, key IssueCacheKey, issues []domain.Issue, cachedAt time.Time, ttl time.Duration) error {
	f.putCalls++
	f.lastPutKey = key
	return f.putErr
}

// fakeBackoffExecutor is a small, self-contained BackoffExecutor test
// double — deliberately NOT the real internal/adapter/backoff.Executor:
// importing that package from this internal usecase test would create an
// import cycle (adapter/backoff imports usecase for the interface it
// implements). Same retry-then-give-up/non-transient-short-circuits
// contract as BR-PI-03, just local to this test file.
type fakeBackoffExecutor struct {
	maxAttempts int
	doCalls     int
}

func (f *fakeBackoffExecutor) Do(ctx context.Context, provider domain.ScmProvider, fn func(context.Context) ([]domain.Issue, error)) ([]domain.Issue, error) {
	f.doCalls++
	max := f.maxAttempts
	if max <= 0 {
		max = 3
	}
	var lastErr error
	for i := 0; i < max; i++ {
		issues, err := fn(ctx)
		if err == nil {
			return issues, nil
		}
		lastErr = err
		if strings.Contains(err.Error(), "status 404") { // 4xx-shaped, non-transient
			return nil, err
		}
	}
	return nil, lastErr
}

// flakyIssuesProvider wraps fakeProvider, overriding ListIssues to fail the
// first `failures` calls before succeeding (or always failing when
// failures is large) — lets backoff-retry tests exercise a provider whose
// behavior changes across attempts, which the static fakeProvider can't do.
type flakyIssuesProvider struct {
	*fakeProvider
	failures  int
	failMsg   string
	callCount int
}

func (p *flakyIssuesProvider) ListIssues(ctx context.Context, cred Credential, repo string, filter IssueFilter) ([]domain.Issue, error) {
	p.callCount++
	p.lastFilter = filter
	if p.callCount <= p.failures {
		msg := p.failMsg
		if msg == "" {
			msg = "github: list issues: unexpected status 502"
		}
		return nil, errors.New(msg)
	}
	return p.fakeProvider.issues, nil
}

// TestListIssues_FilterReachesProvider_RegressionGuard is BUG-PI-01's exact
// regression guard: the fake ScmProvider must be called with the caller's
// real IssueFilter, not a hardcoded IssueFilter{}.
func TestListIssues_FilterReachesProvider_RegressionGuard(t *testing.T) {
	github := &fakeProvider{}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: github}}
	uc := NewListIssues(&fakeCredentialResolver{token: "tok"}, registry, nil, nil)

	filter := IssueFilter{State: "open", Assignee: "octocat", Labels: []string{"bug", "p0"}, Milestone: "v1"}
	_, err := uc.Execute(context.Background(), ListIssuesInput{
		TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "o/r", Filter: filter,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(github.lastFilter, filter) {
		t.Fatalf("expected provider to receive the exact filter %+v, got %+v (regression: was hardcoded to IssueFilter{})", filter, github.lastFilter)
	}
}

func TestListIssues_CacheHit_SkipsProviderCall(t *testing.T) {
	github := &fakeProvider{issues: []domain.Issue{{ID: "should-not-be-returned"}}}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: github}}
	cached := []domain.Issue{{ID: "cached-1"}}
	cache := &fakeIssueListCache{hit: true, entry: CachedIssueList{Issues: cached, CachedAt: time.Now().Add(-1 * time.Minute)}}
	uc := NewListIssues(&fakeCredentialResolver{token: "tok"}, registry, cache, nil)

	out, err := uc.Execute(context.Background(), ListIssuesInput{TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "o/r"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.FromCache {
		t.Error("expected FromCache=true on a cache hit")
	}
	if len(out.Issues) != 1 || out.Issues[0].ID != "cached-1" {
		t.Fatalf("expected the cached issues back, got %+v", out.Issues)
	}
	if github.calls != 0 {
		t.Fatalf("expected a cache hit to short-circuit the provider call entirely, got %d calls", github.calls)
	}
}

func TestListIssues_CacheMissCallsProviderAndPopulatesCache(t *testing.T) {
	live := []domain.Issue{{ID: "live-1"}}
	github := &fakeProvider{issues: live}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: github}}
	cache := &fakeIssueListCache{hit: false}
	uc := NewListIssues(&fakeCredentialResolver{token: "tok"}, registry, cache, nil)

	out, err := uc.Execute(context.Background(), ListIssuesInput{TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "o/r"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.FromCache {
		t.Error("expected FromCache=false on a cache miss")
	}
	if github.calls != 1 {
		t.Fatalf("expected exactly one provider call on a cache miss, got %d", github.calls)
	}
	if cache.putCalls != 1 {
		t.Fatalf("expected the live result to be written back to the cache, putCalls=%d", cache.putCalls)
	}
}

func TestListIssues_ForceRefreshBypassesCache(t *testing.T) {
	github := &fakeProvider{issues: []domain.Issue{{ID: "live-1"}}}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: github}}
	// A fresh cache hit that would normally short-circuit — force_refresh
	// must bypass it regardless of freshness.
	cache := &fakeIssueListCache{hit: true, entry: CachedIssueList{Issues: []domain.Issue{{ID: "stale-cached"}}, CachedAt: time.Now()}}
	uc := NewListIssues(&fakeCredentialResolver{token: "tok"}, registry, cache, nil)

	out, err := uc.Execute(context.Background(), ListIssuesInput{TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "o/r", ForceRefresh: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cache.getCalls != 0 {
		t.Fatalf("expected force_refresh to skip the cache read entirely, getCalls=%d", cache.getCalls)
	}
	if github.calls != 1 {
		t.Fatalf("expected force_refresh to call the provider, got %d calls", github.calls)
	}
	if out.FromCache {
		t.Error("expected FromCache=false when force_refresh bypassed the cache")
	}
}

func TestListIssues_CacheReadErrorDoesNotFailExecute(t *testing.T) {
	github := &fakeProvider{issues: []domain.Issue{{ID: "live-1"}}}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: github}}
	cache := &fakeIssueListCache{getErr: errors.New("cache unreachable")}
	uc := NewListIssues(&fakeCredentialResolver{token: "tok"}, registry, cache, nil)

	out, err := uc.Execute(context.Background(), ListIssuesInput{TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "o/r"})
	if err != nil {
		t.Fatalf("expected a cache read error to be swallowed, not fail Execute: %v", err)
	}
	if github.calls != 1 {
		t.Fatalf("expected the provider to still be called after a cache read error, got %d calls", github.calls)
	}
	if len(out.Issues) != 1 || out.Issues[0].ID != "live-1" {
		t.Fatalf("expected a live result despite the cache error, got %+v", out.Issues)
	}
}

func TestListIssues_CacheWriteErrorDoesNotFailExecute(t *testing.T) {
	github := &fakeProvider{issues: []domain.Issue{{ID: "live-1"}}}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: github}}
	cache := &fakeIssueListCache{putErr: errors.New("cache unreachable")}
	uc := NewListIssues(&fakeCredentialResolver{token: "tok"}, registry, cache, nil)

	out, err := uc.Execute(context.Background(), ListIssuesInput{TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "o/r"})
	if err != nil {
		t.Fatalf("expected a cache write error to be swallowed, not fail Execute: %v", err)
	}
	if len(out.Issues) != 1 || out.Issues[0].ID != "live-1" {
		t.Fatalf("expected a result despite the cache write error, got %+v", out.Issues)
	}
}

func TestListIssues_BackoffRetriesTransientFailureUntilSuccess(t *testing.T) {
	flaky := &flakyIssuesProvider{fakeProvider: &fakeProvider{issues: []domain.Issue{{ID: "eventually-ok"}}}, failures: 2}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: flaky}}
	backoff := &fakeBackoffExecutor{maxAttempts: 3}
	uc := NewListIssues(&fakeCredentialResolver{token: "tok"}, registry, nil, backoff)

	out, err := uc.Execute(context.Background(), ListIssuesInput{TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "o/r"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if backoff.doCalls != 1 {
		t.Fatalf("expected ListIssues to call BackoffExecutor.Do exactly once, got %d", backoff.doCalls)
	}
	if flaky.callCount != 3 {
		t.Fatalf("expected the provider to be retried until success (2 failures + 1 success = 3 calls), got %d", flaky.callCount)
	}
	if len(out.Issues) != 1 || out.Issues[0].ID != "eventually-ok" {
		t.Fatalf("expected the eventual success result, got %+v", out.Issues)
	}
}

func TestListIssues_NonTransientErrorIsNotRetried(t *testing.T) {
	flaky := &flakyIssuesProvider{fakeProvider: &fakeProvider{}, failures: 100, failMsg: "github: list issues: unexpected status 404"}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: flaky}}
	backoff := &fakeBackoffExecutor{maxAttempts: 3}
	uc := NewListIssues(&fakeCredentialResolver{token: "tok"}, registry, nil, backoff)

	_, err := uc.Execute(context.Background(), ListIssuesInput{TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "o/r"})
	if err == nil {
		t.Fatal("expected the non-transient error to propagate")
	}
	if flaky.callCount != 1 {
		t.Fatalf("expected exactly 1 provider call for a non-transient (4xx-mapped) error, got %d", flaky.callCount)
	}
}
