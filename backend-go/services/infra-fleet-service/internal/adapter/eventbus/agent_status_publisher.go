// Package eventbus — agent_status_publisher.go implements
// usecase.AgentStatusEventPublisher. statusChanged publishes DIRECTLY
// (bypassing the outbox — see TASK-AG-05-05's Context for why this is a
// deliberate, signed-off exception to 08-inter-service-communication.md's
// outbox-always rule); rateLimited goes through the outbox, same pattern
// usage-service's RecordUsageSession/Repository.SaveSession already
// establishes for a real alerting event.
package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

const (
	statusChangedSubject = "orca.infra.agent.statusChanged"
	rateLimitedSubject   = "orca.infra.agent.rateLimited"
)

type statusChangedPayload struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
}

type rateLimitedPayload struct {
	SessionID string `json:"session_id"`
}

// rateLimitedOutboxEnqueuer is the one method this package needs from
// postgres.AgentRateLimitedOutboxStore — a local interface (rather than
// importing internal/adapter/postgres directly) keeps this adapter package
// decoupled from the specific store implementation, same reasoning
// usecase/ports.go's port interfaces already apply one layer up.
type rateLimitedOutboxEnqueuer interface {
	Enqueue(ctx context.Context, rec outbox.Record) error
}

// AgentStatusPublisher implements usecase.AgentStatusEventPublisher.
type AgentStatusPublisher struct {
	pub   *commoneventbus.Publisher
	store rateLimitedOutboxEnqueuer // rateLimited only — statusChanged never touches it
}

func New(pub *commoneventbus.Publisher, store rateLimitedOutboxEnqueuer) *AgentStatusPublisher {
	return &AgentStatusPublisher{pub: pub, store: store}
}

func (p *AgentStatusPublisher) PublishStatusChanged(ctx context.Context, tenantID, sessionID string, status domain.AgentStatus) error {
	payload, err := json.Marshal(statusChangedPayload{SessionID: sessionID, Status: string(status)})
	if err != nil {
		return fmt.Errorf("eventbus: marshal statusChanged payload: %w", err)
	}
	return p.pub.Publish(ctx, statusChangedSubject, commoneventbus.Event{
		ID: uuid.NewString(), TenantID: tenantID, OccurredAt: time.Now().UTC(), Version: 1, Payload: payload,
	})
}

// PublishRateLimited enqueues into the outbox rather than publishing
// directly — see this file's package doc comment.
func (p *AgentStatusPublisher) PublishRateLimited(ctx context.Context, tenantID, sessionID string) error {
	payload, err := json.Marshal(rateLimitedPayload{SessionID: sessionID})
	if err != nil {
		return fmt.Errorf("eventbus: marshal rateLimited payload: %w", err)
	}
	return p.store.Enqueue(ctx, outbox.Record{
		ID:      uuid.NewString(),
		Subject: rateLimitedSubject,
		Event: commoneventbus.Event{
			TenantID: tenantID, OccurredAt: time.Now().UTC(), Version: 1, Payload: payload,
		},
	})
}
