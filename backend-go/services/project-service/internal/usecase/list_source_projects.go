package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type ListSourceProjectsInput struct {
	ContainerProjectID string
}

// ListSourceProjects returns every source project linked into
// containerProjectID's shared view — any member (including the OPA
// any_member tier's viewer, per project-service.md §9). Backs
// orcaProjects.list's per-project N+1 lookup (see wscompat's
// channels_orca_project_sharing.go doc comment for why N+1 here is
// acceptable, matching the legacy handler's own Promise.all pattern).
type ListSourceProjects struct {
	sourceProjects SourceProjectRepository
	membership     MembershipRepository
	opa            OPAClient
}

func NewListSourceProjects(sourceProjects SourceProjectRepository, membership MembershipRepository, opa OPAClient) *ListSourceProjects {
	return &ListSourceProjects{sourceProjects: sourceProjects, membership: membership, opa: opa}
}

func (uc *ListSourceProjects) Execute(ctx context.Context, in ListSourceProjectsInput) ([]domain.SourceProject, error) {
	if err := requireProjectAccess(ctx, uc.membership, uc.opa, in.ContainerProjectID, projectActionAnyMember); err != nil {
		return nil, err
	}
	list, err := uc.sourceProjects.List(ctx, in.ContainerProjectID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "PROJECT_LIST_SOURCE_PROJECTS_FAILED", "failed to list source projects", err)
	}
	return list, nil
}
