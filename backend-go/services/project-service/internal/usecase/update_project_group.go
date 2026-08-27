package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type UpdateProjectGroupInput struct {
	GroupID string
	Name    string
}

// UpdateProjectGroup renames a group only — parent_group_id is never
// rewired through this path, deliberately, the same way workflow-service's
// WorkflowTemplate avoids an UpdateTemplate parent-rewrite RPC: allowing an
// existing node to move to a new parent reopens a cycle-detection problem
// (a group could be rewired to become a descendant of its own descendant)
// that CreateProjectGroup's simple "parent must already exist" check
// avoids entirely by construction. See domain.ErrGroupSelfParent's doc
// comment and docs/execution-plan.md §11's identical precedent. If
// parent-rewiring is ever needed, it should ship as a dedicated RPC with
// its own cycle check, not folded into this one.
type UpdateProjectGroup struct {
	repo ProjectGroupRepository
}

func NewUpdateProjectGroup(repo ProjectGroupRepository) *UpdateProjectGroup {
	return &UpdateProjectGroup{repo: repo}
}

func (uc *UpdateProjectGroup) Execute(ctx context.Context, in UpdateProjectGroupInput) (domain.ProjectGroup, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ProjectGroup{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if in.GroupID == "" {
		return domain.ProjectGroup{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_GROUP_ID_REQUIRED", "group_id is required", nil)
	}
	if in.Name == "" {
		return domain.ProjectGroup{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_GROUP_INVALID", domain.ErrEmptyName.Error(), domain.ErrEmptyName)
	}

	updated, err := uc.repo.UpdateProjectGroup(ctx, tenantID, in.GroupID, in.Name)
	if errors.Is(err, domain.ErrProjectGroupNotFound) {
		return domain.ProjectGroup{}, apperrors.New(apperrors.KindNotFound, "PROJECT_GROUP_NOT_FOUND", "project group not found", err)
	}
	if err != nil {
		return domain.ProjectGroup{}, apperrors.New(apperrors.KindInternal, "PROJECT_GROUP_UPDATE_FAILED", "failed to update project group", err)
	}
	return updated, nil
}
