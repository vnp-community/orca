// Package usecase holds scm-integration-service's application services and
// the ports they need — defined here, implemented in internal/adapter/*,
// per the Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
//
// usecase/ code never imports a provider package (github, gitlab, ...)
// directly — it depends only on the ScmProvider interface below, handed the
// right implementation by ProviderRegistry, itself wired by
// cmd/server/main.go's composition root. That's what turns a future
// multi-provider fan-out (e.g. CheckHostedReviewEligibility, not in this
// scaffold's RPC surface) into a loop over one interface instead of one
// hand-copied branch per provider — see scm-integration-service.md §6.
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// Credential is the resolved per-tenant OAuth token for one provider call.
// It is deliberately the only place a token value lives outside an
// adapter's own HTTP auth-header construction — never logged, never
// persisted, never cached beyond a single usecase Execute call, per
// scm-integration-service.md §9 ("no shared, service-wide, or process-wide
// credential — a structural guarantee, not a runtime check").
type Credential struct {
	Token string
}

// IssueFilter narrows a ListIssues call. Empty State means "all states".
type IssueFilter struct {
	State string
}

// CreatePullRequestInput is the provider-facing input for opening a new
// pull/merge request.
type CreatePullRequestInput struct {
	Title      string
	Body       string
	HeadBranch string
	BaseBranch string
}

// ScmProvider is the port each concrete provider adapter
// (internal/adapter/{github,gitlab,bitbucket,azuredevops,gitea}) implements
// — see scm-integration-service.md §4/§6. Every method takes its Credential
// as an explicit parameter (never a field on the adapter itself) so there is
// no way for one tenant's call to accidentally reuse another tenant's
// resolved token.
type ScmProvider interface {
	ListIssues(ctx context.Context, cred Credential, repo string, filter IssueFilter) ([]domain.Issue, error)
	CreatePullRequest(ctx context.Context, cred Credential, repo string, input CreatePullRequestInput) (domain.PullRequest, error)
	ListPullRequests(ctx context.Context, cred Credential, repo string) ([]domain.PullRequest, error)
	GetRateLimitStatus(ctx context.Context, cred Credential) (domain.RateLimitStatus, error)
}

// ProviderRegistry resolves which concrete ScmProvider implementation to use
// for a given provider enum value. cmd/server/main.go's composition root
// registers one entry per adapter package; usecase/ code never imports a
// provider package directly (§6).
type ProviderRegistry interface {
	Resolve(provider domain.ScmProvider) (ScmProvider, error)
}

// CredentialResolver represents the call to credential-broker-service that
// resolves this tenant's OAuth token for the given provider, before every
// provider call (§7/§9).
//
// STUB — credential-broker-service doesn't exist as a running service in
// this scaffold; internal/adapter/credentialbroker holds the stub
// implementation. Replace it with a real gRPC call to
// credential-broker-service before this service is deployed anywhere real
// tenant credentials matter — see scm-integration-service.md §7.
type CredentialResolver interface {
	Resolve(ctx context.Context, tenantID string, provider domain.ScmProvider) (Credential, error)
}
