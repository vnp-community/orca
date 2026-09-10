package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type UnlinkSourceProjectInput struct {
	ContainerProjectID string
	SourceProjectID    string
}

// UnlinkSourceProject removes a source-project link — legacy TS
// reference's orcaProjects.unlinkSourceProject, owner (or global admin)
// only per project-service.md §9 / the legacy handler's docblock
// ("removing/altering is" owner-gated, unlike linking). Idempotent:
// unlinking an absent link is a success no-op, not an error — matches the
// legacy handler exactly.
type UnlinkSourceProject struct {
	sourceProjects SourceProjectRepository
	membership     MembershipRepository
	opa            OPAClient
}

func NewUnlinkSourceProject(sourceProjects SourceProjectRepository, membership MembershipRepository, opa OPAClient) *UnlinkSourceProject {
	return &UnlinkSourceProject{sourceProjects: sourceProjects, membership: membership, opa: opa}
}

func (uc *UnlinkSourceProject) Execute(ctx context.Context, in UnlinkSourceProjectInput) error {
	if err := requireProjectAccess(ctx, uc.membership, uc.opa, in.ContainerProjectID, projectActionOwnerOnly); err != nil {
		return err
	}
	if _, err := domain.NewSourceProject("", in.ContainerProjectID, in.SourceProjectID, ""); err != nil {
		return apperrors.New(apperrors.KindInvalidArgument, "PROJECT_SOURCE_PROJECT_INVALID", err.Error(), err)
	}
	if err := uc.sourceProjects.Unlink(ctx, in.ContainerProjectID, in.SourceProjectID); err != nil {
		return apperrors.New(apperrors.KindInternal, "PROJECT_UNLINK_SOURCE_PROJECT_FAILED", "failed to unlink source project", err)
	}
	return nil
}
