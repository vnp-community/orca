# TASK-PI-01-02: Extend `IssueFilter` + add `IssueListCache`/`BackoffExecutor` ports

**From Solution:** SOL-PI-01
**Priority:** P0
**Service:** `scm-integration-service`
**File:** `backend-go/services/scm-integration-service/internal/usecase/ports.go`
**Depends on:** TASK-PI-01-01
**Status:** `[x] DONE — IssueFilter extended, IssueListCache/BackoffExecutor ports added to ports.go.`

---

## Context

`ports.go`'s `IssueFilter` currently has only `State` (BUG-PI-01's cited
`ports.go:33-35`); `Assignee`/`Labels`/`Milestone` were never added because
nothing ever populated them. This task extends the type and adds the two new
ports `list_issues.go` (TASK-PI-01-03) needs: a 5-minute cache (sibling of
`RateLimitCache`) and a jittered-exponential backoff wrapper.

## Changes to make

Replace the existing `IssueFilter` type and add below it:

```go
// IssueFilter narrows a ListIssues call. All-empty fields mean "no filter".
type IssueFilter struct {
	State     string
	Assignee  string
	Labels    []string
	Milestone string
}

// CachedIssueList is what IssueListCache.Get returns on a hit.
type CachedIssueList struct {
	Issues   []domain.Issue
	CachedAt time.Time
}

// IssueCacheKey identifies one cached ListIssues call by request shape —
// distinct filter combos for the same tenant/provider/repo never collide.
type IssueCacheKey struct {
	TenantID string
	Provider domain.ScmProvider
	Repo     string
	Filter   IssueFilter
}

// IssueListCache is a 5-minute-TTL hot read in front of ListIssues' live
// provider call (BR-PI-01) — sibling of RateLimitCache
// (adapter/postgres/rate_limit_cache.go), not a source of truth. Cache
// read/write errors are logged by the caller and never fail Execute.
type IssueListCache interface {
	Get(ctx context.Context, key IssueCacheKey) (CachedIssueList, bool, error)
	Put(ctx context.Context, key IssueCacheKey, issues []domain.Issue, cachedAt time.Time, ttl time.Duration) error
}

// BackoffExecutor wraps a provider call with jittered exponential retry
// (BR-PI-03), keyed per (provider, tenant) — same key shape as §8's
// circuit breaker so both mechanisms trip independently per provider, never
// globally. A non-transient (4xx) error is not retried.
type BackoffExecutor interface {
	Do(ctx context.Context, provider domain.ScmProvider, fn func(context.Context) ([]domain.Issue, error)) ([]domain.Issue, error)
}
```

Add `"time"` to the file's import block if not already present.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/scm-integration-service/...
go vet ./services/scm-integration-service/...
```
