package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// FlowSettingsRepository stores the per-tenant request_flow_enabled row of the ctx tenant.
type FlowSettingsRepository interface {
	// Get reports ok=false when the tenant never set the flag.
	Get(ctx context.Context) (s domain.FlowSettings, ok bool, err error)
	Upsert(ctx context.Context, enabled bool, updatedBy string) error
}

// ActiveSourceFinder answers LookupRequestBySource: the newest Request of the ctx tenant for an
// external issue that is not completed or cancelled. An empty site matches any site.
type ActiveSourceFinder interface {
	FindActiveBySource(ctx context.Context, provider, site, ref string) (requestID string, ok bool, err error)
}

// RPCAuditEvent is one decision to record. Metadata keys are filtered by the adapter allow-list.
type RPCAuditEvent struct {
	TenantID   string
	ActorID    string
	ActorKind  domain.AuditActorKind
	Action     string
	TargetType string
	TargetID   string
	Outcome    string
	Metadata   map[string]string
}

// Target is the audit_log.target column: "<type>:<id>".
func (e RPCAuditEvent) Target() string { return e.TargetType + ":" + e.TargetID }

// RPCAuditRecorder is best effort: a failure must never undo or fail the decision it describes.
type RPCAuditRecorder interface {
	Record(ctx context.Context, e RPCAuditEvent)
}

type NoopRPCAuditRecorder struct{}

func (NoopRPCAuditRecorder) Record(context.Context, RPCAuditEvent) {}

// RequestObserver receives the metrics that cannot be derived from outbox events (everything else is
// counted by the metrics OutboxTap). Arguments are low-cardinality constants, never ids or text.
type RequestObserver interface {
	// ObserveAIGeneration times one AI call; kind is classification|solution|plan|diagnosis, outcome ok|error.
	ObserveAIGeneration(kind, outcome string, elapsed time.Duration)
	// ObserveTaskOutcome counts a task result reported back by task-service (succeeded|failed|cancelled).
	ObserveTaskOutcome(outcome string)
}

type NoopRequestObserver struct{}

func (NoopRequestObserver) ObserveAIGeneration(string, string, time.Duration) {}
func (NoopRequestObserver) ObserveTaskOutcome(string)                         {}

// MetricsSampleSource backs the gauges that are read from the database, across all tenants.
// It reports counts only; no row content leaves the adapter.
type MetricsSampleSource interface {
	// PendingApprovalsBySubject counts approvals in status pending by subject_type.
	PendingApprovalsBySubject(ctx context.Context) (map[string]int, error)
	// CountStuck counts Requests in status whose updated_at is before olderThan.
	CountStuck(ctx context.Context, status string, olderThan time.Time) (int, error)
	// CountOutboxPending counts outbox rows not yet published.
	CountOutboxPending(ctx context.Context) (int, error)
}
