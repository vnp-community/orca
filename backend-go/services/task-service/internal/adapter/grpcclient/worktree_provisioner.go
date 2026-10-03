package grpcclient

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// WorktreeProvisioner implements usecase.WorktreeProvisioner against
// git-gateway-service's CreateWorktree RPC — delegates the whole
// create+record saga rather than re-implementing it (SOL-TG-04).
//
// Resolved wiring detail (TASK-TG-04-02's "open wiring detail, flagged
// rather than guessed at, not to be guessed at"): CreateWorktreeRequest
// requires a repo_id (gitgateway.proto's CreateWorktreeRequest) that
// task.ProjectID alone cannot supply. Reading
// git-gateway-service/internal/usecase/create_worktree.go settles which of
// the task file's two options is correct:
//
//   - Option 1 (server-side project_id resolution inside CreateWorktree) is
//     NOT how the saga works: CreateWorktree.Execute resolves exclusively
//     via projects.GetRepo(in.RepoID) — in.ProjectID is accepted on the wire
//     but never read by that usecase today (see that file's own doc comment:
//     "no real caller ever sends project_id on this RPC").
//   - project-service's project→repo cardinality is 1:N, not 1:1
//     (project.repos has a project_id FK, a position ordering column, and
//     ReorderRepos/ListRepos operate over a project's whole repo list) — so
//     even a hypothetical server-side resolution couldn't pick a single repo
//     from project_id alone without a stated default.
//
// This adapter therefore implements Option 2: it resolves RepoID itself via
// project-service's ListRepos, taking the lowest-position entry —
// ListReposResponse's own doc comment states repos come back "ordered by
// position", the same ordering ReorderRepos/ListRepos already establish
// elsewhere. This is a real, explicit assumption (task-service has no
// per-task repo-selection concept yet, only a project_id), not a re-guess
// of the open question — revisit once a task can target a project's
// non-default repo.
type WorktreeProvisioner struct {
	git      gitgatewayv1.GitGatewayServiceClient
	projects projectv1.ProjectServiceClient
	// sources, when set, lets EnsureWorktree adopt a worktree already created
	// for the task's external issue — see worktree_issue_reuse.go.
	sources TaskSourceReader
}

func NewWorktreeProvisioner(git gitgatewayv1.GitGatewayServiceClient, projects projectv1.ProjectServiceClient) *WorktreeProvisioner {
	return &WorktreeProvisioner{git: git, projects: projects}
}

func (p *WorktreeProvisioner) EnsureWorktree(ctx context.Context, tenantID string, task domain.Task) (worktreeID, path string, err error) {
	if task.WorktreeID != "" {
		// reuse — spec's "IF task.worktreeId exists: use existing
		// worktree". BUG-028: this used to return an empty path, relying
		// on the caller (SimpleExecutor.Execute, via
		// ProjectExecutionResolver) to guess one — but that resolver only
		// ever knows about the project's REPO, never a specific worktree,
		// so its guess was always the repo's shared root, not this task's
		// own isolated directory. Live-confirmed: every task re-execution
		// ran the agent in the shared repo checkout, on whatever branch
		// happened to be checked out there (not the task's own branch).
		// Resolve the real path here instead, at the one place that
		// actually knows which worktree this is.
		path, err := p.resolveWorktreePath(ctx, task.WorktreeID)
		if err == nil {
			return task.WorktreeID, path, nil
		}
		if !isWorktreeGone(err) {
			return "", "", err // fail closed — a silent repo-root fallback here would just reintroduce BUG-028
		}
		// The worktree was removed (e.g. workspace cleanup) and the task still
		// points at it. Falling back to the repo root would reintroduce BUG-028,
		// but a worktree that no longer exists has nothing left to protect:
		// provision a fresh one below; Execute persists the new id.
		slog.WarnContext(ctx, "worktree_provisioner: task's worktree no longer exists, creating a new one",
			slog.String("task_id", task.ID), slog.String("worktree_id", task.WorktreeID))
	}

	if id, wtPath, ok := p.findIssueWorktree(ctx, tenantID, task); ok {
		return id, wtPath, nil
	}

	repoID, err := p.resolveRepoID(ctx, task.ProjectID)
	if err != nil {
		return "", "", err
	}

	origCtx := ctx
	ctx, err = withTenantMetadata(ctx)
	if err != nil {
		return "", "", err
	}
	// Who triggered the run, so project-service's worktree.created event can name
	// the actor and issue-status-sync can act with that person's own credential.
	if userID, ok := tenant.UserID(origCtx); ok && userID != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataUserID, userID)
	}
	req := &gitgatewayv1.CreateWorktreeRequest{
		ProjectId: task.ProjectID,
		RepoId:    repoID,
		Branch:    fmt.Sprintf("task/%s", task.ID),
		TaskId:    &task.ID,
	}
	// Record the issue on the worktree this task creates, so issue-status-sync
	// moves it to In Progress and a later "Start work" on the same issue finds
	// this worktree instead of forking another. Looked up on the original ctx:
	// withTenantMetadata above only decorates the outgoing call.
	if src, ok := p.sourceFor(origCtx, tenantID, task); ok {
		provider, ref := string(src.Provider), src.Ref
		req.LinkedIssueProvider, req.LinkedIssueRef = &provider, &ref
	}
	resp, err := p.git.CreateWorktree(ctx, req)
	if err != nil {
		return "", "", fmt.Errorf("worktree_provisioner: create worktree: %w", err)
	}
	return resp.GetWorktreeId(), resp.GetPath(), nil
}

