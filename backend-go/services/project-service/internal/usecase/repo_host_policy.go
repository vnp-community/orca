package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// CodeRepoHostRequired is returned when a repo would be stored with no dev
// server. Frontend matches it by substring (REPO_HOST_REQUIRED).
const CodeRepoHostRequired = "PROJECT_REPO_HOST_REQUIRED"

// RepoHostPolicy says whether a repo may exist without a dev server. Only
// deployments where git-gateway shares a filesystem with the users (not
// Docker) enable it (ORCA_ALLOW_LOCAL_REPOS); elsewhere an unbound repo
// can never be operated on and git-gateway would reject it later.
type RepoHostPolicy struct {
	AllowLocalRepos bool
}

// requireHost rejects an empty dev server id unless local repos are allowed.
func (p RepoHostPolicy) requireHost(devServerID string) error {
	if devServerID != "" || p.AllowLocalRepos {
		return nil
	}
	return apperrors.New(apperrors.KindInvalidArgument, CodeRepoHostRequired,
		"choose the dev server (host) where this repository lives; repositories without a host cannot be operated on by this deployment", nil)
}

// ProjectDirectory is the read side AddRepo needs: membership for
// authorization plus the project row for its dev server fallback.
type ProjectDirectory interface {
	MembershipRepository
	Get(ctx context.Context, tenantID, id string) (domain.Project, error)
}
