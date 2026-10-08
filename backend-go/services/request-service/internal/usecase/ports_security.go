package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// AccessPolicy is the request.rego decision (opaclient.RequestPolicy).
type AccessPolicy interface {
	Allow(ctx context.Context, in domain.AccessInput) (bool, error)
}

// ProjectRoleResolver answers from project-service membership; "" means not a member.
type ProjectRoleResolver interface {
	RoleOf(ctx context.Context, tenantID, projectID, userID string) (string, error)
	ProjectsOf(ctx context.Context, tenantID, userID string) ([]string, error)
}

// ApprovalLocator maps an approval to its Request without a row lock.
type ApprovalLocator interface {
	RequestIDOf(ctx context.Context, approvalID string) (string, error)
}

// EntityRequestResolver maps a child entity (clarification, decision, finding...) to its Request.
// Owners of those entities register resolvers; none registered means non-admins are refused.
type EntityRequestResolver interface {
	RequestIDOf(ctx context.Context, id string) (string, error)
}

// DeniedAccess is the audit payload of a refused RPC: ids and names only, never content.
type DeniedAccess struct {
	RPC       string
	RequestID string
	ActorType string
}

type AccessAuditor interface {
	RecordDenied(ctx context.Context, d DeniedAccess)
}

// Clock is injected so expiry and retention tests do not sleep.
type Clock interface{ Now() time.Time }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// AuditOutboxRecord is one audit entry waiting for delivery to auth-service.
type AuditOutboxRecord struct {
	ID, AuditID, TenantID                    string
	Action, ActorID, ActorType               string
	TargetType, TargetID, Outcome, IPAddress string
	MetadataJSON                             string
	Attempts                                 int
	CreatedAt                                time.Time
}

// AuditOutboxStore persists durable audit entries. Enqueue joins the transaction in ctx so the entry commits
// or rolls back with the change it describes.
type AuditOutboxStore interface {
	Enqueue(ctx context.Context, rec AuditOutboxRecord) error
	// ProcessDue locks due rows (SKIP LOCKED) across tenants, calls deliver for each and records the outcome:
	// delivered_at on success, attempts+1 and next_attempt_at=now+backoff(attempts) on error.
	ProcessDue(ctx context.Context, now time.Time, limit int, backoff func(attempts int) time.Duration, deliver func(context.Context, AuditOutboxRecord) error) (delivered, failed int, err error)
	PurgeDelivered(ctx context.Context, before time.Time, limit int) (int, error)
	CountPending(ctx context.Context) (int, error)
}

// WebhookNonceStore remembers signatures already seen so a captured webhook cannot be replayed.
type WebhookNonceStore interface {
	// Remember reports false when the nonce was already recorded (and not yet expired).
	Remember(ctx context.Context, tenantID, source, nonceHash string, expiresAt time.Time) (accepted bool, err error)
	PruneExpired(ctx context.Context, now time.Time, limit int) (int, error)
}

// SecurityFlags is the per-Request security state kept beside the requests table.
type SecurityFlags struct {
	ContainsSecretSuspected bool
	ErasedAt                *time.Time
	ErasedBy                string
}

type SecurityFlagStore interface {
	MarkSecretSuspected(ctx context.Context, requestID string) error
	Get(ctx context.Context, requestID string) (SecurityFlags, error)
}

// ConcurrencyCounter counts work in flight from the database so caps hold across replicas.
type ConcurrencyCounter interface {
	CountRunning(ctx context.Context, projectID string) (int, error)
	CountOpenRequests(ctx context.Context) (int, error)
}

// RetentionStore reads settings and anonymizes Request content. Methods other than ListTenants run
// for the tenant in ctx; ListTenants is the one cross-tenant read (a relay-style scan).
type RetentionStore interface {
	ListTenants(ctx context.Context) ([]string, error)
	Settings(ctx context.Context) (domain.RetentionSettings, error)
	// AnonymizeExpired locks up to limit finished Requests older than cutoff (SKIP LOCKED, so replicas do not
	// overlap), clears their content and returns how many it changed.
	AnonymizeExpired(ctx context.Context, cutoff time.Time, limit int, pseudonym func(reporterID string) string, at time.Time) (int, error)
	// Anonymize does the same for one Request and reports false when it was already erased.
	Anonymize(ctx context.Context, requestID string, pseudonym func(reporterID string) string, at time.Time, erasedBy string) (bool, error)
}
