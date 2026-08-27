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
	BaseRef   string // NEW (SOL-WT-04)
}

// RecordWorktreeCreated is called by git-gateway-service AFTER the real
// `git worktree add` succeeded on the Dev Server Agent — this usecase only
// writes the bookkeeping row, it never triggers the git operation itself.
// See domain.Worktree's doc comment. Also enqueues a durable
// "orca.project.worktree.created" outbox event in the SAME transaction as
// the worktree insert (the transactional-outbox pattern,
// 05-data-architecture.md).
type RecordWorktreeCreated struct {
	repo WorktreeRepository
}

func NewRecordWorktreeCreated(repo WorktreeRepository) *RecordWorktreeCreated {
	return &RecordWorktreeCreated{repo: repo}
}

func (uc *RecordWorktreeCreated) Execute(ctx context.Context, in RecordWorktreeCreatedInput) (domain.Worktree, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.Worktree{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}

	wt, err := domain.NewWorktree(uuid.NewString(), in.ProjectID, in.RepoID, in.Path, in.Branch, in.BaseRef)
	if err != nil {
		return domain.Worktree{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_WORKTREE_INVALID", err.Error(), err)
	}

	payload, err := json.Marshal(worktreeCreatedPayload{WorktreeID: wt.ID, ProjectID: wt.ProjectID, RepoID: wt.RepoID, Path: wt.Path, Branch: wt.Branch})
	if err != nil {
		return domain.Worktree{}, apperrors.New(apperrors.KindInternal, "PROJECT_EVENT_MARSHAL_FAILED", "failed to build worktree.created event", err)
	}
	event := domain.OutboxEvent{ID: uuid.NewString(), Subject: "orca.project.worktree.created", OccurredAt: time.Now().UTC(), PayloadJSON: payload}

	created, err := uc.repo.RecordWorktreeCreated(ctx, wt, event)
	if err != nil {
		return domain.Worktree{}, apperrors.New(apperrors.KindInternal, "PROJECT_RECORD_WORKTREE_FAILED", "failed to persist worktree", err)
	}
	return created, nil
}

type worktreeCreatedPayload struct {
	WorktreeID string `json:"worktree_id"`
	ProjectID  string `json:"project_id"`
	RepoID     string `json:"repo_id"`
	Path       string `json:"path"`
	Branch     string `json:"branch"`
}
