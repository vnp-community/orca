package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type ReorderReposInput struct {
	ProjectID      string
	RepoIDsInOrder []string
}

// ReorderRepos rewrites a project's repo display order in one transaction.
// The input must be the FULL ordered id list — a partial or mismatched list
// (missing an existing repo, containing an unknown id, or a duplicate) is
// rejected with KindInvalidArgument before any write happens, rather than
// silently reordering a subset.
//
// Authorization stays project-level owner-only (projectActionOwnerOnly),
// NOT the repo-scoped repo_admin_only/repo_lead_or_admin tier — deliberately:
// this rewrites EVERY repo's position in the project in one call, so there
// is no single repo to resolve a repo_members grant against (a caller could
// hold "lead" on one repo in the list and nothing on another). Reordering
// the whole catalog is a project-wide action, same as AddRepo.
type ReorderRepos struct {
	repo       RepoRepository
	membership MembershipRepository
	opa        OPAClient
}

func NewReorderRepos(repo RepoRepository, membership MembershipRepository, opa OPAClient) *ReorderRepos {
	return &ReorderRepos{repo: repo, membership: membership, opa: opa}
}

func (uc *ReorderRepos) Execute(ctx context.Context, in ReorderReposInput) error {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if err := requireProjectAccess(ctx, uc.membership, uc.opa, in.ProjectID, projectActionOwnerOnly); err != nil {
		return err
	}

	existing, err := uc.repo.ListRepos(ctx, in.ProjectID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "PROJECT_LIST_REPOS_FAILED", "failed to list repos", err)
	}

	if !sameIDSet(existing, in.RepoIDsInOrder) {
		return apperrors.New(apperrors.KindInvalidArgument, "PROJECT_REORDER_REPOS_MISMATCH", "repo_ids_in_order must be an exact permutation of the project's existing repo ids", nil)
	}

	if err := uc.repo.ReorderRepos(ctx, in.ProjectID, in.RepoIDsInOrder); err != nil {
		return apperrors.New(apperrors.KindInternal, "PROJECT_REORDER_REPOS_FAILED", "failed to reorder repos", err)
	}
	return nil
}

// sameIDSet reports whether ids is an exact permutation of existing's ids —
// same length, same members, no duplicates, no unknown/missing ids.
func sameIDSet(existing []domain.Repo, ids []string) bool {
	if len(existing) != len(ids) {
		return false
	}
	want := make(map[string]struct{}, len(existing))
	for _, r := range existing {
		want[r.ID] = struct{}{}
	}
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := want[id]; !ok {
			return false
		}
		if _, dup := seen[id]; dup {
			return false
		}
		seen[id] = struct{}{}
	}
	return true
}
