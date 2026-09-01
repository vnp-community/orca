// This file holds the repo.*/ssh.*/status.get/workspacePorts.* channel
// registrations added for SOL-023/SOL-024/SOL-025/SOL-027
// (specs/backend-go/bugs/missing-v1). Deliberately kept OUT of channels.go —
// another group is concurrently extending that file's registerGitChannels/
// RegisterRealChannels for git.*/files.* work, and merging two sets of
// hand-edits to the same function bodies is exactly the kind of conflict a
// separate file avoids. See this file's bottom doc comment for the
// one-line wiring RegisterRealChannels/main.go still need once this file's
// registerRepoSshStatusWorkspaceChannels is folded in.
//
// workspace.refreshFileTree (TASK-168/SOL-026) is NOT implemented here — it
// wraps a files.* RPC that does not exist yet in this worktree (see
// specs/backend-go/bugs/missing-v1/tasks/TASK-168-wire-workspace-refresh-file-tree.md,
// marked [blocked] in this pass). Wiring it before files.* lands would mean
// inventing a throwaway parallel directory-listing RPC — do not do that.
package wscompat

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// repoSSHStatusWorkspaceRPCTimeout mirrors the rpcTimeout convention
// TASK-151/154/160/165/167/169/172's design already assumes exists in
// channels.go (it doesn't yet, in this worktree's current state) — named
// distinctly here so this file compiles standalone and doesn't collide with
// whatever name the integration pass's merge settles on.
const repoSSHStatusWorkspaceRPCTimeout = 8 * time.Second

// registerRepoSshStatusWorkspaceChannels wires every repo.*/ssh.*/
// status.get/workspacePorts.* channel this pass gives real backend-go
// implementations to. NOT called from RegisterRealChannels yet — see this
// file's bottom doc comment for the one-line wiring the integration pass
// needs to fold this in.
func registerRepoSshStatusWorkspaceChannels(
	r *Registry,
	project projectv1.ProjectServiceClient,
	git gitgatewayv1.GitGatewayServiceClient,
	infraFleet infrafleetv1.InfraFleetServiceClient,
) {
	registerRepoChannels(r, project, git)
	registerSshChannels(r, infraFleet)
	registerStatusChannels(r)
	registerWorkspacePortsChannels(r, infraFleet)
}

// repoView/toRepoView: same camelCase-view fix as channels_tenant_project.go
// (see that file's projectView doc comment for the full reasoning) —
// projectv1.Repo's project_id is snake_case on the wire via plain
// encoding/json, but shared/types.ts's Repo needs projectId (4b-4 threads
// it through so per-repo actions know their owning project without a
// second lookup).
type repoView struct {
	ID          string `json:"id"`
	ProjectID   string `json:"projectId"`
	URL         string `json:"url"`
	DisplayName string `json:"displayName"`
	Position    int32  `json:"position"`
}

func toRepoView(r *projectv1.Repo) repoView {
	return repoView{
		ID: r.GetId(), ProjectID: r.GetProjectId(), URL: r.GetUrl(),
		DisplayName: r.GetDisplayName(), Position: r.GetPosition(),
	}
}

// cloneResultView/initRepoResultView: git-gateway-service's CloneResponse/
// InitRepoResponse have their own default_branch snake_case field — same
// bug, same fix, one level removed (a different backend service's proto
// package, not project-service's).
type cloneResultView struct {
	WorktreePath  string `json:"worktreePath"`
	DefaultBranch string `json:"defaultBranch"`
}

type initRepoResultView struct {
	Path          string `json:"path"`
	DefaultBranch string `json:"defaultBranch"`
}

