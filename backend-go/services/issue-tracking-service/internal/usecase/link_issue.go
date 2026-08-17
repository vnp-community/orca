package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// LinkIssueInput mirrors the LinkIssue gRPC request 1:1 by design.
type LinkIssueInput struct {
	IssueID string
	TaskID  string
}

// LinkIssue publishes orca.issuetracking.link.created instead of writing
// directly into task-service/project-service's data (design doc §7) — this
// replaces the TS behavior of writing a field onto the worktree's Postgres
// blob row directly, which is no longer legal once task-service and
// project-service own separate databases.
//
// The publish IS this use case's persisted side effect: issue-tracking-service
// owns no database of its own (design doc §2/§5), so unlike
// usage-service's RecordUsageSession (where the event is a best-effort
// bonus alongside a real DB write), a failed publish here must fail the RPC
// rather than silently succeed with nothing durable to show for it.
type LinkIssue struct {
	publisher EventPublisher
}

func NewLinkIssue(publisher EventPublisher) *LinkIssue {
	return &LinkIssue{publisher: publisher}
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
	if uc.publisher == nil {
		return apperrors.New(apperrors.KindInternal, "ISSUETRACKING_EVENTBUS_UNAVAILABLE", "event bus is not configured", nil)
	}

	if err := uc.publisher.PublishLinkCreated(ctx, tenantID, in.IssueID, in.TaskID); err != nil {
		return apperrors.New(apperrors.KindInternal, "ISSUETRACKING_LINK_PUBLISH_FAILED", "failed to publish link.created event", err)
	}
	return nil
}
