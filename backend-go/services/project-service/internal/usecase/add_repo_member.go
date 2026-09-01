package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type AddRepoMemberInput struct {
	RepoID string
	UserID string
	Role   domain.RepoRole
}

// AddRepoMember grants a project member a functional role on one specific
// repo — additional to (never a replacement for) their project membership.
// Authorization: repo_admin_only — a project owner always passes (see
// requireRepoAccess), or an existing "admin" grant on this same repo.
// AddRepoMemberInput carries only a repo_id, so Execute resolves the repo's
// owning project via RepoRepository.GetRepo first, same pattern as
// UpdateRepo/RemoveRepo.
type AddRepoMember struct {
	repo       RepoRepository
	membership MembershipRepository
	opa        OPAClient
}

func NewAddRepoMember(repo RepoRepository, membership MembershipRepository, opa OPAClient) *AddRepoMember {
	return &AddRepoMember{repo: repo, membership: membership, opa: opa}
}

func (uc *AddRepoMember) Execute(ctx context.Context, in AddRepoMemberInput) (domain.RepoMember, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.RepoMember{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}

	existing, err := uc.repo.GetRepo(ctx, in.RepoID)
	if errors.Is(err, domain.ErrRepoNotFound) {
		return domain.RepoMember{}, apperrors.New(apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND", "repo not found", err)
	}
	if err != nil {
		return domain.RepoMember{}, apperrors.New(apperrors.KindInternal, "PROJECT_REPO_FETCH_FAILED", "failed to fetch repo", err)
	}

	member, err := domain.NewRepoMember(in.RepoID, in.UserID, in.Role)
	if err != nil {
		return domain.RepoMember{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_REPO_MEMBER_INVALID", err.Error(), err)
	}

	if err := requireRepoAccess(ctx, uc.membership, uc.repo, uc.opa, existing.ProjectID, in.RepoID, repoActionAdminOnly); err != nil {
		return domain.RepoMember{}, err
	}

	if err := uc.repo.AddRepoMember(ctx, member); err != nil {
		return domain.RepoMember{}, apperrors.New(apperrors.KindInternal, "PROJECT_ADD_REPO_MEMBER_FAILED", "failed to add repo member", err)
	}
	return member, nil
}
