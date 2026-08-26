// Package usecase holds issue-tracking-service's application services and
// the ports they need — defined here, implemented in internal/adapter/*,
// per the Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
package usecase

import (
	"context"
	"errors"

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
	Whoami(ctx context.Context, cred Credential) (domain.Viewer, error)

	SearchIssues(ctx context.Context, cred Credential, query string, limit int) ([]domain.Issue, error)
	ListIssues(ctx context.Context, cred Credential, projectKey, filterJSON string, limit int) ([]domain.Issue, error)
	GetIssue(ctx context.Context, cred Credential, issueID string) (domain.Issue, error)
	CreateIssue(ctx context.Context, cred Credential, in domain.NewIssueInput) (domain.Issue, error)
	UpdateIssue(ctx context.Context, cred Credential, in domain.IssueUpdate) (domain.Issue, error)
	AddIssueComment(ctx context.Context, cred Credential, issueID, bodyMarkdown string) (domain.IssueComment, error)
	ListIssueComments(ctx context.Context, cred Credential, issueID string) ([]domain.IssueComment, error)

	ListProjects(ctx context.Context, cred Credential, workspaceID string) ([]domain.ProjectRef, error)
	ListIssueTypes(ctx context.Context, cred Credential, projectIDOrKey string) ([]domain.IssueTypeRef, error)
	ListCreateFields(ctx context.Context, cred Credential, projectIDOrKey, issueTypeID string) ([]domain.CreateField, error)
	ListAssignableUsers(ctx context.Context, cred Credential, projectIDOrKey, issueID string) ([]domain.UserRef, error)
	ListPriorities(ctx context.Context, cred Credential) ([]domain.PriorityRef, error)
	ListTransitions(ctx context.Context, cred Credential, issueID string) ([]domain.Transition, error)
	GetProjectStatusOrder(ctx context.Context, cred Credential, projectIDOrKey string) (domain.ProjectStatusOrder, error)

	// CreateProject/GetProject: a shared concept (both providers' "project"
	// is a bounded set of issues with a name/lead/status) — wired only by
	// linear.* today (SOL-016's mapping table has no jira.createProject
	// channel); jira/client.go implements these as clear unsupported
	// errors rather than a silent no-op.
	CreateProject(ctx context.Context, cred Credential, workspaceID, teamID, name, description string) (domain.ProjectRef, error)
	GetProject(ctx context.Context, cred Credential, projectID, workspaceID string) (domain.ProjectRef, error)

	// ListTeams/ListTeamLabels/ListTeamMembers/GetCustomView/
	// ListWorkflowStates are Linear-only concepts (SOL-016's "genuinely
	// diverges" table) — jira/client.go implements these as clear
	// unsupported errors to satisfy the interface.
	ListTeams(ctx context.Context, cred Credential, workspaceID string) ([]domain.Team, error)
	ListTeamLabels(ctx context.Context, cred Credential, teamID string) ([]domain.TeamLabel, error)
	ListTeamMembers(ctx context.Context, cred Credential, teamID string) ([]domain.TeamMember, error)
	GetCustomView(ctx context.Context, cred Credential, viewID, model string) (domain.CustomView, error)
	ListWorkflowStates(ctx context.Context, cred Credential, teamID string) ([]domain.WorkflowState, error)
}

// ProviderRegistry resolves the IssueTrackerProvider implementation for a
// given domain.Provider — keeps usecases provider-agnostic instead of
// switching on domain.Provider themselves (design doc §4).
type ProviderRegistry interface {
	Resolve(provider domain.Provider) (IssueTrackerProvider, error)
}

// CredentialResolver resolves the per-(tenant,user,provider,workspace)
// credential before a provider call, and writes new credential material
// during Connect. Resolve is keyed by the same 4-tuple ConnectionRepository
// uses, not by an opaque credential_id directly — the usecase layer never
// needs to know a credential_id exists; the adapter looks it up via
// ConnectionRepository internally.
type CredentialResolver interface {
	// Resolve looks up the connection row for
	// (tenantID, userID, provider, workspaceID) and calls
	// credential-broker-service's ResolveCredential(credential_id) — the
	// per-request read path. workspaceID may be "" to mean "the currently
	// selected workspace."
	Resolve(ctx context.Context, tenantID, userID string, provider domain.Provider, workspaceID string) (Credential, error)

	// Write encrypts and persists cred under a composite owner_id
	// ("<userID>:<provider>") via credential-broker-service.WriteCredential,
	// returning the opaque credential_id the caller must store on the
	// connection row it creates/updates.
	Write(ctx context.Context, tenantID, userID string, provider domain.Provider, cred Credential) (credentialID string, err error)

	// ExistingCredentialID checks whether a credential already exists for
	// this composite owner_id via ResolveCredentialByOwner — used only by
	// Connect's create-new-vs-already-connected bootstrap decision, never
	// the per-request read path.
	ExistingCredentialID(ctx context.Context, tenantID, userID string, provider domain.Provider) (credentialID string, found bool, err error)
}

