package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type AddRepoInput struct {
	ProjectID   string
	URL         string
	DisplayName string
}

// AddRepo appends a new repo to a project's catalog, at the next available
// position — see RepoRepository.AddRepo's doc comment.
//
// Authorization is a judgment call: AddRepo isn't named in
// project-service.md §9's matrix, but a repo belongs to exactly one
// project, so the project's owner-or-admin rule (the matrix's owner-only
// tier) is the natural fit for mutating that project's repo catalog. See
// this service's README "Known gaps" for the RPCs this same judgment call
// was NOT extended to.
type AddRepo struct {
	repo       RepoRepository
	membership MembershipRepository
	opa        OPAClient
}

func NewAddRepo(repo RepoRepository, membership MembershipRepository, opa OPAClient) *AddRepo {
	return &AddRepo{repo: repo, membership: membership, opa: opa}
}

func (uc *AddRepo) Execute(ctx context.Context, in AddRepoInput) (domain.Repo, error) {
	// Every usecase requires authenticated tenant context uniformly, even
	// though the repo's tenant scoping is transitive through its project_id
	// FK rather than a tenant_id column of its own — same convention as
	// AddMember.
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.Repo{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if err := requireProjectAccess(ctx, uc.membership, uc.opa, in.ProjectID, projectActionOwnerOnly); err != nil {
		return domain.Repo{}, err
	}

	r, err := domain.NewRepo(uuid.NewString(), in.ProjectID, in.URL, in.DisplayName)
	if err != nil {
		return domain.Repo{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_REPO_INVALID", err.Error(), err)
	}

	created, err := uc.repo.AddRepo(ctx, r)
	if err != nil {
		return domain.Repo{}, apperrors.New(apperrors.KindInternal, "PROJECT_ADD_REPO_FAILED", "failed to persist repo", err)
	}
	return created, nil
}
