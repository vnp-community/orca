# TASK-PI-01-03: Fix `ListIssues.Execute` to pass the real filter, cache, and retry with backoff

**From Solution:** SOL-PI-01
**Priority:** P0 — this is the actual bug BUG-PI-01 reports
**Service:** `scm-integration-service`
**File:** `backend-go/services/scm-integration-service/internal/usecase/list_issues.go`
**Depends on:** TASK-PI-01-02
**Status:** `[ ]` TODO

---

## Context

`ListIssues.Execute` currently calls
`provider.ListIssues(ctx, cred, in.Repo, IssueFilter{})` with a hardcoded
empty filter (`list_issues.go:53`) even though the port already takes a
`filter IssueFilter` parameter — every one of BL-PI-01's step-5 filters is
unreachable end-to-end. This task wires `in.Filter` through, and adds the
5-minute cache (BR-PI-01) and backoff retry (BR-PI-03) around the same call.

## Changes to make

```go
// ListIssuesInput mirrors the ListIssuesRequest RPC 1:1.
type ListIssuesInput struct {
	TenantID     string
	Provider     domain.ScmProvider
	Repo         string
	Filter       IssueFilter
	ForceRefresh bool
}

type ListIssues struct {
	credentials CredentialResolver
	providers   ProviderRegistry
	cache       IssueListCache
	backoff     BackoffExecutor
}

func NewListIssues(credentials CredentialResolver, providers ProviderRegistry, cache IssueListCache, backoff BackoffExecutor) *ListIssues {
	return &ListIssues{credentials: credentials, providers: providers, cache: cache, backoff: backoff}
}

func (uc *ListIssues) Execute(ctx context.Context, in ListIssuesInput) (ListIssuesOutput, error) {
	if in.TenantID == "" {
		return ListIssuesOutput{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Repo == "" {
		return ListIssuesOutput{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_EMPTY_REPO", "repo is required", nil)
	}

	key := IssueCacheKey{TenantID: in.TenantID, Provider: in.Provider, Repo: in.Repo, Filter: in.Filter}
	if !in.ForceRefresh && uc.cache != nil {
		if cached, ok, err := uc.cache.Get(ctx, key); err == nil && ok {
			return ListIssuesOutput{Issues: cached.Issues, FromCache: true, CachedAt: cached.CachedAt}, nil
		}
		// Cache-read errors are logged upstream (grpc handler), never fail
		// the request — the cache is an optimization, not a source of truth.
	}

	cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.Provider)
	if err != nil {
		return ListIssuesOutput{}, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_RESOLVE_FAILED", "failed to resolve provider credential", err)
	}
	provider, err := uc.providers.Resolve(in.Provider)
	if err != nil {
		return ListIssuesOutput{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no adapter registered for this provider", err)
	}

	fetch := func(ctx context.Context) ([]domain.Issue, error) {
		return provider.ListIssues(ctx, cred, in.Repo, in.Filter) // in.Filter, not IssueFilter{} — the fix
	}
	var issues []domain.Issue
	if uc.backoff != nil {
		issues, err = uc.backoff.Do(ctx, in.Provider, fetch)
	} else {
		issues, err = fetch(ctx)
	}
	if err != nil {
		return ListIssuesOutput{}, apperrors.New(apperrors.KindInternal, "SCM_LIST_ISSUES_FAILED", "failed to list issues", err)
	}

	now := time.Now().UTC()
	if uc.cache != nil {
		_ = uc.cache.Put(ctx, key, issues, now, 5*time.Minute) // log-and-ignore, same non-blocking posture as the read
	}
	return ListIssuesOutput{Issues: issues, FromCache: false, CachedAt: now}, nil
}

// ListIssuesOutput replaces the old bare []domain.Issue return so
// FromCache/CachedAt can be surfaced to the gRPC layer.
type ListIssuesOutput struct {
	Issues    []domain.Issue
	FromCache bool
	CachedAt  time.Time
}
```

Update the gRPC server handler (`internal/adapter/grpc/server.go`'s
`ListIssues` method) to pass `req.GetFilter()`/`req.GetForceRefresh()` into
`ListIssuesInput` and map `ListIssuesOutput.FromCache`/`CachedAt` onto the
new `ListIssuesResponse` fields — follow the existing handler's shape for
every other RPC in that file.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/scm-integration-service/...
go vet ./services/scm-integration-service/...
```