// ConnectionRepository is the persistence port for issuetracking_connections
// — one row per connected (tenant, user, provider, workspace).
type ConnectionRepository interface {
	// Upsert inserts or updates the row for
	// (tenantID, userID, provider, workspace.ID). A first connect for a
	// (tenant,user,provider) also sets is_selected=true; connecting an
	// additional workspace under an already-connected provider defaults
	// is_selected=false (SelectWorkspace changes it explicitly).
	Upsert(ctx context.Context, tenantID, userID string, provider domain.Provider, workspace domain.Workspace, viewer domain.Viewer, credentialID string) (domain.ConnectionStatus, error)

	// Delete removes the row for workspaceID, or every row for
	// (tenantID, userID, provider) when workspaceID is "".
	Delete(ctx context.Context, tenantID, userID string, provider domain.Provider, workspaceID string) error

	// GetStatus returns every connected workspace for
	// (tenantID, userID, provider) plus which one is_selected — the
	// GetConnectionStatus/Connect/SelectWorkspace return shape.
	GetStatus(ctx context.Context, tenantID, userID string, provider domain.Provider) (domain.ConnectionStatus, error)

	// SelectWorkspace flips is_selected to workspaceID's row (or, when
	// workspaceID == "all", clears any single selection — JiraSiteSelection
	// allows "all" as a valid value) and returns the updated status.
	SelectWorkspace(ctx context.Context, tenantID, userID string, provider domain.Provider, workspaceID string) (domain.ConnectionStatus, error)

	// GetCredentialID resolves which credential_id backs
	// (tenantID, userID, provider, workspaceID) — workspaceID == "" means
	// "the selected workspace." Returns ErrConnectionNotFound if none.
	GetCredentialID(ctx context.Context, tenantID, userID string, provider domain.Provider, workspaceID string) (credentialID string, err error)
}

// ErrConnectionNotFound is returned by ConnectionRepository methods (and
// surfaced as ISSUETRACKING_NOT_CONNECTED at the usecase layer) when no
// connection row exists for the requested key.
var ErrConnectionNotFound = errors.New("usecase: no issue-tracking connection found")

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
// CredentialWriter is this service's write path into
// credential-broker-service, backing SetIntegrationCredential -- mirrors
// scm-integration-service's port of the same name/shape.
type CredentialWriter interface {
	// WriteRaw writes a manually pasted token (+ optional non-secret
	// config) as this tenant's credential for provider. See this file's
	// package doc comment (TASK-041) for the JSON envelope shape this must
	// produce to match Resolve's existing decode convention.
	WriteRaw(ctx context.Context, tenantID string, provider domain.Provider, token, configJSON string) error
}

// CredentialStatusReader backs GetIntegrationCredentialStatus -- metadata
// only, via credential-broker-service's GetCredentialMetadataByOwner RPC
// (TASK-038), never ResolveCredentialByOwner (which would leak plaintext
// for a status check).
type CredentialStatusReader interface {
	GetStatus(ctx context.Context, tenantID string, provider domain.Provider) (configured bool, configJSON string, err error)
}

// CredentialLister backs ListIntegrationCredentials via
// credential-broker-service's ListCredentialsByCategory RPC (TASK-038).
type CredentialLister interface {
	ListConfiguredProviders(ctx context.Context, tenantID string) ([]domain.Provider, error)
}

// CredentialRevoker backs RevokeAuth -- this service's disconnect path into
// credential-broker-service, new here (unlike scm-integration-service,
// which already had RevokeAuth to reuse -- see TASK-041's context note).
type CredentialRevoker interface {
	RevokeByOwner(ctx context.Context, tenantID string, provider domain.Provider) error
}

type OutboxEnqueuer interface {
	Enqueue(ctx context.Context, tenantID string, event domain.OutboxEvent) error
}
