package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type RemoveRepoMemberInput struct {
	RepoID string
	UserID string
}

// RemoveRepoMember revokes a functional-role grant — same repo_admin_only
// tier as AddRepoMember. Unlike RemoveMember's project-level "≥1 owner"
// invariant, there's no equivalent floor here: a repo with zero repo_members
// rows is the normal, common state (repo_members is opt-in — a project
// owner's access never depends on holding one), so removing the last grant
// is never itself a problem.
type RemoveRepoMember struct {
	repo       RepoRepository
	membership MembershipRepository
	opa        OPAClient
}

func NewRemoveRepoMember(repo RepoRepository, membership MembershipRepository, opa OPAClient) *RemoveRepoMember {
	return &RemoveRepoMember{repo: repo, membership: membership, opa: opa}
}

func (uc *RemoveRepoMember) Execute(ctx context.Context, in RemoveRepoMemberInput) error {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}

	existing, err := uc.repo.GetRepo(ctx, in.RepoID)
	if errors.Is(err, domain.ErrRepoNotFound) {
		return apperrors.New(apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND", "repo not found", err)
	}
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "PROJECT_REPO_FETCH_FAILED", "failed to fetch repo", err)
	}

	if err := requireRepoAccess(ctx, uc.membership, uc.repo, uc.opa, existing.ProjectID, in.RepoID, repoActionAdminOnly); err != nil {
		return err
	}

	if err := uc.repo.RemoveRepoMember(ctx, in.RepoID, in.UserID); err != nil {
		if errors.Is(err, domain.ErrRepoMembershipNotFound) {
			return apperrors.New(apperrors.KindNotFound, "PROJECT_REPO_MEMBERSHIP_NOT_FOUND", "repo membership does not exist", err)
		}
		return apperrors.New(apperrors.KindInternal, "PROJECT_REMOVE_REPO_MEMBER_FAILED", "failed to remove repo member", err)
	}
	return nil
}
