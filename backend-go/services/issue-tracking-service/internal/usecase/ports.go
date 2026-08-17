// Package usecase holds issue-tracking-service's application services and
// the ports they need — defined here, implemented in internal/adapter/*,
// per the Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

// Credential is the per-tenant material an IssueTrackerProvider needs to
// call out to Jira or Linear on the caller's behalf — resolved fresh per
// request (design doc §8: "no per-connection session state"), never cached
// beyond the request. Not every field applies to every provider: Jira uses
// BaseURL+Email+Token (HTTPS Basic Auth, base64(email:apiToken)); Linear
// uses only Token (bearer, personal API key or OAuth access token) — see
// design doc §9.
type Credential struct {
	BaseURL string // Jira site base URL, e.g. https://your-domain.atlassian.net; unused by Linear
	Email   string // Jira account email; unused by Linear
	Token   string // Jira API token, or Linear personal API key / OAuth access token
}

// IssueTrackerProvider is the port each provider adapter
// (internal/adapter/jira, internal/adapter/linear) implements against its
// own wire protocol (Jira REST, Linear GraphQL) — usecases depend only on
// this port, never on a provider-specific client type, per design doc §4.
type IssueTrackerProvider interface {
	ListIssues(ctx context.Context, cred Credential, projectKey string) ([]domain.Issue, error)
	CreateIssue(ctx context.Context, cred Credential, projectKey, title, description string) (domain.Issue, error)
}

// ProviderRegistry resolves the IssueTrackerProvider implementation for a
// given domain.Provider — keeps usecases provider-agnostic instead of
// switching on domain.Provider themselves (design doc §4).
type ProviderRegistry interface {
	Resolve(provider domain.Provider) (IssueTrackerProvider, error)
}

// CredentialResolver resolves the per-tenant Jira/Linear credential before
// every provider call (design doc §7, §9).
//
// STUB PORT: the real implementation calls credential-broker-service's
// ResolveCredential RPC against Vault KV v2, one path per
// (tenant, service, user) — see
// specs/backend-go/architecture/06-secrets-vault-architecture.md's
// "Integration OAuth tokens" row, which names Jira/Linear explicitly. This
// scaffold's concrete implementation (internal/adapter/credential) is a
// local-dev stub reading environment variables and MUST be replaced with a
// real credential-broker-service client before this service is deployed
// anywhere real tenant secrets exist. Same pattern scm-integration-service
// uses for its own credential resolution.
type CredentialResolver interface {
	Resolve(ctx context.Context, tenantID string, provider domain.Provider) (Credential, error)
}

// OutboxEnqueuer is the outbound event port for issue linking —
// issue-tracking-service durably records orca.issuetracking.link.created
// so task-service/project-service can update their own records of which
// task/worktree references which external issue (design doc §7). Backed by
// this service's own Postgres database (internal/adapter/postgres) as of
// Epic G (docs/execution-plan.md) — this service previously owned no
// database at all, so unlike a normal transactional-outbox write (a
// domain-state INSERT plus an outbox-row INSERT in one transaction), the
// enqueued row here IS the entire write: there is no other domain state in
// this service to be atomic with. What changed from the pre-Epic-G direct
// NATS publish: the event now has a durable Postgres row proving intent
// the instant Enqueue returns, closing the old "publish IS the persisted
// side effect, so publish failure must fail the RPC with nothing durable
// to retry against" gap — a transient NATS outage no longer needs the
// caller to retry LinkIssue itself, only the outbox relay retries.
type OutboxEnqueuer interface {
	Enqueue(ctx context.Context, tenantID string, event domain.OutboxEvent) error
}
