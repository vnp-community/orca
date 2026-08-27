package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type MoveProjectInput struct {
	ProjectID           string
	TargetParentGroupID string
}

// MoveProject requires owner (or global admin) — same tier as
// UpdateProject/DeleteProject, project-service.md §9.
type MoveProject struct {
	repo      ProjectRepository
	groupRepo ProjectGroupRepository
	opa       OPAClient
}

func NewMoveProject(repo ProjectRepository, groupRepo ProjectGroupRepository, opa OPAClient) *MoveProject {
	return &MoveProject{repo: repo, groupRepo: groupRepo, opa: opa}
}

func (uc *MoveProject) Execute(ctx context.Context, tenantID string, in MoveProjectInput) (domain.ProjectGroup, error) {
	if err := requireProjectAccess(ctx, uc.repo, uc.opa, in.ProjectID, projectActionOwnerOnly); err != nil {
		return domain.ProjectGroup{}, err
	}

	if in.TargetParentGroupID != "" {
		if _, err := uc.groupRepo.GetProjectGroup(ctx, tenantID, in.TargetParentGroupID); err != nil {
			return domain.ProjectGroup{}, apperrors.New(apperrors.KindNotFound, "PROJECT_GROUP_NOT_FOUND", "target parent group does not exist", err)
		}
	}

	project, err := uc.repo.Get(ctx, tenantID, in.ProjectID)
	if err != nil {
		return domain.ProjectGroup{}, apperrors.New(apperrors.KindNotFound, "PROJECT_NOT_FOUND", "project does not exist", err)
	}

	// A leaf group holding exactly one project_id has no children by
	// construction — no cycle check needed, same reasoning
	// domain.ErrGroupSelfParent's doc comment documents for the general
	// parent-assignment case.
	group, err := uc.groupRepo.UpsertLeafGroupForProject(ctx, tenantID, in.ProjectID, project.Name, in.TargetParentGroupID)
	if err != nil {
		return domain.ProjectGroup{}, apperrors.New(apperrors.KindInternal, "PROJECT_MOVE_FAILED", "failed to move project", err)
	}
	return group, nil
}
