package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type GetRepoInput struct {
	RepoID string
}

type GetRepoResult struct {
	Repo domain.Repo
	// DevServerID is this repo's own dev-server binding (domain.Repo.
	// DevServerID) — Phase 10 moved dev-server ownership from the project to
	// the repo, so this no longer needs a second lookup through the owning
	// project.
	DevServerID string
}

// GetRepo answers git-gateway-service's "does this repo exist, and which
// dev server does it live on" — the confirmed gap git-gateway-service's
// project_client.go GetRepo doc comment flags: ListRepos(project_id) was
// the only lookup RPC available before this, unusable when a caller (every
// worktree.* usecase) only has a repo_id.
//
// Authorization: tenant-scoped only, no per-user OPA/membership check —
// same trust boundary RecordWorktreeCreated/RecordWorktreeRemoved already
// established for this exact service-to-service edge. git-gateway-service
// is an internal caller that never forwards the acting user's identity
// (see its withTenantMetadata helper, tenant-only), and the original
// wscompat caller (api-gateway) already authorized the end user's access
// before ever reaching git-gateway-service — re-checking membership here
// would need a user identity this call doesn't carry.
type GetRepo struct {
	repos RepoRepository
}

func NewGetRepo(repos RepoRepository) *GetRepo {
	return &GetRepo{repos: repos}
}

func (uc *GetRepo) Execute(ctx context.Context, in GetRepoInput) (GetRepoResult, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return GetRepoResult{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if in.RepoID == "" {
		return GetRepoResult{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_REPO_ID_REQUIRED", "repo_id is required", nil)
	}

	repo, err := uc.repos.GetRepo(ctx, in.RepoID)
	if errors.Is(err, domain.ErrRepoNotFound) {
		return GetRepoResult{}, apperrors.New(apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND", "repo not found", err)
	}
	if err != nil {
		return GetRepoResult{}, apperrors.New(apperrors.KindInternal, "PROJECT_REPO_FETCH_FAILED", "failed to fetch repo", err)
	}

	return GetRepoResult{Repo: repo, DevServerID: repo.DevServerID}, nil
}
