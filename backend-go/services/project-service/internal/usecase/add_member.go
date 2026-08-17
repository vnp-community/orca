package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type AddMemberInput struct {
	ProjectID string
	UserID    string
	Role      domain.ProjectRole
}

// AddMember links a user into a project. Role/owner-only authorization
// (owner or global admin per project-service.md §9) is enforced by the OPA
// policy check in the gRPC interceptor chain, not here — this usecase
// assumes the caller already passed that check and only validates/persists.
type AddMember struct {
	repo ProjectRepository
}

func NewAddMember(repo ProjectRepository) *AddMember {
	return &AddMember{repo: repo}
}

func (uc *AddMember) Execute(ctx context.Context, in AddMemberInput) (domain.ProjectMember, error) {
	// Every usecase method requires authenticated tenant context uniformly,
	// even though AddMember's write is scoped by project_id (already
	// tenant-bound via the projects FK / RLS) rather than a tenant_id column
	// of its own.
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.ProjectMember{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}

	member, err := domain.NewProjectMember(in.ProjectID, in.UserID, in.Role)
	if err != nil {
		return domain.ProjectMember{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_MEMBER_INVALID", err.Error(), err)
	}

	if err := uc.repo.AddMember(ctx, member); err != nil {
		return domain.ProjectMember{}, apperrors.New(apperrors.KindInternal, "PROJECT_ADD_MEMBER_FAILED", "failed to add project member", err)
	}
	return member, nil
}
