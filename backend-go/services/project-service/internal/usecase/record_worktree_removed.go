package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type RecordWorktreeRemovedInput struct {
	WorktreeID string
}

// RecordWorktreeRemoved hard-deletes the worktree row — see
// WorktreeRepository.RecordWorktreeRemoved's doc comment for the
// soft-vs-hard-delete decision. As of SOL-PI-03, it also durably enqueues
// orca.project.worktree.deleted in the same transaction as the DELETE
// (WorktreeRepository.RemoveWorktreeWithEvent).
//
// had_open_pr is ALWAYS published false — resolving the real value needs a
// live scm-integration-service call, which must not happen inside this DB
// transaction (05-data-architecture.md never sanctions a cross-service
// call from inside a DB tx). The consumer (issue-status-sync) resolves it
// at processing time instead, via GetPullRequestForBranch.
type RecordWorktreeRemoved struct {
	repo WorktreeRepository
}

func NewRecordWorktreeRemoved(repo WorktreeRepository) *RecordWorktreeRemoved {
	return &RecordWorktreeRemoved{repo: repo}
}

func (uc *RecordWorktreeRemoved) Execute(ctx context.Context, in RecordWorktreeRemovedInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if in.WorktreeID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "PROJECT_WORKTREE_ID_REQUIRED", "worktree_id is required", nil)
	}

	buildEvent := func(removed domain.Worktree) domain.OutboxEvent {
		payload, _ := json.Marshal(worktreeLifecycleEventPayload{
			WorktreeID: removed.ID, ProjectID: removed.ProjectID,
			LinkedIssueProvider: removed.LinkedIssueProvider, LinkedIssueRef: removed.LinkedIssueRef,
			HadOpenPr: false, // always false — see this type's doc comment
		})
		return domain.OutboxEvent{
			ID: uuid.NewString(), TenantID: tenantID,
			Subject: subjectWorktreeDeleted, OccurredAt: time.Now().UTC(), PayloadJSON: payload,
		}
	}

	err = uc.repo.RemoveWorktreeWithEvent(ctx, in.WorktreeID, buildEvent)
	if errors.Is(err, domain.ErrWorktreeNotFound) {
		return apperrors.New(apperrors.KindNotFound, "PROJECT_WORKTREE_NOT_FOUND", "worktree not found", err)
	}
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "PROJECT_REMOVE_WORKTREE_FAILED", "failed to remove worktree", err)
	}
	return nil
}
