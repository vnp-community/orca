// Package eventbus implements task-service's usecase.EventPublisher port
// via the transactional-outbox pattern (specs/backend-go/architecture/05-data-architecture.md),
// mirroring usage-service's internal/adapter/postgres outbox-write +
// common/outbox.Relay polling-publish shape (usage-service has no separate
// adapter/eventbus package of its own — the outbox write lives on its
// Repository directly; this package exists here only because task-service's
// EventPublisher port is a thin, no-error-return, "best-effort" shape
// distinct from a repository method, per TASK-TG-03-07's given port
// signature).
package eventbus

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// OutboxWriter is the narrow persistence port Publisher needs — satisfied
// by internal/adapter/postgres.Repository.WriteOutboxEvent.
type OutboxWriter interface {
	WriteOutboxEvent(ctx context.Context, tenantID string, event domain.OutboxEvent) error
}

// Publisher implements usecase.EventPublisher — every call durably
// enqueues an outbox row; common/outbox.Relay (wired in cmd/server/main.go)
// is what actually gets it to NATS. Publish deliberately has no error
// return (see usecase.EventPublisher's doc comment: "best-effort") — a
// write failure is logged, never surfaced to the caller, since a missed
// audit event must never fail the grant mutation it's describing.
type Publisher struct {
	writer OutboxWriter
	logger *slog.Logger
}

func New(writer OutboxWriter, logger *slog.Logger) *Publisher {
	if logger == nil {
		logger = slog.Default()
	}
	return &Publisher{writer: writer, logger: logger}
}

func (p *Publisher) Publish(ctx context.Context, tenantID, eventType string, payload map[string]any) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		p.logger.WarnContext(ctx, "eventbus: marshal payload failed, event dropped",
			slog.String("subject", eventType), slog.Any("error", err))
		return
	}
	event := domain.OutboxEvent{
		ID:          uuid.NewString(),
		Subject:     eventType,
		OccurredAt:  time.Now().UTC(),
		PayloadJSON: payloadJSON,
	}
	if err := p.writer.WriteOutboxEvent(ctx, tenantID, event); err != nil {
		p.logger.WarnContext(ctx, "eventbus: write outbox event failed, event dropped",
			slog.String("subject", eventType), slog.Any("error", err))
	}
}
