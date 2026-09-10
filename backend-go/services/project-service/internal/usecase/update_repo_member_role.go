package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type UpdateRepoMemberRoleInput struct {
	RepoID string
	UserID string
	Role   domain.RepoRole
}

// UpdateRepoMemberRole changes a functional-role grant — same
// repo_admin_only tier as AddRepoMember/RemoveRepoMember.
type UpdateRepoMemberRole struct {
	repo       RepoRepository
	membership MembershipRepository
	opa        OPAClient
}

func NewUpdateRepoMemberRole(repo RepoRepository, membership MembershipRepository, opa OPAClient) *UpdateRepoMemberRole {
	return &UpdateRepoMemberRole{repo: repo, membership: membership, opa: opa}
}

func (uc *UpdateRepoMemberRole) Execute(ctx context.Context, in UpdateRepoMemberRoleInput) (domain.RepoMember, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.RepoMember{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if !in.Role.Valid() {
		return domain.RepoMember{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_INVALID_REPO_ROLE", "invalid repo role", nil)
	}

	existing, err := uc.repo.GetRepo(ctx, in.RepoID)
	if errors.Is(err, domain.ErrRepoNotFound) {
		return domain.RepoMember{}, apperrors.New(apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND", "repo not found", err)
	}
	if err != nil {
		return domain.RepoMember{}, apperrors.New(apperrors.KindInternal, "PROJECT_REPO_FETCH_FAILED", "failed to fetch repo", err)
	}

	if err := requireRepoAccess(ctx, uc.membership, uc.repo, uc.opa, existing.ProjectID, in.RepoID, repoActionAdminOnly); err != nil {
		return domain.RepoMember{}, err
	}

	member, err := uc.repo.UpdateRepoMemberRole(ctx, in.RepoID, in.UserID, in.Role)
	if err != nil {
		if errors.Is(err, domain.ErrRepoMembershipNotFound) {
			return domain.RepoMember{}, apperrors.New(apperrors.KindNotFound, "PROJECT_REPO_MEMBERSHIP_NOT_FOUND", "repo membership does not exist", err)
		}
		return domain.RepoMember{}, apperrors.New(apperrors.KindInternal, "PROJECT_UPDATE_REPO_MEMBER_ROLE_FAILED", "failed to update repo member role", err)
	}
	return member, nil
}
