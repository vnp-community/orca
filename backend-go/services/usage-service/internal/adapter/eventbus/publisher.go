// Package eventbus implements usage-service's EventPublisher port against
// NATS JetStream via common/eventbus — see
// specs/backend-go/architecture/08-inter-service-communication.md.
package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/services/usage-service/internal/domain"
)

const Subject = "orca.usage.session.recorded"

type Publisher struct {
	pub *commoneventbus.Publisher
}

func New(pub *commoneventbus.Publisher) *Publisher {
	return &Publisher{pub: pub}
}

type sessionRecordedPayload struct {
	SessionID string  `json:"session_id"`
	Provider  string  `json:"provider"`
	CostUSD   float64 `json:"cost_usd"`
}

func (p *Publisher) PublishSessionRecorded(ctx context.Context, tenantID string, s domain.UsageSession) error {
	payload, err := json.Marshal(sessionRecordedPayload{SessionID: s.ID, Provider: string(s.Provider), CostUSD: s.CostUSD})
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
