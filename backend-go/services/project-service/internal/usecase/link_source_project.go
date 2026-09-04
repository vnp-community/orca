package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type LinkSourceProjectInput struct {
	ContainerProjectID string
	SourceProjectID    string
}

// LinkSourceProject links sourceProjectID's repos/worktrees into
// containerProjectID's shared view — legacy TS reference's
// orcaProjects.linkSourceProject. Any member of containerProjectID may
// link (project-service.md §9, matching the legacy handler's docblock:
// "contributing your own Project in is not owner-gated — only removing/
// altering is"). The caller must ALSO have access to sourceProjectID
// itself — this service's equivalent of legacy's "ownerUserId must match
// acting user" anti-spoofing check (there, a per-user JSON blob only the
// owner could name; here, an ordinary membership check on the project
// being shared), so a member of container A can't link an arbitrary
// project B they have no access to.
type LinkSourceProject struct {
	sourceProjects SourceProjectRepository
	membership     MembershipRepository
	opa            OPAClient
}

func NewLinkSourceProject(sourceProjects SourceProjectRepository, membership MembershipRepository, opa OPAClient) *LinkSourceProject {
	return &LinkSourceProject{sourceProjects: sourceProjects, membership: membership, opa: opa}
}

func (uc *LinkSourceProject) Execute(ctx context.Context, in LinkSourceProjectInput) (domain.SourceProject, error) {
	actorID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.SourceProject{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_USER", "no user in request context", nil)
	}

	if err := requireProjectAccess(ctx, uc.membership, uc.opa, in.ContainerProjectID, projectActionAnyMember); err != nil {
		return domain.SourceProject{}, err
	}
	if err := requireProjectAccess(ctx, uc.membership, uc.opa, in.SourceProjectID, projectActionAnyMember); err != nil {
		return domain.SourceProject{}, err
	}

	sp, err := domain.NewSourceProject(uuid.NewString(), in.ContainerProjectID, in.SourceProjectID, actorID)
	if err != nil {
		return domain.SourceProject{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_SOURCE_PROJECT_INVALID", err.Error(), err)
	}

	linked, err := uc.sourceProjects.Link(ctx, sp)
	if err != nil {
		return domain.SourceProject{}, apperrors.New(apperrors.KindInternal, "PROJECT_LINK_SOURCE_PROJECT_FAILED", "failed to link source project", err)
	}
	return linked, nil
}
