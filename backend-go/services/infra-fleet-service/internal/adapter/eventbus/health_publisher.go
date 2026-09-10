// Package eventbus implements usecase.HealthEventPublisher against
// infra-fleet-service's own transactional-outbox table
// (migrations/0010_outbox, adapter/postgres.Repository.EnqueueOutboxEvent)
// — infra-fleet-service.md §7 already lists dev_server.health_degraded as
// an intended NATS JetStream event published via the outbox pattern; this
// fills in a documented integration point that had no writer to trigger it
// AND no outbox infrastructure at all (neither existed in this service
// before TASK-FLEET-03-06 — a correction from the task's own draft, which
// assumed an outbox.Outbox instance "already constructed for other event
// publishers in this service": no such instance, and no such type, existed
// — common/outbox exposes a Store port + Relay, not an Outbox/Enqueue
// helper; the enqueue INSERT is each service's own responsibility per that
// package's doc comment, same as usage-service's Repository.SaveSession).
// Follows tenant-service's internal/adapter/eventbus/publisher.go naming
// convention.
package eventbus

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// HealthDegradedSubject is the outbox subject/event type
// infra-fleet-service.md §7 documents for this event.
const HealthDegradedSubject = "dev_server.health_degraded"

// OutboxEnqueuer is the narrow port this package needs — declared here
// (consumer-side), not in adapter/postgres, per this codebase's existing
// Dependency Inversion convention (see e.g. adapter/sshrelay's
// SshTargetResolver). Implemented by adapter/postgres.Repository.
type OutboxEnqueuer interface {
	EnqueueOutboxEvent(ctx context.Context, id, tenantID, subject string, occurredAt time.Time, version int, payload []byte) error
}

// HealthPublisher implements usecase.HealthEventPublisher.
type HealthPublisher struct {
	store  OutboxEnqueuer
	logger *slog.Logger
}

func NewHealthPublisher(store OutboxEnqueuer, logger *slog.Logger) *HealthPublisher {
	if logger == nil {
		logger = slog.Default()
	}
	return &HealthPublisher{store: store, logger: logger}
}

// PublishStatusChange enqueues one dev_server.health_degraded outbox row —
// fire-and-forget from PollFleetHealth's perspective (see
// usecase.HealthEventPublisher's doc comment: no error return), so a
// marshal or enqueue failure is logged, never propagated.
func (p *HealthPublisher) PublishStatusChange(ctx context.Context, ds domain.DevServer, from, to domain.HealthStatus) {
	payload, err := json.Marshal(map[string]any{
		"devServerId": ds.ID, "host": ds.Host, "tenantId": ds.TenantID,
		"from": from, "to": to, "timestamp": time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		p.logger.ErrorContext(ctx, "health_publisher: marshal payload failed", slog.Any("error", err))
		return
	}
	if err := p.store.EnqueueOutboxEvent(ctx, uuid.NewString(), ds.TenantID, HealthDegradedSubject, time.Now().UTC(), 1, payload); err != nil {
		p.logger.ErrorContext(ctx, "health_publisher: enqueue failed", slog.Any("error", err))
	}
}
