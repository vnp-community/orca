package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type ListRepoMembersInput struct {
	RepoID string
}

// ListRepoMembers requires repo_any_functional_role — a project owner, or a
// caller holding any repo_members grant (developer/lead/admin) on this
// specific repo. Mirrors ListMembers' any-membership tier one level down.
type ListRepoMembers struct {
	repo       RepoRepository
	membership MembershipRepository
	opa        OPAClient
}

func NewListRepoMembers(repo RepoRepository, membership MembershipRepository, opa OPAClient) *ListRepoMembers {
	return &ListRepoMembers{repo: repo, membership: membership, opa: opa}
}

func (uc *ListRepoMembers) Execute(ctx context.Context, in ListRepoMembersInput) ([]domain.RepoMember, error) {
	existing, err := uc.repo.GetRepo(ctx, in.RepoID)
	if errors.Is(err, domain.ErrRepoNotFound) {
		return nil, apperrors.New(apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND", "repo not found", err)
	}
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "PROJECT_REPO_FETCH_FAILED", "failed to fetch repo", err)
	}

	if err := requireRepoAccess(ctx, uc.membership, uc.repo, uc.opa, existing.ProjectID, in.RepoID, repoActionAnyFunctionalRole); err != nil {
		return nil, err
	}

	members, err := uc.repo.ListRepoMembers(ctx, in.RepoID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "PROJECT_LIST_REPO_MEMBERS_FAILED", "failed to list repo members", err)
	}
	return members, nil
}
