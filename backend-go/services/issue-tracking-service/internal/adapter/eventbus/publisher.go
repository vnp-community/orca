// Package eventbus implements issue-tracking-service's EventPublisher port
// against NATS JetStream via common/eventbus — see
// specs/backend-go/architecture/08-inter-service-communication.md and
// issue-tracking-service.md §7. This is a real implementation, not a stub:
// LinkIssue's publish IS this service's persisted side effect.
package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
)

// Subject is the event subject LinkIssue publishes to — task-service and
// project-service consume it to update their own records of which
// task/worktree references which external issue (design doc §7).
const Subject = "orca.issuetracking.link.created"

type Publisher struct {
	pub *commoneventbus.Publisher
}

func New(pub *commoneventbus.Publisher) *Publisher {
	return &Publisher{pub: pub}
}

type linkCreatedPayload struct {
	IssueID string `json:"issue_id"`
	TaskID  string `json:"task_id"`
}

func (p *Publisher) PublishLinkCreated(ctx context.Context, tenantID, issueID, taskID string) error {
	payload, err := json.Marshal(linkCreatedPayload{IssueID: issueID, TaskID: taskID})
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
