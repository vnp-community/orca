package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// ListIssuesInput mirrors the ListIssuesRequest RPC 1:1 — see
// architecture/03's note that usecase granularity mirrors the RPC surface.
// TenantID arrives as an explicit field (not pulled from grpc context via
// common/tenant, unlike usage-service) because scm-integration-service.md §3
// is explicit that every request already carries a tenant_id resolved
// upstream by api-gateway, as a proto field.
type ListIssuesInput struct {
	TenantID     string
	Provider     domain.ScmProvider
	Repo         string
	Filter       IssueFilter
	ForceRefresh bool
}

// ListIssuesOutput replaces the old bare []domain.Issue return so
// FromCache/CachedAt can be surfaced to the gRPC layer (BUG-PI-01).
type ListIssuesOutput struct {
	Issues    []domain.Issue
	FromCache bool
	CachedAt  time.Time
}

// ListIssues resolves this tenant's per-provider credential (stubbed, see
// CredentialResolver), resolves the concrete provider adapter from the
// registry, and delegates — the usecase layer's whole job here is correct
// dispatch, not the HTTP call itself. BUG-PI-01's fix: in.Filter is now
// actually passed through to the provider call (was previously a hardcoded
// IssueFilter{}), wrapped by a 5-minute cache (BR-PI-01) and jittered
// backoff retry (BR-PI-03).
type ListIssues struct {
	credentials CredentialResolver
	providers   ProviderRegistry
	cache       IssueListCache
	backoff     BackoffExecutor
}

// NewListIssues — cache/backoff are optional (nil-safe): a nil cache skips
// the read-through entirely, a nil backoff calls the provider directly.
// Lets tests and any composition root that hasn't wired the new Postgres
// cache/backoff adapters yet keep building against the old 2-arg shape's
// behavior.
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
		return provider.ListIssues(ctx, cred, in.Repo, in.Filter) // in.Filter, not IssueFilter{} — the BUG-PI-01 fix
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
