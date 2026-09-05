package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type RebindRepoDevServerInput struct {
	RepoID         string
	NewDevServerID string
}

// RebindRepoDevServer is RebindDevServer's repo-scoped counterpart (Phase
// 10: dev_server_id moved from project.projects to project.repos, one per
// repo instead of one per project). The active-execution guard stays at the
// whole-project level for this phase — rebinding any one repo blocks on ANY
// active execution anywhere in the project, same conservative behavior as
// today's project-level rebind, rather than trying to scope the guard down
// to just the affected repo's own executions.
type RebindRepoDevServer struct {
	repos           RepoRepository
	membership      MembershipRepository
	opa             OPAClient
	workflowChecker WorkflowExecutionChecker
	taskChecker     TaskExecutionChecker
	devServers      DevServerLister
}

func NewRebindRepoDevServer(repos RepoRepository, membership MembershipRepository, opa OPAClient, workflowChecker WorkflowExecutionChecker, taskChecker TaskExecutionChecker, devServers DevServerLister) *RebindRepoDevServer {
	return &RebindRepoDevServer{
		repos:           repos,
		membership:      membership,
		opa:             opa,
		workflowChecker: workflowChecker,
		taskChecker:     taskChecker,
		devServers:      devServers,
	}
}

// Execute requires the caller's project role to be owner, or global admin —
// same owner-only tier as RebindDevServer.Execute, resolved via the repo's
// owning project since RebindRepoDevServerInput carries only a repo_id.
func (uc *RebindRepoDevServer) Execute(ctx context.Context, in RebindRepoDevServerInput) (domain.Repo, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Repo{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}

	existing, err := uc.repos.GetRepo(ctx, in.RepoID)
	if errors.Is(err, domain.ErrRepoNotFound) {
		return domain.Repo{}, apperrors.New(apperrors.KindNotFound, "PROJECT_REPO_NOT_FOUND", "repo not found", err)
	}
	if err != nil {
		return domain.Repo{}, apperrors.New(apperrors.KindInternal, "PROJECT_REPO_FETCH_FAILED", "failed to fetch repo", err)
	}

	if err := requireProjectAccess(ctx, uc.membership, uc.opa, existing.ProjectID, projectActionOwnerOnly); err != nil {
		return domain.Repo{}, err
	}

	// Both checks are synchronous gRPC calls per project-service.md §3 (saga
	// pattern) — the caller cannot know a rebind is safe without both
	// answers. A checker error fails closed (treated as "has active
	// executions") per §3/§8's timeout policy, rather than letting a
	// transient outbound failure silently allow the rebind through.
	workflowActive, err := uc.workflowChecker.HasActiveExecutions(ctx, existing.ProjectID)
	if err != nil {
		return domain.Repo{}, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_HAS_ACTIVE_WORKFLOWS", "failed to verify workflow executions, failing closed", err)
	}
	taskActive, err := uc.taskChecker.HasActiveExecutions(ctx, existing.ProjectID)
	if err != nil {
		return domain.Repo{}, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_HAS_ACTIVE_WORKFLOWS", "failed to verify task executions, failing closed", err)
	}
	if workflowActive || taskActive {
		return domain.Repo{}, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_HAS_ACTIVE_WORKFLOWS", "project has active workflow or task executions", nil)
	}

	if in.NewDevServerID != "" {
		exists, err := uc.devServers.Exists(ctx, tenantID, in.NewDevServerID)
		if err != nil {
			return domain.Repo{}, apperrors.New(apperrors.KindInternal, "PROJECT_DEV_SERVER_LOOKUP_FAILED", "failed to validate dev server", err)
		}
		if !exists {
			return domain.Repo{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_DEV_SERVER_NOT_FOUND", "dev server does not exist", nil)
		}
	}

	updated, err := uc.repos.UpdateDevServerID(ctx, in.RepoID, in.NewDevServerID)
	if err != nil {
		return domain.Repo{}, apperrors.New(apperrors.KindInternal, "PROJECT_REBIND_FAILED", "failed to update dev server binding", err)
	}
	return updated, nil
}
