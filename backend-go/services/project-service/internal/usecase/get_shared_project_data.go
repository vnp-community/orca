package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type GetSharedProjectDataInput struct {
	ContainerProjectID string
	SourceProjectID    string
}

type GetSharedProjectDataResult struct {
	Project   domain.Project
	Repos     []domain.Repo
	Worktrees []domain.Worktree
}

// GetSharedProjectData is the cross-project read: sourceProjectID's
// project/repos/worktrees, but ONLY if it's actually linked into
// containerProjectID — legacy TS reference's orcaProjects.getProjectData,
// its module doc comment marked SECURITY-CRITICAL. Unlike the TS
// implementation (which filters one Project + its repos/worktree-meta out
// of an owner's single monolithic per-user JSON blob), this service's data
// is already scoped per-project by the DB schema itself — ListRepos/
// ListWorktrees(sourceProjectID) can never return another project's rows,
// so there's no blob-filtering invariant to maintain here.
//
// Enumeration resistance (matching legacy's ACCESS_DENIED_MESSAGE): a
// caller who isn't a member of containerProjectID and a caller who IS a
// member but passes a sourceProjectID that isn't linked both get the
// identical PROJECT_NOT_AUTHORIZED error — never a distinguishing
// "not linked" message that would let a member probe which project ids
// exist elsewhere in the tenant.
type GetSharedProjectData struct {
	projects       ProjectRepository
	repos          RepoRepository
	worktrees      WorktreeRepository
	sourceProjects SourceProjectRepository
	membership     MembershipRepository
	opa            OPAClient
}

func NewGetSharedProjectData(
	projects ProjectRepository,
	repos RepoRepository,
	worktrees WorktreeRepository,
	sourceProjects SourceProjectRepository,
	membership MembershipRepository,
	opa OPAClient,
) *GetSharedProjectData {
	return &GetSharedProjectData{
		projects:       projects,
		repos:          repos,
		worktrees:      worktrees,
		sourceProjects: sourceProjects,
		membership:     membership,
		opa:            opa,
	}
}

func (uc *GetSharedProjectData) Execute(ctx context.Context, in GetSharedProjectDataInput) (GetSharedProjectDataResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return GetSharedProjectDataResult{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}

	if err := requireProjectAccess(ctx, uc.membership, uc.opa, in.ContainerProjectID, projectActionAnyMember); err != nil {
		return GetSharedProjectDataResult{}, err
	}

	notAuthorized := apperrors.New(apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED", "caller is not authorized for this action", nil)

	if _, err := uc.sourceProjects.Get(ctx, in.ContainerProjectID, in.SourceProjectID); err != nil {
		if errors.Is(err, domain.ErrSourceProjectNotFound) {
			// Deliberately the SAME error requireProjectAccess returns for
			// "not a member" — see doc comment above.
			return GetSharedProjectDataResult{}, notAuthorized
		}
		return GetSharedProjectDataResult{}, apperrors.New(apperrors.KindInternal, "PROJECT_GET_SOURCE_PROJECT_FAILED", "failed to look up source project link", err)
	}

	project, err := uc.projects.Get(ctx, tenantID, in.SourceProjectID)
	if err != nil {
		if errors.Is(err, domain.ErrProjectNotFound) {
			// The link row referenced a project that no longer exists
			// (e.g. deleted after linking) — same not-authorized answer,
			// never a distinguishing "project gone" message.
			return GetSharedProjectDataResult{}, notAuthorized
		}
		return GetSharedProjectDataResult{}, apperrors.New(apperrors.KindInternal, "PROJECT_FETCH_FAILED", "failed to fetch source project", err)
	}

	repos, err := uc.repos.ListRepos(ctx, in.SourceProjectID)
	if err != nil {
		return GetSharedProjectDataResult{}, apperrors.New(apperrors.KindInternal, "PROJECT_LIST_REPOS_FAILED", "failed to list source project's repos", err)
	}

	worktrees, err := uc.worktrees.ListWorktrees(ctx, in.SourceProjectID)
	if err != nil {
		return GetSharedProjectDataResult{}, apperrors.New(apperrors.KindInternal, "PROJECT_LIST_WORKTREES_FAILED", "failed to list source project's worktrees", err)
	}

	return GetSharedProjectDataResult{Project: project, Repos: repos, Worktrees: worktrees}, nil
}
