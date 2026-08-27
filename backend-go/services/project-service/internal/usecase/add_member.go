package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type AddMemberInput struct {
	ProjectID string
	UserID    string
	Role      domain.ProjectRole
}

// AddMember links a user into a project. Requires the caller's project role
// to be owner, or global admin — project-service.md §9's owner-only tier —
// enforced here via requireProjectAccess, with one bootstrap exception: see
// Execute's doc comment.
type AddMember struct {
	repo ProjectRepository
	opa  OPAClient
}

func NewAddMember(repo ProjectRepository, opa OPAClient) *AddMember {
	return &AddMember{repo: repo, opa: opa}
}

func (uc *AddMember) Execute(ctx context.Context, in AddMemberInput) (domain.ProjectMember, error) {
	// Every usecase method requires authenticated tenant context uniformly,
	// even though AddMember's write is scoped by project_id (already
	// tenant-bound via the projects FK / RLS) rather than a tenant_id column
	// of its own.
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ProjectMember{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	actorID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.ProjectMember{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_USER", "no user in request context", nil)
	}

	member, err := domain.NewProjectMember(in.ProjectID, in.UserID, in.Role)
	if err != nil {
		return domain.ProjectMember{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_MEMBER_INVALID", err.Error(), err)
	}

	// Bootstrap exception (judgment call): the project's recorded creator
	// adding THEMSELVES bypasses the owner-only gate below.
	// CreateProject.Execute's doc comment documents api-gateway's flow — the
	// creator becomes an owner via a follow-up AddMember call made before
	// any membership row exists for that project yet. Requiring existing
	// project ownership to become a project's first owner would be an
	// unsatisfiable deadlock. Scoped tightly to self-add
	// (in.UserID == actorID) by the project's own recorded creator — adding
	// any OTHER user, or self-adding to a project this caller didn't
	// create, still goes through the owner-only check.
	isCreatorBootstrap := false
	if in.UserID == actorID {
		project, getErr := uc.repo.Get(ctx, tenantID, in.ProjectID)
		if getErr != nil && !errors.Is(getErr, domain.ErrProjectNotFound) {
			return domain.ProjectMember{}, apperrors.New(apperrors.KindInternal, "PROJECT_FETCH_FAILED", "failed to fetch project", getErr)
		}
		isCreatorBootstrap = getErr == nil && project.CreatedBy == actorID
	}

	if !isCreatorBootstrap {
		if err := requireProjectAccess(ctx, uc.repo, uc.opa, in.ProjectID, projectActionOwnerOnly); err != nil {
			return domain.ProjectMember{}, err
		}
	}

	if err := uc.repo.AddMember(ctx, member); err != nil {
		return domain.ProjectMember{}, apperrors.New(apperrors.KindInternal, "PROJECT_ADD_MEMBER_FAILED", "failed to add project member", err)
	}
	return member, nil
}
