package usecase

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

// LinkCreatedSubject is the event subject a link's outbox row publishes
// under — task-service/project-service consume it to update their own
// records of which task/worktree references which external issue (design
// doc §7).
const LinkCreatedSubject = "orca.issuetracking.link.created"

// linkCreatedPayload is LinkCreatedSubject's JSON payload shape.
type linkCreatedPayload struct {
	IssueID string `json:"issue_id"`
	TaskID  string `json:"task_id"`
}

// LinkIssueInput mirrors the LinkIssue gRPC request 1:1 by design.
type LinkIssueInput struct {
	IssueID string
	TaskID  string
}

// LinkIssue durably enqueues orca.issuetracking.link.created instead of
// writing directly into task-service/project-service's data (design doc
// §7) — this replaces the TS behavior of writing a field onto the
// worktree's Postgres blob row directly, which is no longer legal once
// task-service and project-service own separate databases.
//
// The enqueue IS this usecase's persisted side effect: issue-tracking-service
// gained its own (minimal, outbox-only) database in Epic G
// (docs/execution-plan.md) specifically because Enqueue's own doc comment
// on OutboxEnqueuer explains why — there's no other domain state to be
// atomic with. A failed enqueue still fails the RPC, same as before Epic G;
// what changed is that a transient NATS outage no longer needs to surface
// as a failed enqueue at all (see common/outbox.Relay), only a database
// outage does.
type LinkIssue struct {
	enqueuer OutboxEnqueuer
}

func NewLinkIssue(enqueuer OutboxEnqueuer) *LinkIssue {
	return &LinkIssue{enqueuer: enqueuer}
}

func (uc *LinkIssue) Execute(ctx context.Context, in LinkIssueInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	if in.IssueID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_EMPTY_ISSUE_ID", "issue_id is required", nil)
	}
	if in.TaskID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_EMPTY_TASK_ID", "task_id is required", nil)
	}
	if uc.enqueuer == nil {
		return apperrors.New(apperrors.KindInternal, "ISSUETRACKING_OUTBOX_UNAVAILABLE", "outbox store is not configured", nil)
	}

	payload, err := json.Marshal(linkCreatedPayload{IssueID: in.IssueID, TaskID: in.TaskID})
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "ISSUETRACKING_MARSHAL_EVENT_FAILED", "failed to marshal link-created event payload", err)
	}
	event := domain.OutboxEvent{
		ID:          uuid.NewString(),
		Subject:     LinkCreatedSubject,
		OccurredAt:  time.Now().UTC(),
		PayloadJSON: payload,
	}

	if err := uc.enqueuer.Enqueue(ctx, tenantID, event); err != nil {
		return apperrors.New(apperrors.KindInternal, "ISSUETRACKING_LINK_ENQUEUE_FAILED", "failed to durably enqueue link.created event", err)
	}
	return nil
}
