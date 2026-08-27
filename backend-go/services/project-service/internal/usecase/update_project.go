package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// UpdateProjectInput mirrors UpdateProjectRequest 1:1 — empty string on
// Name/Description/DefaultBranch/Visibility means "no change", per
// project.proto's doc comment. Deliberately has no DevServerID field:
// RebindDevServer (with its active-execution guard) stays the sole path
// that may change dev_server_id.
type UpdateProjectInput struct {
	ProjectID     string
	Name          string
	Description   string
	DefaultBranch string
	Visibility    string
	// IssueStatusSyncEnabled is presence-based (nil = no change) — see
	// domain.ProjectUpdatePatch's doc comment.
	IssueStatusSyncEnabled *bool
}

type UpdateProject struct {
	repo ProjectRepository
	opa  OPAClient
}

func NewUpdateProject(repo ProjectRepository, opa OPAClient) *UpdateProject {
	return &UpdateProject{repo: repo, opa: opa}
}

// Execute requires the caller's project role to be owner, or global admin —
// project-service.md §9's owner-only tier.
func (uc *UpdateProject) Execute(ctx context.Context, in UpdateProjectInput) (domain.Project, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Project{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if in.ProjectID == "" {
		return domain.Project{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_ID_REQUIRED", "project_id is required", nil)
	}
	if err := requireProjectAccess(ctx, uc.repo, uc.opa, in.ProjectID, projectActionOwnerOnly); err != nil {
		return domain.Project{}, err
	}
	if in.Visibility != "" && !domain.ValidVisibility(in.Visibility) {
		return domain.Project{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_INVALID_VISIBILITY", domain.ErrInvalidVisibility.Error(), domain.ErrInvalidVisibility)
	}

	patch := domain.ProjectUpdatePatch{
		Name:                   in.Name,
		Description:            in.Description,
		DefaultBranch:          in.DefaultBranch,
		Visibility:             in.Visibility,
		IssueStatusSyncEnabled: in.IssueStatusSyncEnabled,
	}

	updated, err := uc.repo.UpdateProject(ctx, tenantID, in.ProjectID, patch)
	if errors.Is(err, domain.ErrProjectNotFound) {
		return domain.Project{}, apperrors.New(apperrors.KindNotFound, "PROJECT_NOT_FOUND", "project not found", err)
	}
	if err != nil {
		return domain.Project{}, apperrors.New(apperrors.KindInternal, "PROJECT_UPDATE_FAILED", "failed to update project", err)
	}
	return updated, nil
}
