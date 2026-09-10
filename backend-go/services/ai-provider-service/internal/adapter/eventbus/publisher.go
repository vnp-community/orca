// Package eventbus implements usecase.RateLimitEventPublisher against NATS
// JetStream via common/eventbus — mirrors tenant-service's
// internal/adapter/eventbus/publisher.go shape.
package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
)

const SubjectRateLimited = "orca.aiprovider.account.rate_limited"

type RateLimitPayload struct {
	AccountID string `json:"account_id"`
	Provider  string `json:"provider"`
	UserID    string `json:"user_id"`
	ResetAt   *int64 `json:"reset_at_unix_ms,omitempty"`
}

type Publisher struct{ pub *commoneventbus.Publisher }

func New(pub *commoneventbus.Publisher) *Publisher { return &Publisher{pub: pub} }

func (p *Publisher) PublishRateLimited(ctx context.Context, tenantID string, payload RateLimitPayload) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("eventbus: marshal rate-limit payload: %w", err)
	}
	return p.pub.Publish(ctx, SubjectRateLimited, commoneventbus.Event{
		ID: uuid.NewString(), TenantID: tenantID, OccurredAt: time.Now().UTC(), Version: 1, Payload: raw,
	})
}