// resolveRepoID picks the project's default repo — see this type's doc
// comment for why a lookup is needed here at all and why "lowest position"
// is the chosen default.
func (p *WorktreeProvisioner) resolveRepoID(ctx context.Context, projectID string) (string, error) {
	if projectID == "" {
		return "", fmt.Errorf("worktree_provisioner: task has no project_id, cannot resolve a repo to create a worktree against")
	}
	// project-service's ListRepos is membership-gated (requireProjectAccess)
	// and needs BOTH tenant AND acting-user identity forwarded as outbound
	// metadata — withTenantMetadata alone is NOT enough (its own doc
	// comment scopes it to infra-fleet-service calls only) — see
	// ProjectContextResolver.GetProjectContext's doc comment and
	// ProjectExecutionResolver.resolveViaDefaultRepo's identical fix
	// (BUG-026) for the same requirement on sibling project-service calls.
	// Found live: PROJECT_NO_USER, TASK_EXECUTE_WORKTREE_FAILED on every
	// first-time task execution until this was added.
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return "", err
	}
	userID, _ := tenant.UserID(ctx) // absent -> project-service denies with PROJECT_NO_USER, a legitimate fail-closed outcome
	projectCtx := metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID, grpcmw.MetadataUserID, userID)
	resp, err := p.projects.ListRepos(projectCtx, &projectv1.ListReposRequest{ProjectId: projectID})
	if err != nil {
		return "", fmt.Errorf("worktree_provisioner: list repos for project %q: %w", projectID, err)
	}
	repos := resp.GetRepos()
	if len(repos) == 0 {
		return "", fmt.Errorf("worktree_provisioner: project %q has no repos", projectID)
	}
	return repos[0].GetId(), nil
}

// resolveWorktreePath resolves an EXISTING worktree's real, isolated
// filesystem path via project-service's GetWorktree — the same RPC
// git-gateway-service's own ConnectionResolver.resolveLocal already calls
// for the identical purpose. See EnsureWorktree's reuse-branch doc comment
// (BUG-028) for why this must exist and why its error must propagate
// rather than degrade to a guessed path.
func (p *WorktreeProvisioner) resolveWorktreePath(ctx context.Context, worktreeID string) (string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return "", err
	}
	userID, _ := tenant.UserID(ctx) // absent -> project-service denies with PROJECT_NO_USER, a legitimate fail-closed outcome — see resolveRepoID's identical comment
	projectCtx := metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID, grpcmw.MetadataUserID, userID)
	resp, err := p.projects.GetWorktree(projectCtx, &projectv1.GetWorktreeRequest{WorktreeId: worktreeID})
	if err != nil {
		return "", fmt.Errorf("worktree_provisioner: get worktree %q: %w", worktreeID, err)
	}
	return resp.GetPath(), nil
}

// isWorktreeGone reports whether err is project-service saying the worktree
// does not exist (as opposed to being unreachable or forbidden, which must
// still fail closed).
func isWorktreeGone(err error) bool {
	var st interface{ GRPCStatus() *status.Status }
	if errors.As(err, &st) {
		return st.GRPCStatus().Code() == codes.NotFound
	}
	return false
}
