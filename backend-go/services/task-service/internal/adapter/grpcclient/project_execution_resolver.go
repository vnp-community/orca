// This file implements usecase.ProjectExecutionResolver, mirroring
// git-gateway-service's ConnectionResolver
// (internal/adapter/grpcclient/resolver.go) exactly: project_id is passed
// through verbatim as infra-fleet-service's connection_id, per that file's
// confirmed convention (ResolveConnectionRequest has no separate
// project/worktree field, only connection_id).
//
// BUG-025 follow-up: that connectionId=projectID lookup ALWAYS misses.
// infra.connections.id is a server-generated UUID (usecase.CreateConnection
// mints one internally; the proto request has no caller-supplied-id field
// at all) — nothing in this codebase ever creates a row whose id equals a
// project id, so !resp.GetConnected() is the system-wide, zero-row norm,
// not a rare edge case (confirmed live: TASK_EXECUTE_NO_CONNECTION on every
// task execute attempt, on a project with a real, reachable, agent-
// connected dev server bound to its default repo). git-gateway-service's
// own ConnectionResolver hit this exact same problem for worktree-keyed
// connections (BUG-012/CR-PW-010/SOL-013, BUG-015/CR-PW-011/SOL-014) and
// was fixed with a fallback: on !Connected, resolve the resource's own
// dev-server binding directly and check live reachability via
// GetFleetHealth, instead of ever requiring an infra.connections row to
// exist. This mirrors that exact fallback, one level up (project's default
// repo, per WorktreeProvisioner.resolveRepoID's own "lowest position wins"
// convention — task-service has no per-task repo-selection concept, so
// this reuses the same default-repo rule rather than inventing a second
// one).
package grpcclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

// ProjectExecutionResolver implements usecase.ProjectExecutionResolver by
// calling infra-fleet-service's ResolveConnection RPC, falling back to a
// repo-dev-server-reachability check (see this file's doc comment) when
// that lookup reports not connected.
type ProjectExecutionResolver struct {
	client       infrafleetv1.InfraFleetServiceClient
	projects     projectv1.ProjectServiceClient
	reachability usecase.DevServerReachability
}

func NewProjectExecutionResolver(client infrafleetv1.InfraFleetServiceClient, projects projectv1.ProjectServiceClient, reachability usecase.DevServerReachability) *ProjectExecutionResolver {
	return &ProjectExecutionResolver{client: client, projects: projects, reachability: reachability}
}

// ResolveConnection asks infra-fleet-service which host owns projectID.
// Like git-gateway-service's worktreeID, task-service's projectID IS the
// infra-fleet-service connectionId — passed through verbatim, and echoed
// back as the connectionID on a successful resolve. worktreePath is
// resp.GetRepoPath() verbatim — the same field git-gateway-service's own
// ConnectionResolver (internal/adapter/grpcclient/resolver.go:82) reads for
// its ResolvedConnection.RepoPath, per that response message's own doc
// comment (proto/orca/infrafleet/v1/infrafleet.proto's
// ResolveConnectionResponse.repo_path: "Callers like git-gateway-service's
// RelayExecutor need repo_path alongside dev_server to know which path to
// operate on").
//
// On !Connected, falls back to resolveViaDefaultRepo (this file's doc
// comment) instead of failing outright.
func (p *ProjectExecutionResolver) ResolveConnection(ctx context.Context, tenantID, projectID string) (string, string, string, string, bool, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return "", "", "", "", false, err
	}
	resp, err := p.client.ResolveConnection(ctx, &infrafleetv1.ResolveConnectionRequest{ConnectionId: projectID})
	if err != nil {
		return "", "", "", "", false, fmt.Errorf("grpcclient: ResolveConnection(%q): %w", projectID, err)
	}
	if resp.GetConnected() {
		return projectID, resp.GetRepoPath(), resp.GetWorktreeId(), "", true, nil
	}
	return p.resolveViaDefaultRepo(ctx, projectID)
}

// resolveViaDefaultRepo is the fallback this file's header comment
// describes: resolve the project's default repo (lowest position, same
// convention WorktreeProvisioner.resolveRepoID already establishes), read
// its bound dev server, and treat the project as "connected" iff that dev
// server is currently reachable. worktreeID is deliberately left empty
// here — there is no infra.connections row backing this path, so there is
// no real worktree_id to echo (mirrors git-gateway-service's
// ConnectionResolver.ResolveConnection's own !Connected/reachable branch,
// which leaves ConnectionID/Mode zero-valued for the same reason).
// repoPath is the repo's own root (Repo.Url, which doubles as an absolute
// filesystem path — see project-service's domain.RepoInfo doc comment) —
// good enough for ExecuteTask's create-branch path (a brand new task has no
// worktree yet); a task REUSING an existing worktree gets its real path
// from WorktreeProvisioner.EnsureWorktree directly, not from this fallback
// (see execute_task.go's "reuse branch" handling) — an already-flagged,
// separate, smaller imprecision, not fixed here.
//
// devServerID is returned alongside connectionID="" on success — see
// usecase.ProjectExecutionResolver's doc comment for why callers need it:
// there is no infra.connections row here for infra-fleet-service's
// connectionId-keyed Relay RPC to key on, so a caller must relay via
// RelayByDevServer(devServerID) instead. Confirmed live:
// INFRA_RELAY_NO_CONNECTION on every task.execute dispatch reaching this
// fallback until callers started branching on this value.
func (p *ProjectExecutionResolver) resolveViaDefaultRepo(ctx context.Context, projectID string) (string, string, string, string, bool, error) {
	// project-service's ListRepos is membership-gated (requireProjectAccess)
	// and needs BOTH tenant AND acting-user identity forwarded as outbound
	// metadata, not just tenant — see ProjectContextResolver.GetProjectContext's
	// doc comment for the same requirement on a sibling project-service
	// call. withTenantMetadata (used for the infra-fleet-service calls in
	// this file) is deliberately tenant-only, scoped to that service — found
	// live: PROJECT_NO_USER, ListRepos rejecting every call from this
	// fallback until this was added (BUG-026 follow-up).
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return "", "", "", "", false, err
	}
	userID, _ := tenant.UserID(ctx) // absent -> project-service denies with PROJECT_NO_USER, a legitimate fail-closed outcome
	projectCtx := metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID, grpcmw.MetadataUserID, userID)

	resp, err := p.projects.ListRepos(projectCtx, &projectv1.ListReposRequest{ProjectId: projectID})
	if err != nil {
		return "", "", "", "", false, fmt.Errorf("grpcclient: ListRepos(%q): %w", projectID, err)
	}
	repos := resp.GetRepos()
	if len(repos) == 0 {
		return "", "", "", "", false, nil
	}
	repo := repos[0]
	devServerID := repo.GetDevServerId()
	if devServerID == "" {
		return "", "", "", "", false, nil
	}
	reachable, err := p.reachability.IsReachable(ctx, devServerID)
	if err != nil {
		return "", "", "", "", false, fmt.Errorf("grpcclient: IsReachable(%q): %w", devServerID, err)
	}
	if !reachable {
		return "", "", "", "", false, nil
	}
	return "", repo.GetUrl(), "", devServerID, true, nil
}
