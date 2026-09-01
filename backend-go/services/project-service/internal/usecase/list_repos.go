package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type ListReposInput struct {
	ProjectID string
}

type ListRepos struct {
	repo       RepoRepository
	membership MembershipRepository
	opa        OPAClient
}

func NewListRepos(repo RepoRepository, membership MembershipRepository, opa OPAClient) *ListRepos {
	return &ListRepos{repo: repo, membership: membership, opa: opa}
}

// Execute requires any membership (owner/member/viewer) in the project, or
// global admin — project-service.md §9's "any membership" tier. An empty
// ProjectID means "every repo in my tenant" (wscompat's repo.list channel
// has no project filter at all — see channels_repo_ssh_status_workspace.go
// and the old TS backend's repo.ts, params: null) — there's no single
// project to check per-project membership against, so that path is gated
// on tenant membership alone, same reasoning as ListWorktreeLineage's doc
// comment.
//
// When ProjectID is set, the result is further filtered by the repo_members
// functional-role tier: an owner sees every repo in the project; a non-owner
// member sees only repos they hold an explicit repo_members grant on.
func (uc *ListRepos) Execute(ctx context.Context, in ListReposInput) ([]domain.Repo, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}

	if in.ProjectID == "" {
		repos, err := uc.repo.ListReposForTenant(ctx)
		if err != nil {
			return nil, apperrors.New(apperrors.KindInternal, "PROJECT_LIST_REPOS_FAILED", "failed to list repos", err)
		}
		return repos, nil
	}

	if err := requireProjectAccess(ctx, uc.membership, uc.opa, in.ProjectID, projectActionAnyMember); err != nil {
		return nil, err
	}

	repos, err := uc.repo.ListRepos(ctx, in.ProjectID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "PROJECT_LIST_REPOS_FAILED", "failed to list repos", err)
	}

	// Visibility filter, repo_members tier: a project owner sees every repo
	// in their project unconditionally (repo_members is opt-in — an owner's
	// access never depends on holding a grant). A non-owner member sees
	// only the repos they hold an explicit repo_members grant on — "a
	// developer sees/acts on repo X, a lead manages repo Y" per this
	// feature's own design intent, not every repo in the project.
	actorID, ok := tenant.UserID(ctx)
	if !ok {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_USER", "no user in request context", nil)
	}
	m, err := uc.membership.GetMembership(ctx, in.ProjectID, actorID)
	if err != nil && !errors.Is(err, domain.ErrMembershipNotFound) {
		return nil, apperrors.New(apperrors.KindInternal, "PROJECT_MEMBERSHIP_LOOKUP_FAILED", "failed to resolve caller's project membership", err)
	}
	if err == nil && m.Role == domain.ProjectRoleOwner {
		return repos, nil
	}

	visibleIDs, err := uc.repo.ListRepoIDsWithMembership(ctx, in.ProjectID, actorID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "PROJECT_LIST_REPOS_FAILED", "failed to resolve caller's repo-level visibility", err)
	}
	visible := make(map[string]struct{}, len(visibleIDs))
	for _, id := range visibleIDs {
		visible[id] = struct{}{}
	}
	filtered := make([]domain.Repo, 0, len(repos))
	for _, r := range repos {
		if _, ok := visible[r.ID]; ok {
			filtered = append(filtered, r)
		}
	}
	return filtered, nil
}
