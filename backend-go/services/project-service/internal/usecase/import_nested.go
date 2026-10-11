package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type ImportNestedInput struct {
	// DevServerID is the dev server the candidates were scanned on
	// (project.proto's ImportNestedRequest.dev_server_id) — stamped onto
	// each created project's DevServerID, matching what ScanNested reported
	// them found under.
	DevServerID   string
	ParentGroupID string
	Selected      []domain.NestedRepoCandidate
}

// ImportNested needs no relay call — once the user has selected candidates,
// materializing them into project_groups (+ a Project/Repo per group) is
// pure metadata writes, one DB transaction (see
// ProjectGroupRepository.ImportNested's doc comment).
type ImportNested struct {
	groupRepo ProjectGroupRepository
	policy    RepoHostPolicy
	guard     repoLocationGuard
}

func NewImportNested(groupRepo ProjectGroupRepository, repos RepoRepository, members MembershipRepository, policy RepoHostPolicy) *ImportNested {
	return &ImportNested{groupRepo: groupRepo, policy: policy, guard: repoLocationGuard{repos: repos, members: members}}
}

func (uc *ImportNested) Execute(ctx context.Context, in ImportNestedInput) ([]domain.ProjectGroup, []domain.Project, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, nil, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return nil, nil, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_USER", "no user in request context", nil)
	}

	if len(in.Selected) > 0 {
		if err := uc.policy.requireHost(in.DevServerID); err != nil {
			return nil, nil, err
		}
	}

	// All-or-nothing import: refuse the whole selection if any candidate is
	// already registered on this host, or listed twice.
	seen := make(map[string]struct{}, len(in.Selected))
	for _, c := range in.Selected {
		loc := domain.NormalizeRepoLocation(c.Path)
		if _, dup := seen[loc]; dup {
			return nil, nil, apperrors.New(apperrors.KindInvalidArgument, CodeImportNestedDuplicatePath,
				"the same folder is selected more than once: "+c.Path, nil)
		}
		seen[loc] = struct{}{}
		if err := uc.guard.ensureFree(ctx, tenantID, in.DevServerID, c.Path, ""); err != nil {
			return nil, nil, err
		}
	}

	if in.ParentGroupID != "" {
		if _, err := uc.groupRepo.GetProjectGroup(ctx, tenantID, in.ParentGroupID); err != nil {
			if errors.Is(err, domain.ErrProjectGroupNotFound) {
				return nil, nil, apperrors.New(apperrors.KindNotFound, "PROJECT_GROUP_NOT_FOUND", "parent group does not exist", err)
			}
			return nil, nil, apperrors.New(apperrors.KindInternal, "PROJECT_GROUP_FETCH_FAILED", "failed to fetch parent group", err)
		}
	}

	groups, projects, err := uc.groupRepo.ImportNested(ctx, tenantID, userID, in.DevServerID, in.ParentGroupID, in.Selected)
	if errors.Is(err, domain.ErrRepoAlreadyRegistered) {
		// Lost a race on one candidate: name it by re-running the pre-check.
		for _, c := range in.Selected {
			if guardErr := uc.guard.ensureFree(ctx, tenantID, in.DevServerID, c.Path, ""); guardErr != nil {
				return nil, nil, guardErr
			}
		}
		return nil, nil, apperrors.New(apperrors.KindAborted, CodeRepoRegistrationConflict, "a repository in this selection was registered by a concurrent request; retry", err)
	}
	if err != nil {
		return nil, nil, apperrors.New(apperrors.KindInternal, "PROJECT_IMPORT_NESTED_FAILED", "failed to import nested repos", err)
	}
	return groups, projects, nil
}
