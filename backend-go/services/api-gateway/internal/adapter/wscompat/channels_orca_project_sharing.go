// orcaProjects.* — cross-project source sharing. Links another Project's
// repos/worktrees into a project's shared view — the "Linked Projects"
// picker in Project Settings. Live bug this closes: picking a project to
// link errored with "channel \"orcaProjects.linkSourceProject\" is not yet
// implemented in backend-go" (project-service's own README flagged this as
// a genuine, not-half-built gap — see project-service's
// LinkSourceProject/UnlinkSourceProject/ListSourceProjects/
// GetSharedProjectData usecases, now real).
//
// Naming note: legacy's "OrcaProject" (the shared, membership-scoped
// container) IS project-service's Project — there is no separate entity.
// "orcaProjectId" on the wire below is simply the container project's id;
// kept as the wire field name because it's the frontend's own, already-
// shipped, unchangeable contract (types/workspace-types.ts,
// LinkedProjectsManager.tsx/CreateProjectDialog.tsx).
//
// No access-control code lives here: every handler just AttachIdentity +
// call + propagate the raw gRPC error, matching every other project.*
// channel in this package — project-service's own requireProjectAccess
// (OPA-gated) is what actually authorizes each call, and its denial comes
// back as a clear error automatically.
package wscompat

import (
	"context"
	"encoding/json"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// sourceProjectRefView is orcaProjects.list/linkSourceProject's per-link
// wire shape — {ownerUserId, projectId} is the frontend's already-shipped
// contract (types/workspace-types.ts's SourceProjectRef). ownerUserId maps
// to this service's linked_by (who performed the link — an audit trail,
// never an ownership claim; see project.proto's SourceProject doc comment)
// since project-service has no separate "owner of a per-user project"
// concept the way the legacy TS backend did.
type sourceProjectRefView struct {
	OwnerUserID string `json:"ownerUserId"`
	ProjectID   string `json:"projectId"`
}

func toSourceProjectRefView(sp *projectv1.SourceProject) sourceProjectRefView {
	return sourceProjectRefView{OwnerUserID: sp.GetLinkedBy(), ProjectID: sp.GetSourceProjectId()}
}

// sharedWorktreeView is orcaProjects.getProjectData's minimal per-worktree
// shape — deliberately not the rich worktreeView channels_worktree.go's
// worktree.* channels return (that shape needs live git-status data this
// cross-project read has no reason to fetch); just enough to identify and
// locate a linked project's worktrees.
type sharedWorktreeView struct {
	ID     string `json:"id"`
	RepoID string `json:"repoId"`
	Path   string `json:"path"`
	Branch string `json:"branch"`
}

func toSharedWorktreeView(wt *projectv1.Worktree) sharedWorktreeView {
	return sharedWorktreeView{ID: wt.GetId(), RepoID: wt.GetRepoId(), Path: wt.GetPath(), Branch: wt.GetBranch()}
}

func registerOrcaProjectSharingChannels(r *Registry, client projectv1.ProjectServiceClient) {
	// orcaProjects.list has no per-id fetch RPC — the frontend filters by
	// orcaProjectId locally (LinkedProjectsManager.tsx) — so this returns
	// every project the caller is a member of, each annotated with its
	// linked source projects. N+1 (one ListSourceProjects call per
	// project), matching the legacy reference implementation's own
	// Promise.all pattern — acceptable, project counts per caller are
	// small.
	r.Register("orcaProjects.list", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()

		listResp, err := client.ListProjects(rpcCtx, &projectv1.ListProjectsRequest{TenantId: id.TenantID})
		if err != nil {
			return nil, err
		}

		type orcaProjectListItem struct {
			OrcaProject    projectView            `json:"orcaProject"`
			SourceProjects []sourceProjectRefView `json:"sourceProjects"`
		}
		items := make([]orcaProjectListItem, 0, len(listResp.GetProjects()))
		for _, p := range listResp.GetProjects() {
			sourcesResp, err := client.ListSourceProjects(rpcCtx, &projectv1.ListSourceProjectsRequest{ContainerProjectId: p.GetId()})
			if err != nil {
				return nil, err
			}
			refs := make([]sourceProjectRefView, 0, len(sourcesResp.GetSourceProjects()))
			for _, sp := range sourcesResp.GetSourceProjects() {
				refs = append(refs, toSourceProjectRefView(sp))
			}
			items = append(items, orcaProjectListItem{OrcaProject: toProjectView(p), SourceProjects: refs})
		}
		return items, nil
	})

	linkArgs := func(args []json.RawMessage) (string, string, error) {
		type in struct {
			OrcaProjectID string `json:"orcaProjectId"`
			ProjectID     string `json:"projectId"`
		}
		v, err := decodeArg[in](args, 0)
		if err != nil {
			return "", "", err
		}
		return v.OrcaProjectID, v.ProjectID, nil
	}

	r.Register("orcaProjects.linkSourceProject", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		orcaProjectID, projectID, err := linkArgs(args)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		if _, err := client.LinkSourceProject(rpcCtx, &projectv1.LinkSourceProjectRequest{
			ContainerProjectId: orcaProjectID, SourceProjectId: projectID,
		}); err != nil {
			return nil, err
		}
		return map[string]any{"success": true}, nil
	})

	r.Register("orcaProjects.unlinkSourceProject", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		orcaProjectID, projectID, err := linkArgs(args)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		if _, err := client.UnlinkSourceProject(rpcCtx, &projectv1.UnlinkSourceProjectRequest{
			ContainerProjectId: orcaProjectID, SourceProjectId: projectID,
		}); err != nil {
			return nil, err
		}
		return map[string]any{"success": true}, nil
	})

	// orcaProjects.getProjectData: now has a real caller —
	// LinkedProjectsManager.tsx resolves the display name of a linked
	// project that isn't in the caller's own local `projects` (e.g. linked
	// by a different OrcaProject member) via this channel, rather than
	// showing a raw UUID. Response shape confirmed adequate as shipped:
	// the caller only reads `project.name`.
	r.Register("orcaProjects.getProjectData", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		orcaProjectID, projectID, err := linkArgs(args)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.GetSharedProjectData(rpcCtx, &projectv1.GetSharedProjectDataRequest{
			ContainerProjectId: orcaProjectID, SourceProjectId: projectID,
		})
		if err != nil {
			return nil, err
		}
		repos := make([]repoView, 0, len(resp.GetRepos()))
		for _, rp := range resp.GetRepos() {
			repos = append(repos, toRepoView(rp))
		}
		worktrees := make([]sharedWorktreeView, 0, len(resp.GetWorktrees()))
		for _, wt := range resp.GetWorktrees() {
			worktrees = append(worktrees, toSharedWorktreeView(wt))
		}
		return map[string]any{
			"project":   toProjectView(resp.GetProject()),
			"repos":     repos,
			"worktrees": worktrees,
		}, nil
	})
}
