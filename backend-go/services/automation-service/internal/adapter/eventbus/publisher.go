// Package eventbus implements automation-service's outbound
// orca.automation.run.completed publish (transactional-outbox, see
// publisher.go) and inbound event-trigger subscription (durable JetStream
// consumer, see consumer.go) against NATS JetStream via common/eventbus.
package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
)

// RunCompletedSubject is the event subject notification-service already
// subscribes to (automation-service.md) — nothing published it before this
// package existed.
const RunCompletedSubject = "orca.automation.run.completed"

type runCompletedPayload struct {
	AutomationID string `json:"automation_id"`
	RunID        string `json:"run_id"`
	Status       string `json:"status"`
}

// RunCompletedPublisher writes the run-completed outbox entry — see
// PublishRunCompleted.
type RunCompletedPublisher struct{}

func NewRunCompletedPublisher() *RunCompletedPublisher {
	return &RunCompletedPublisher{}
}

// PublishRunCompleted writes the run-completed outbox entry inside tx — the
// SAME transaction as the terminal status UPDATE
// (internal/adapter/postgres.AutomationRunRepository.UpdateStatus), per the
// transactional-outbox rule (never a direct publish call inside a request
// handler). common/outbox.Relay (wired in cmd/server/main.go) polls
// automation.outbox_events and actually delivers to NATS.
func (p *RunCompletedPublisher) PublishRunCompleted(ctx context.Context, tx pgx.Tx, run domain.AutomationRun) error {
	payload, err := json.Marshal(runCompletedPayload{AutomationID: run.AutomationID, RunID: run.ID, Status: string(run.Status)})
	if err != nil {
		return fmt.Errorf("eventbus: marshal run-completed payload: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO automation.outbox_events (id, tenant_id, subject, occurred_at, version, payload)
		VALUES ($1, $2, $3, $4, 1, $5)
	`, uuid.NewString(), run.TenantID, RunCompletedSubject, time.Now().UTC(), payload)
	if err != nil {
		return fmt.Errorf("eventbus: insert outbox event: %w", err)
	}
	return nil
}
