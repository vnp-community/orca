package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// ProjectAccess answers "does this project exist in the tenant, and what is the user's role in it".
type ProjectAccess struct {
	Exists bool
	// Role is "owner" or "member"; empty when the user is not a member or the project does not exist.
	Role string
}

// projectAccessRepository is the part of ProjectRepository this use case reads.
type projectAccessRepository interface {
	Get(ctx context.Context, tenantID, id string) (domain.Project, error)
	MembershipRepository
}

// ResolveProjectAccess is an internal-service lookup (guarded by the service token at the gRPC edge). Callers used to infer
// "the project is missing" from a refusal or an empty member list, which cannot tell a missing project from a project the
// user does not belong to; this reports both facts separately and never as an error.
type ResolveProjectAccess struct {
	repo projectAccessRepository
}

func NewResolveProjectAccess(repo projectAccessRepository) *ResolveProjectAccess {
	return &ResolveProjectAccess{repo: repo}
}

func (uc *ResolveProjectAccess) Execute(ctx context.Context, projectID, userID string) (ProjectAccess, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ProjectAccess{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if projectID == "" {
		return ProjectAccess{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_ID_REQUIRED", "project_id is required", nil)
	}
	if _, err := uc.repo.Get(ctx, tenantID, projectID); err != nil {
		if errors.Is(err, domain.ErrProjectNotFound) {
			return ProjectAccess{Exists: false}, nil
		}
		return ProjectAccess{}, apperrors.New(apperrors.KindInternal, "PROJECT_FETCH_FAILED", "failed to fetch project", err)
	}
	if userID == "" {
		return ProjectAccess{Exists: true}, nil
	}
	m, err := uc.repo.GetMembership(ctx, projectID, userID)
	switch {
	case errors.Is(err, domain.ErrMembershipNotFound):
		return ProjectAccess{Exists: true}, nil
	case err != nil:
		return ProjectAccess{}, apperrors.New(apperrors.KindInternal, "PROJECT_MEMBERSHIP_LOOKUP_FAILED", "failed to resolve the user's project membership", err)
	}
	return ProjectAccess{Exists: true, Role: string(m.Role)}, nil
}
