package usecase

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type UpdateWorktreeMetaInput struct {
	WorktreeID string
	// Metadata is a partial-update JSON object patch — see
	// postgres.WorktreeRepository.UpdateWorktreeMeta's doc comment for the
	// shallow-merge semantics. Must be a JSON object; nil/empty means "no
	// change" (a harmless no-op, not an error — a caller might call this
	// with only lineage or activation fields set on the same wire request).
	Metadata json.RawMessage
}

// UpdateWorktreeMeta persists the UI-authored WorktreeMeta fields
// (displayName/comment/isPinned/pushTarget/sparse*/...) the frontend already
// keeps durable on desktop (orca-data.json) but backend-go had no
// persistence path for at all until this usecase — see
// proto/orca/project/v1/project.proto's UpdateWorktreeMetaRequest doc
// comment for the design rationale (opaque blob, not per-field columns).
type UpdateWorktreeMeta struct {
	repo WorktreeRepository
}

func NewUpdateWorktreeMeta(repo WorktreeRepository) *UpdateWorktreeMeta {
	return &UpdateWorktreeMeta{repo: repo}
}

func (uc *UpdateWorktreeMeta) Execute(ctx context.Context, in UpdateWorktreeMetaInput) (domain.Worktree, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.Worktree{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if in.WorktreeID == "" {
		return domain.Worktree{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_WORKTREE_ID_REQUIRED", "worktree_id is required", nil)
	}
	patch := in.Metadata
	if len(patch) == 0 {
		patch = json.RawMessage("{}")
	}

	updated, err := uc.repo.UpdateWorktreeMeta(ctx, in.WorktreeID, patch)
	if errors.Is(err, domain.ErrWorktreeNotFound) {
		return domain.Worktree{}, apperrors.New(apperrors.KindNotFound, "PROJECT_WORKTREE_NOT_FOUND", "worktree not found", err)
	}
	if err != nil {
		return domain.Worktree{}, apperrors.New(apperrors.KindInternal, "PROJECT_UPDATE_WORKTREE_META_FAILED", "failed to update worktree metadata", err)
	}
	return updated, nil
}
