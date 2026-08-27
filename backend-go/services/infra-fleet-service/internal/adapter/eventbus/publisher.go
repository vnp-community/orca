// Package eventbus implements usecase.LifecycleEventPublisher against NATS
// JetStream via common/eventbus — mirrors tenant-service's
// internal/adapter/eventbus/publisher.go shape (best-effort, not
// outbox-backed: a missed publish only means a mobile push notification is
// late/missing, not a correctness issue for the terminal session itself).
package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
)

// StreamName is the JetStream stream this package publishes into —
// EnsureStream'd by cmd/server/main.go against the "orca.infra.>" subject
// prefix covering every subject below.
const StreamName = "INFRA"

const (
	SubjectAgentCompleted = "orca.infra.terminal_session.agent_completed"
	SubjectAgentError     = "orca.infra.terminal_session.agent_error"
	SubjectAgentWaiting   = "orca.infra.terminal_session.agent_waiting"
)

// AgentLifecyclePayload is the JSON body for every subject above —
// notification-service's TranslateEvent decodes it to build a mobile push
// (BL-MB-02). UserIDs empty means "no known recipient" — TranslateEvent
// treats that as a no-op (ErrNoRecipients), not an error.
type AgentLifecyclePayload struct {
	PtyID        string   `json:"pty_id"`
	ConnectionID string   `json:"connection_id"`
	AgentKind    string   `json:"agent_kind"`
	ExitCode     *int32   `json:"exit_code,omitempty"`
	UserIDs      []string `json:"user_ids"`
}

type Publisher struct {
	pub *commoneventbus.Publisher
}

func New(pub *commoneventbus.Publisher) *Publisher { return &Publisher{pub: pub} }

func (p *Publisher) PublishAgentLifecycle(ctx context.Context, tenantID, subject string, payload AgentLifecyclePayload) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("eventbus: marshal agent lifecycle payload: %w", err)
	}
	return p.pub.Publish(ctx, subject, commoneventbus.Event{
		ID: uuid.NewString(), TenantID: tenantID, OccurredAt: time.Now().UTC(), Version: 1, Payload: raw,
	})
}
