package usecase

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type RecordWorktreeCreatedInput struct {
	ProjectID string
	RepoID    string
	Path      string
	Branch    string
	// LinkedIssueProvider/LinkedIssueRef come from git-gateway-service's
	// CreateWorktreeFromIssue saga (SOL-PI-02) — empty for a plain
	// CreateWorktree call.
	LinkedIssueProvider string
	LinkedIssueRef      string
}

// RecordWorktreeCreated is called by git-gateway-service AFTER the real
// `git worktree add` succeeded on the Dev Server Agent — this usecase only
// writes the bookkeeping row, it never triggers the git operation itself.
// See domain.Worktree's doc comment. As of SOL-PI-03, it also durably
// enqueues orca.project.worktree.created in the SAME transaction as the
// worktrees INSERT (WorktreeRepository.CreateWorktreeWithEvent) —
// git-gateway-service owns no database and cannot itself participate in
// the outbox pattern, so this service — already the durable writer of
// worktree existence — is where the outbox row belongs.
type RecordWorktreeCreated struct {
	repo WorktreeRepository
}

func NewRecordWorktreeCreated(repo WorktreeRepository) *RecordWorktreeCreated {
	return &RecordWorktreeCreated{repo: repo}
}

func (uc *RecordWorktreeCreated) Execute(ctx context.Context, in RecordWorktreeCreatedInput) (domain.Worktree, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Worktree{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}

	wt, err := domain.NewWorktree(uuid.NewString(), in.ProjectID, in.RepoID, in.Path, in.Branch)
	if err != nil {
		return domain.Worktree{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_WORKTREE_INVALID", err.Error(), err)
	}
	wt.LinkedIssueProvider = in.LinkedIssueProvider
	wt.LinkedIssueRef = in.LinkedIssueRef

	payload, _ := json.Marshal(worktreeLifecycleEventPayload{
		WorktreeID: wt.ID, ProjectID: in.ProjectID,
		LinkedIssueProvider: in.LinkedIssueProvider, LinkedIssueRef: in.LinkedIssueRef,
	})
	event := domain.OutboxEvent{
		ID: uuid.NewString(), TenantID: tenantID,
		Subject: subjectWorktreeCreated, OccurredAt: time.Now().UTC(), PayloadJSON: payload,
	}

	created, err := uc.repo.CreateWorktreeWithEvent(ctx, wt, event)
	if err != nil {
		return domain.Worktree{}, apperrors.New(apperrors.KindInternal, "PROJECT_RECORD_WORKTREE_FAILED", "failed to persist worktree", err)
	}
	return created, nil
}
