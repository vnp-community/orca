// Package eventbus implements project-service's outbox-pattern event
// publishing against NATS JetStream via common/eventbus — mirrors
// tenant-service/internal/adapter/eventbus/publisher.go's shape. This is
// this service's first adapter/eventbus package (find internal/adapter
// previously listed only postgres/grpc/opaclient/grpcclient).
package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
)

const StreamName = "PROJECT"
const AuditSubject = "orca.project.audit.recorded"
const DevServerChangedSubject = "orca.project.devserver.changed"

type Publisher struct {
	pub *commoneventbus.Publisher
}

func New(pub *commoneventbus.Publisher) *Publisher {
	return &Publisher{pub: pub}
}

type auditPayload struct {
	Action  string `json:"action"`
	ActorID string `json:"actor_id"`
	Target  string `json:"target"`
}

// PublishAuditEvent implements usecase.AuditPublisher.
func (p *Publisher) PublishAuditEvent(ctx context.Context, tenantID, actorID, action, target string) error {
	payload, err := json.Marshal(auditPayload{Action: action, ActorID: actorID, Target: target})
	if err != nil {
		return fmt.Errorf("eventbus: marshal audit payload: %w", err)
	}
	return p.pub.Publish(ctx, AuditSubject, commoneventbus.Event{
		ID: uuid.NewString(), TenantID: tenantID, OccurredAt: time.Now().UTC(), Version: 1, Payload: payload,
	})
}

// notificationEventPayload matches notification-service's generic
// EventPayload{UserIDs, Title, Body, DeepLink} shape exactly
// (notification_event.go:39-45) so it translates without a dedicated
// subjectRules entry even before TASK-PRF-03-08 adds one.
type notificationEventPayload struct {
	UserIDs  []string `json:"user_ids"`
	Title    string   `json:"title"`
	Body     string   `json:"body"`
	DeepLink string   `json:"deep_link"`
}

// NotifyDevServerChanged implements usecase.MemberNotifier.
func (p *Publisher) NotifyDevServerChanged(ctx context.Context, tenantID string, userIDs []string, projectID, oldDevServerID, newDevServerID string) error {
	payload, err := json.Marshal(notificationEventPayload{
		UserIDs: userIDs, Title: "Dev server changed",
		Body:     "This project's dev server binding was changed.",
		DeepLink: "/projects/" + projectID,
	})
	if err != nil {
		return fmt.Errorf("eventbus: marshal devserver-changed payload: %w", err)
	}
	return p.pub.Publish(ctx, DevServerChangedSubject, commoneventbus.Event{
		ID: uuid.NewString(), TenantID: tenantID, OccurredAt: time.Now().UTC(), Version: 1, Payload: payload,
	})
}
