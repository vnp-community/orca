package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type DeleteProjectGroupInput struct {
	GroupID string
}

// DeleteProjectGroup deletes a group. Descendant groups cascade (ON DELETE
// CASCADE on parent_group_id — see migrations/0005) rather than being
// orphaned to root or blocking the delete: a folder-tree node's children
// have no independent meaning once their parent folder is gone, mirroring
// DeleteProject's cascade rationale for repos/worktrees.
type DeleteProjectGroup struct {
	repo ProjectGroupRepository
}

func NewDeleteProjectGroup(repo ProjectGroupRepository) *DeleteProjectGroup {
	return &DeleteProjectGroup{repo: repo}
}

func (uc *DeleteProjectGroup) Execute(ctx context.Context, in DeleteProjectGroupInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if in.GroupID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "PROJECT_GROUP_ID_REQUIRED", "group_id is required", nil)
	}

	err = uc.repo.DeleteProjectGroup(ctx, tenantID, in.GroupID)
	if errors.Is(err, domain.ErrProjectGroupNotFound) {
		return apperrors.New(apperrors.KindNotFound, "PROJECT_GROUP_NOT_FOUND", "project group not found", err)
	}
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "PROJECT_GROUP_DELETE_FAILED", "failed to delete project group", err)
	}
	return nil
}
