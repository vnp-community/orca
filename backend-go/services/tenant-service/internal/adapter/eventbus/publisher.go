// Package eventbus implements tenant-service's cross-replica profile-cache
// invalidation broadcast against NATS JetStream via common/eventbus — see
// docs/execution-plan.md Epic F and this service's README "Known gaps" for
// why this exists: internal/adapter/cache.LRUTTLCache is an in-process
// cache, one instance per replica. A mutating usecase already invalidates
// the entry on the replica that served the write; the remaining gap was
// that every OTHER replica's copy of the same entry stayed correct only by
// waiting out its TTL, not by being told. Publishing this event is
// best-effort, not outbox-backed: a missed publish just means a peer
// replica falls back to today's TTL-bounded staleness, never wrong data —
// this doesn't need the stronger at-least-once guarantee a
// domain-of-record event would.
package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
)

// Subject is the event subject SetUserDepartment/AddTeamMember publish to
// after invalidating their own replica's cache entry. Every tenant-service
// replica (including the publisher) also subscribes to this subject — see
// consumer.go — so a resolved-profile cache stays correct cluster-wide
// within event-delivery latency, not just within
// usecase.DefaultProfileCacheTTL.
const Subject = "orca.tenant.profile.invalidated"

type invalidatedPayload struct {
	UserID string `json:"user_id"`
}

// Publisher implements usecase.CacheInvalidationPublisher.
type Publisher struct {
	pub *commoneventbus.Publisher
}

func New(pub *commoneventbus.Publisher) *Publisher {
	return &Publisher{pub: pub}
}

func (p *Publisher) PublishProfileInvalidated(ctx context.Context, tenantID, userID string) error {
	payload, err := json.Marshal(invalidatedPayload{UserID: userID})
	if err != nil {
		return fmt.Errorf("eventbus: marshal payload: %w", err)
	}
	return p.pub.Publish(ctx, Subject, commoneventbus.Event{
		ID:         uuid.NewString(),
		TenantID:   tenantID,
		OccurredAt: time.Now().UTC(),
		Version:    1,
		Payload:    payload,
	})
}

// AuditSubject is the event subject UpdateCompany/UpdateDepartment/
// CreateDepartment publish to after a successful write. Something must
// consume this and append into auth.audit_log for these events to reach
// QueryAuditLog — that consumer is auth-service's own follow-up work (it
// already owns writes to that table), not built by this task.
const AuditSubject = "orca.tenant.audit.recorded"

type auditPayload struct {
	Action  string `json:"action"`
	ActorID string `json:"actor_id"`
	Target  string `json:"target"`
}

// PublishAuditEvent implements usecase.AuditPublisher — best-effort, same
// posture as PublishProfileInvalidated above (a missed publish degrades the
// audit trail, it never blocks or fails the write it's reporting on).
func (p *Publisher) PublishAuditEvent(ctx context.Context, tenantID, actorID, action, target string) error {
	payload, err := json.Marshal(auditPayload{Action: action, ActorID: actorID, Target: target})
	if err != nil {
		return fmt.Errorf("eventbus: marshal audit payload: %w", err)
	}
	return p.pub.Publish(ctx, AuditSubject, commoneventbus.Event{
		ID:         uuid.NewString(),
		TenantID:   tenantID,
		OccurredAt: time.Now().UTC(),
		Version:    1,
		Payload:    payload,
	})
}