// ── repo.* ───────────────────────────────────────────────────────────────
//
// repo.add/list/reorder/rm/update map 1:1 onto ProjectService's
// AddRepo/ListRepos/ReorderRepos/RemoveRepo/UpdateRepo — pure catalog CRUD
// against the repos Postgres table, per project-service.md §4's "Repo —
// pure metadata, no working-tree state." The remaining 8 (clone,
// baseRefDefault, searchRefs, create, hooksCheck, issueCommandRead,
// issueCommandWrite, setupScriptImports) are file/host operations against a
// repo's working tree, so they dispatch to git-gateway-service instead —
// see SOL-023 Bucket 3.
func registerRepoChannels(r *Registry, project projectv1.ProjectServiceClient, git gitgatewayv1.GitGatewayServiceClient) {
	r.Register("repo.add", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type addArgs struct {
			ProjectID   string `json:"projectId"`
			URL         string `json:"url"`
			DisplayName string `json:"displayName"`
		}
		in, err := decodeArg[addArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, repoSSHStatusWorkspaceRPCTimeout)
		defer cancel()
		resp, err := project.AddRepo(rpcCtx, &projectv1.AddRepoRequest{
			ProjectId: in.ProjectID, Url: in.URL, DisplayName: in.DisplayName,
		})
		if err != nil {
			return nil, err
		}
		return toRepoView(resp.GetRepo()), nil
	})

	r.Register("repo.list", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listArgs struct {
			ProjectID string `json:"projectId"`
		}
		in, err := decodeArg[listArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, repoSSHStatusWorkspaceRPCTimeout)
		defer cancel()
		resp, err := project.ListRepos(rpcCtx, &projectv1.ListReposRequest{ProjectId: in.ProjectID})
		if err != nil {
			return nil, err
		}
		// Wrapped in {repos: [...]}, NOT a bare array — both call sites
		// (frontend/src/renderer/src/web/web-preload-api.ts's repos.list AND
		// its repo.list-for-a-runtime-environment path) do
		// `(await callRuntimeResult<{ repos: Repo[] }>('repo.list')).repos`,
		// matching the old TS backend's repo.ts handler:
		// `return { repos: runtime.listRepos() }`. A bare array has no
		// `.repos` property, so that destructure silently produced
		// `undefined` — surfaced as "[repos] repo.list returned a non-array
		// payload ... undefined" once the PROJECT_MEMBERSHIP_LOOKUP_FAILED
		// bug (fixed separately) stopped masking it.
		repos := resp.GetRepos()
		views := make([]repoView, 0, len(repos))
		for _, repo := range repos {
			views = append(views, toRepoView(repo))
		}
		return map[string]any{"repos": views}, nil
	})

	r.Register("repo.reorder", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type reorderArgs struct {
			ProjectID      string   `json:"projectId"`
			RepoIDsInOrder []string `json:"repoIdsInOrder"`
		}
		in, err := decodeArg[reorderArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, repoSSHStatusWorkspaceRPCTimeout)
		defer cancel()
		_, err = project.ReorderRepos(rpcCtx, &projectv1.ReorderReposRequest{
			ProjectId: in.ProjectID, RepoIdsInOrder: in.RepoIDsInOrder,
		})
		return nil, err
	})

	r.Register("repo.rm", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type rmArgs struct {
			RepoID string `json:"repoId"`
		}
		in, err := decodeArg[rmArgs](args, 0)
		if err != nil {
			return nil, err
		}
		// NOTE: RemoveRepo does NOT detach worktrees first —
		// project-service.md §3's comment on RemoveRepoRequest ("caller
		// must detach worktrees via git-gateway-service first") means this
		// handler, or the frontend action that calls it, is responsible
		// for that ordering. Flag as a follow-up if the frontend doesn't
		// already enforce it.
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, repoSSHStatusWorkspaceRPCTimeout)
		defer cancel()
		_, err = project.RemoveRepo(rpcCtx, &projectv1.RemoveRepoRequest{RepoId: in.RepoID})
		return nil, err
	})

	r.Register("repo.update", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			RepoID      string `json:"repoId"`
			URL         string `json:"url"`
			DisplayName string `json:"displayName"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, repoSSHStatusWorkspaceRPCTimeout)
		defer cancel()
		resp, err := project.UpdateRepo(rpcCtx, &projectv1.UpdateRepoRequest{
			RepoId: in.RepoID, Url: in.URL, DisplayName: in.DisplayName,
		})
		if err != nil {
			return nil, err
		}
		return toRepoView(resp.GetRepo()), nil
	})

	// repo.getMembers/addMember/removeMember/updateMemberRole map 1:1 onto
	// ProjectService's ListRepoMembers/AddRepoMember/RemoveRepoMember/
	// UpdateRepoMemberRole — the repo-scoped functional-role tier
	// (developer/lead/admin), layered on top of project.getMembers/
	// addMember/removeMember/updateMemberRole's project-level owner/member
	// tier (channels_tenant_project.go). See policy/orca-authz/repo.rego.
	r.Register("repo.getMembers", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type getArgs struct {
			RepoID string `json:"repoId"`
		}
		in, err := decodeArg[getArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, repoSSHStatusWorkspaceRPCTimeout)
		defer cancel()
		resp, err := project.ListRepoMembers(rpcCtx, &projectv1.ListRepoMembersRequest{RepoId: in.RepoID})
		if err != nil {
			return nil, err
		}
		return resp.GetMembers(), nil
	})

	r.Register("repo.addMember", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type addArgs struct {
			RepoID string `json:"repoId"`
			UserID string `json:"userId"`
			Role   string `json:"role"`
		}
		in, err := decodeArg[addArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, repoSSHStatusWorkspaceRPCTimeout)
		defer cancel()
		resp, err := project.AddRepoMember(rpcCtx, &projectv1.AddRepoMemberRequest{
			RepoId: in.RepoID, UserId: in.UserID, Role: toRepoRoleArg(in.Role),
		})
		if err != nil {
			return nil, err
		}
		return resp.GetMember(), nil
	})

	r.Register("repo.removeMember", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type removeArgs struct {
			RepoID string `json:"repoId"`
			UserID string `json:"userId"`
		}
		in, err := decodeArg[removeArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, repoSSHStatusWorkspaceRPCTimeout)
		defer cancel()
		if _, err := project.RemoveRepoMember(rpcCtx, &projectv1.RemoveRepoMemberRequest{
			RepoId: in.RepoID, UserId: in.UserID,
		}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})

	r.Register("repo.updateMemberRole", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			RepoID string `json:"repoId"`
			UserID string `json:"userId"`
			Role   string `json:"role"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, repoSSHStatusWorkspaceRPCTimeout)
		defer cancel()
		resp, err := project.UpdateRepoMemberRole(rpcCtx, &projectv1.UpdateRepoMemberRoleRequest{
			RepoId: in.RepoID, UserId: in.UserID, Role: toRepoRoleArg(in.Role),
		})
		if err != nil {
			return nil, err
		}
		return resp.GetMember(), nil
	})

	r.Register("repo.clone", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type cloneArgs struct {
			DevServerID string `json:"devServerId"`
			URL         string `json:"url"`
			DestPath    string `json:"destPath"`
		}
		in, err := decodeArg[cloneArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		// Longer-than-default deadline: cloning a real repo, especially
		// relayed to a remote host, can legitimately exceed the default 8s.
		// Same reasoning as ssh.connect's 20s override (SOL-024).
		rpcCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		resp, err := git.Clone(rpcCtx, &gitgatewayv1.CloneRequest{
			DevServerId: in.DevServerID, Url: in.URL, DestPath: in.DestPath,
		})
		if err != nil {
			return nil, err
		}
		return cloneResultView{WorktreePath: resp.GetWorktreePath(), DefaultBranch: resp.GetDefaultBranch()}, nil
	})

	r.Register("repo.baseRefDefault", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type baseRefDefaultArgs struct {
			WorktreeID string `json:"worktreeId"`
		}
		in, err := decodeArg[baseRefDefaultArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, repoSSHStatusWorkspaceRPCTimeout)
		defer cancel()
		resp, err := git.BaseRefDefault(rpcCtx, &gitgatewayv1.BaseRefDefaultRequest{WorktreeId: in.WorktreeID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("repo.searchRefs", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type searchRefsArgs struct {
			WorktreeID string `json:"worktreeId"`
			Query      string `json:"query"`
		}
		in, err := decodeArg[searchRefsArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, repoSSHStatusWorkspaceRPCTimeout)
		defer cancel()
		resp, err := git.SearchRefs(rpcCtx, &gitgatewayv1.SearchRefsRequest{WorktreeId: in.WorktreeID, Query: in.Query})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("repo.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			DevServerID   string `json:"devServerId"`
			DestPath      string `json:"destPath"`
			DefaultBranch string `json:"defaultBranch"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, repoSSHStatusWorkspaceRPCTimeout)
		defer cancel()
		resp, err := git.InitRepo(rpcCtx, &gitgatewayv1.InitRepoRequest{
			DevServerId: in.DevServerID, DestPath: in.DestPath, DefaultBranch: in.DefaultBranch,
		})
		if err != nil {
			return nil, err
		}
		return initRepoResultView{Path: resp.GetPath(), DefaultBranch: resp.GetDefaultBranch()}, nil
	})

	r.Register("repo.hooksCheck", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type hooksCheckArgs struct {
			WorktreeID string `json:"worktreeId"`
		}
		in, err := decodeArg[hooksCheckArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, repoSSHStatusWorkspaceRPCTimeout)
		defer cancel()
		resp, err := git.CheckHooks(rpcCtx, &gitgatewayv1.CheckHooksRequest{WorktreeId: in.WorktreeID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("repo.issueCommandRead", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type issueCommandReadArgs struct {
			WorktreeID string `json:"worktreeId"`
		}
		in, err := decodeArg[issueCommandReadArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, repoSSHStatusWorkspaceRPCTimeout)
		defer cancel()
		resp, err := git.ReadIssueCommand(rpcCtx, &gitgatewayv1.ReadIssueCommandRequest{WorktreeId: in.WorktreeID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("repo.issueCommandWrite", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type issueCommandWriteArgs struct {
			WorktreeID string `json:"worktreeId"`
			Content    string `json:"content"`
		}
		in, err := decodeArg[issueCommandWriteArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, repoSSHStatusWorkspaceRPCTimeout)
		defer cancel()
		_, err = git.WriteIssueCommand(rpcCtx, &gitgatewayv1.WriteIssueCommandRequest{WorktreeId: in.WorktreeID, Content: in.Content})
		return nil, err
	})

	r.Register("repo.setupScriptImports", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type setupScriptImportsArgs struct {
			WorktreeID string `json:"worktreeId"`
		}
		in, err := decodeArg[setupScriptImportsArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, repoSSHStatusWorkspaceRPCTimeout)
		defer cancel()
		resp, err := git.ScanSetupScriptImports(rpcCtx, &gitgatewayv1.ScanSetupScriptImportsRequest{WorktreeId: in.WorktreeID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
}

// ── ssh.* ────────────────────────────────────────────────────────────────
//
// ssh.getUserAccount in the old backend was never a distinct "Linux account
// provisioning" concept — it just read the target's configured username, so
// it derives from ListSshTargets client-side rather than getting its own
// RPC.
func registerSshChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("ssh.listTargets", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, repoSSHStatusWorkspaceRPCTimeout)
		defer cancel()
		resp, err := client.ListSshTargets(rpcCtx, &infrafleetv1.ListSshTargetsRequest{})
		if err != nil {
			return nil, err
		}
		return resp.GetSshTargets(), nil
	})

	r.Register("ssh.getUserAccount", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type getUserAccountArgs struct {
			SshTargetID string `json:"sshTargetId"`
		}
		in, err := decodeArg[getUserAccountArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, repoSSHStatusWorkspaceRPCTimeout)
		defer cancel()
		resp, err := client.ListSshTargets(rpcCtx, &infrafleetv1.ListSshTargetsRequest{})
		if err != nil {
			return nil, err
		}
		for _, t := range resp.GetSshTargets() {
			if t.GetId() == in.SshTargetID {
				return map[string]string{"username": t.GetUser()}, nil
			}
		}
		return nil, fmt.Errorf("ssh target %q not found", in.SshTargetID)
	})

	r.Register("ssh.getState", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type getStateArgs struct {
			SshTargetID string `json:"sshTargetId"`
		}
		in, err := decodeArg[getStateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, repoSSHStatusWorkspaceRPCTimeout)
		defer cancel()
		resp, err := client.GetSshState(rpcCtx, &infrafleetv1.GetSshStateRequest{SshTargetId: in.SshTargetID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("ssh.connect", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type connectArgs struct {
			SshTargetID string `json:"sshTargetId"`
		}
		in, err := decodeArg[connectArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		// Longer-than-default deadline: this is an SSH handshake, not a
		// Postgres read — infra-fleet-service.md §8's "explicit timeout
		// distinct from the default 5s intra-cluster gRPC deadline" rule,
		// same reasoning as BootstrapFleetTarget's streaming RPC.
		rpcCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		resp, err := client.EstablishConnection(rpcCtx, &infrafleetv1.EstablishConnectionRequest{SshTargetId: in.SshTargetID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
}

// ── status.get ────────────────────────────────────────────────────────
//
// Registered as a fast, LOCAL (no downstream call) response, same pattern
// as registerPreflightChannels in channels.go — see SOL-025 for why:
// status.get's simplest caller (windows-terminal-capability-read.ts,
// target.kind==='local') reads nothing but hostPlatform.
//
// CORRECTED (see TASK-036's "Status by layer" section): this doc comment
// used to claim browser-pane-remote.tsx's target.kind==='environment' call
// path was Electron-desktop-only and out of scope for backend-go. That was
// wrong for the web/server-mode build api-gateway actually targets —
// window.api.runtimeEnvironments.call/subscribe DOES reach this package's
// /ws surface for a session-auth environment (WebSessionClient), proven by
// accounts.subscribe (TASK-023). capabilities now reports
// "browser.screencast.v1" for real, matching
// frontend/src/shared/protocol-version.ts's RUNTIME_CAPABILITIES entry —
// browser-pane-remote.tsx's capability gate
// (status.capabilities.includes('browser.screencast.v1')) checks exactly
// this before ever opening the browser.screencast subscription
// (channels_browser_screencast.go).
//
// runtimeId/graphStatus/authoritativeWindowId/liveTabCount/liveLeafCount
// mirror Electron's multi-window runtime-graph concept, which has no
// server-mode equivalent — reported as honest zero-values, not fabricated,
// matching preflight.check's gh/glab convention.
func registerStatusChannels(r *Registry) {
	r.Register("status.get", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		return map[string]any{
			"runtimeId":                         "api-gateway",
			"rendererGraphEpoch":                0,
			"graphStatus":                       "n/a", // no window/tab graph server-side
			"authoritativeWindowId":             nil,
			"liveTabCount":                      0,
			"liveLeafCount":                     0,
			"runtimeProtocolVersion":            currentRuntimeProtocolVersion,
			"minCompatibleRuntimeClientVersion": minCompatibleRuntimeClientVersion,
			"capabilities":                      []string{"browser.screencast.v1"},
			"hostPlatform":                      hostPlatformString(), // the one field windows-terminal-capability-read.ts actually reads
		}, nil
	})
}

// currentRuntimeProtocolVersion/minCompatibleRuntimeClientVersion should
// match frontend/src/shared/protocol-version.ts's RUNTIME_PROTOCOL_VERSION/
// MIN_COMPATIBLE_RUNTIME_SERVER_VERSION constants at implementation time —
// check that file's current values rather than trusting this hardcoded
// number to stay in sync.
const (
	currentRuntimeProtocolVersion     = 3
	minCompatibleRuntimeClientVersion = 2
)

// hostPlatformString reports runtime.GOOS translated to match
// RuntimeStatus.hostPlatform's frontend type (NodeJS.Platform:
// 'win32' | 'darwin' | 'linux' | ...) — runtime.GOOS already matches on
// darwin/linux; only "windows" needs translating to "win32".
func hostPlatformString() string {
	if runtime.GOOS == "windows" {
		return "win32"
	}
	return runtime.GOOS
}

// ── workspacePorts.* ─────────────────────────────────────────────────────
//
// scan calls the already-implemented ScanWorkspacePorts (infrafleet.proto) —
// closes TS Gap 7, see scan_workspace_ports.go's doc comment: always
// resolves the connection first, relays to the agent's ports.scan when
// connectionId is bound, only returns [] for a genuinely unconnected
// worktree. kill calls the new KillWorkspacePort RPC (TASK-170/171),
// following the exact same resolve-then-dispatch shape.
//
// Arg-shape caveat: the frontend's killWorkspacePortForTarget/scan call
// sites pass {repoId, pid, port}/{repoId}
// (frontend/src/renderer/src/lib/workspace-port-actions.ts), not
// {connectionId, worktreeId} directly — repoId needs resolving to the
// worktree's connectionId before calling these RPCs. This handler decodes
// {connectionId, worktreeId} directly per this package's "best-effort,
// verify against the actual call site" convention (channels.go's top-of-file
// doc comment) — verify the exact repoId -> connectionId/worktreeId lookup
// against the real frontend call site before shipping; likely a
// project-service.ListWorktrees join keyed by repoId, resolved either in
// this handler or upstream of it. Not resolved further here.
func registerWorkspacePortsChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("workspacePorts.scan", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type scanArgs struct {
			ConnectionID string `json:"connectionId"`
			WorktreeID   string `json:"worktreeId"`
		}
		in, err := decodeArg[scanArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, repoSSHStatusWorkspaceRPCTimeout)
		defer cancel()
		resp, err := client.ScanWorkspacePorts(rpcCtx, &infrafleetv1.ScanWorkspacePortsRequest{
			ConnectionId: in.ConnectionID,
			WorktreeId:   in.WorktreeID,
		})
		if err != nil {
			return nil, err
		}
		return toWorkspacePortScanResult(resp.GetOpenPorts()), nil
	})

	r.Register("workspacePorts.kill", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type killArgs struct {
			ConnectionID string `json:"connectionId"`
			WorktreeID   string `json:"worktreeId"`
			PID          int32  `json:"pid"`
			Port         int32  `json:"port"`
		}
		in, err := decodeArg[killArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, repoSSHStatusWorkspaceRPCTimeout)
		defer cancel()
		resp, err := client.KillWorkspacePort(rpcCtx, &infrafleetv1.KillWorkspacePortRequest{
			ConnectionId: in.ConnectionID, WorktreeId: in.WorktreeID, Pid: in.PID, Port: in.Port,
		})
		if err != nil {
			return nil, err
		}
		if !resp.GetOk() {
			return map[string]any{"ok": false, "reason": resp.GetReason()}, nil
		}
		return map[string]any{"ok": true}, nil
	})
}

// toWorkspacePortScanResult maps ScanWorkspacePortsResponse's []int32 open
// ports onto the frontend's WorkspacePortScanResult{platform, scannedAt,
// ports, unavailableReason?} shape (frontend/src/shared/workspace-ports.ts)
// — platform reported as "unknown" (this service never inspects the target
// host's OS) and unavailableReason left empty on success, honest-placeholder
// convention.
func toWorkspacePortScanResult(openPorts []int32) map[string]any {
	return map[string]any{
		"platform":  "unknown",
		"scannedAt": time.Now().UnixMilli(),
		"ports":     openPorts,
	}
}

// toRepoRoleArg maps the wscompat wire arg's repo-role string
// ("developer" | "lead" | "admin") onto projectv1.RepoRole — mirrors
// channels_tenant_project.go's toProjectRoleArg for the same kind of
// string-to-enum wire mapping, one tier down.
func toRepoRoleArg(role string) projectv1.RepoRole {
	switch role {
	case "developer":
		return projectv1.RepoRole_REPO_ROLE_DEVELOPER
	case "lead":
		return projectv1.RepoRole_REPO_ROLE_LEAD
	case "admin":
		return projectv1.RepoRole_REPO_ROLE_ADMIN
	default:
		return projectv1.RepoRole_REPO_ROLE_UNSPECIFIED
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Integration TODO — the one-line wiring this file deliberately does NOT
// do itself (channels.go is off-limits to this pass, see this file's top
// doc comment):
//
//  1. In channels.go's RegisterRealChannels, add a `projectClient
//     projectv1.ProjectServiceClient` parameter (main.go already dials
//     project-service — see projectClient at main.go's composition root)
//     and call:
//
//       registerRepoSshStatusWorkspaceChannels(r, projectClient, gitClient, infraFleetClient)
//
//  2. In cmd/server/main.go, thread projectClient into the
//     RegisterRealChannels(...) call site alongside the existing
//     annotationClient/taskClient/gitClient/automationClient/infraFleetClient
//     arguments.
//
// No new gRPC client dials are needed for either step — projectClient and
// infraFleetClient are already constructed in main.go's composition root.
// ─────────────────────────────────────────────────────────────────────────
