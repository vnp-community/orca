package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type ImportNestedInput struct {
	// DevServerID is the dev server the candidates were scanned on
	// (project.proto's ImportNestedRequest.dev_server_id) — stamped onto
	// each created project's DevServerID, matching what ScanNested reported
	// them found under.
	DevServerID   string
	ParentGroupID string
	Selected      []domain.NestedRepoCandidate
}

// ImportNested needs no relay call — once the user has selected candidates,
// materializing them into project_groups (+ a Project/Repo per group) is
// pure metadata writes, one DB transaction (see
// ProjectGroupRepository.ImportNested's doc comment).
type ImportNested struct {
	groupRepo ProjectGroupRepository
}

func NewImportNested(groupRepo ProjectGroupRepository) *ImportNested {
	return &ImportNested{groupRepo: groupRepo}
}

func (uc *ImportNested) Execute(ctx context.Context, in ImportNestedInput) ([]domain.ProjectGroup, []domain.Project, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, nil, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return nil, nil, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_USER", "no user in request context", nil)
	}

	if in.ParentGroupID != "" {
		if _, err := uc.groupRepo.GetProjectGroup(ctx, tenantID, in.ParentGroupID); err != nil {
			return nil, nil, apperrors.New(apperrors.KindNotFound, "PROJECT_GROUP_NOT_FOUND", "parent group does not exist", err)
		}
	}

	groups, projects, err := uc.groupRepo.ImportNested(ctx, tenantID, userID, in.DevServerID, in.ParentGroupID, in.Selected)
	if err != nil {
		return nil, nil, apperrors.New(apperrors.KindInternal, "PROJECT_IMPORT_NESTED_FAILED", "failed to import nested repos", err)
	}
	return groups, projects, nil
}
