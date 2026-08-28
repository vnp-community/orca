// ── worktree.* (git-gateway-service + project-service) ──────────────────
//
// 7 of the 8 channels here are direct 1:1 unary wrappers, same shape as
// registerGitChannels. worktree.list/worktree.set are pure bookkeeping
// wrappers over ProjectServiceClient.ListWorktrees/SetWorktreeActivation
// (already-real RPCs, per BUG-031 — no git-gateway-service involvement).
// worktree.detectedList is the one aggregation: git-gateway-service's
// DetectWorktrees raw on-disk paths merged against project-service's
// ListWorktrees bookkeeping, computed at api-gateway's edge layer per
// 05-data-architecture.md's explicit prescription for a multi-service
// view — parallel gRPC calls via errgroup, merged here, not inside either
// owning service.
//
// This file is intentionally self-contained (TASK-195): registerWorktreeChannels
// is not yet called from channels.go's RegisterRealChannels or wired
// through cmd/server/main.go's projectClient — that central integration
// (channels.go/registry.go/handler.go/main.go) happens in a separate pass
// alongside the concurrent git.*/files.* channel work also landing in this
// package. This file only needs to compile standalone.
package wscompat

import (
	"context"
	"encoding/json"

	"golang.org/x/sync/errgroup"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

func registerWorktreeChannels(r *Registry, gitClient gitgatewayv1.GitGatewayServiceClient, projectClient projectv1.ProjectServiceClient) {
	r.Register("worktree.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			ProjectID string `json:"projectId"`
			RepoID    string `json:"repoId"`
			Branch    string `json:"branch"`
			BaseRef   string `json:"baseRef"`

			// Optional lineage-capture context — see
			// proto/orca/project/v1/project.proto's WorktreeLineageEntry doc
			// comment. Explicit-capture only; omitted fields mean "no
			// lineage captured", the common case.
			ParentWorktreeID        string `json:"parentWorktreeId"`
			Origin                  string `json:"origin"`
			CaptureSource           string `json:"captureSource"`
			TaskID                  string `json:"taskId"`
			OrchestrationRunID      string `json:"orchestrationRunId"`
			CoordinatorHandle       string `json:"coordinatorHandle"`
			CreatedByTerminalHandle string `json:"createdByTerminalHandle"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := gitClient.CreateWorktree(ctx, &gitgatewayv1.CreateWorktreeRequest{
			ProjectId: in.ProjectID, RepoId: in.RepoID, Branch: in.Branch, BaseRef: in.BaseRef,
			ParentWorktreeId:        nonEmptyPtr(in.ParentWorktreeID),
			Origin:                  nonEmptyPtr(in.Origin),
			CaptureSource:           nonEmptyPtr(in.CaptureSource),
			TaskId:                  nonEmptyPtr(in.TaskID),
			OrchestrationRunId:      nonEmptyPtr(in.OrchestrationRunID),
			CoordinatorHandle:       nonEmptyPtr(in.CoordinatorHandle),
			CreatedByTerminalHandle: nonEmptyPtr(in.CreatedByTerminalHandle),
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("worktree.rm", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type rmArgs struct {
			WorktreeID string `json:"worktreeId"`
			Force      bool   `json:"force"`
		}
		in, err := decodeArg[rmArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		if _, err := gitClient.RemoveWorktree(ctx, &gitgatewayv1.RemoveWorktreeRequest{WorktreeId: in.WorktreeID, Force: in.Force}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})

	r.Register("worktree.forceDeleteBranch", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type forceDeleteArgs struct {
			WorktreeID string `json:"worktreeId"`
			Branch     string `json:"branch"`
		}
		in, err := decodeArg[forceDeleteArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		if _, err := gitClient.ForceDeleteBranch(ctx, &gitgatewayv1.ForceDeleteBranchRequest{WorktreeId: in.WorktreeID, Branch: in.Branch}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})

	r.Register("worktree.prefetchCreateBase", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type prefetchArgs struct {
			RepoID  string `json:"repoId"`
			BaseRef string `json:"baseRef"`
		}
		in, err := decodeArg[prefetchArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := gitClient.PrefetchCreateBase(ctx, &gitgatewayv1.PrefetchCreateBaseRequest{RepoId: in.RepoID, BaseRef: in.BaseRef})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("worktree.resolvePrBase", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type resolveArgs struct {
			RepoID   string `json:"repoId"`
			PrNumber int32  `json:"prNumber"`
		}
		in, err := decodeArg[resolveArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := gitClient.ResolvePrBase(ctx, &gitgatewayv1.ResolvePrBaseRequest{RepoId: in.RepoID, PrNumber: in.PrNumber})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("worktree.resolveMrBase", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type resolveArgs struct {
			RepoID   string `json:"repoId"`
			MrNumber int32  `json:"mrNumber"`
		}
		in, err := decodeArg[resolveArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := gitClient.ResolveMrBase(ctx, &gitgatewayv1.ResolveMrBaseRequest{RepoId: in.RepoID, MrNumber: in.MrNumber})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("worktree.list", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listArgs struct {
			ProjectID string `json:"projectId"`
		}
		in, err := decodeArg[listArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := projectClient.ListWorktrees(ctx, &projectv1.ListWorktreesRequest{ProjectId: in.ProjectID})
		if err != nil {
			return nil, err
		}
		return resp.GetWorktrees(), nil
	})

	r.Register("worktree.set", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type setArgs struct {
			WorktreeID string `json:"worktreeId"`
			Active     bool   `json:"active"`
		}
		in, err := decodeArg[setArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := projectClient.SetWorktreeActivation(ctx, &projectv1.SetWorktreeActivationRequest{WorktreeId: in.WorktreeID, Active: in.Active})
		if err != nil {
			return nil, err
		}
		return resp.GetWorktree(), nil
	})

	// worktree.detectedList — the one aggregation. Parallel calls, merged
	// at the edge, per 05-data-architecture.md's explicit prescription for
	// a multi-service view: neither owning service reaches into the
	// other's data.
	r.Register("worktree.detectedList", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type detectedListArgs struct {
			ProjectID string `json:"projectId"`
			RepoID    string `json:"repoId"`
		}
		in, err := decodeArg[detectedListArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})

		var onDisk *gitgatewayv1.DetectWorktreesResponse
		var known *projectv1.ListWorktreesResponse
		g, gctx := errgroup.WithContext(ctx)
		g.Go(func() (err error) {
			onDisk, err = gitClient.DetectWorktrees(gctx, &gitgatewayv1.DetectWorktreesRequest{RepoId: in.RepoID})
			return
		})
		g.Go(func() (err error) {
			known, err = projectClient.ListWorktrees(gctx, &projectv1.ListWorktreesRequest{ProjectId: in.ProjectID})
			return
		})
		if err := g.Wait(); err != nil {
			return nil, err
		}

		knownPaths := make(map[string]bool, len(known.GetWorktrees()))
		for _, w := range known.GetWorktrees() {
			knownPaths[w.GetPath()] = true
		}
		orphaned := make([]string, 0, len(onDisk.GetOnDiskPaths()))
		for _, p := range onDisk.GetOnDiskPaths() {
			if !knownPaths[p] {
				orphaned = append(orphaned, p)
			}
		}
		return map[string]any{"orphanedPaths": orphaned}, nil
	})

	// worktree.lineageList — no args (tenant-scoped via identity + RLS,
	// matching the old TS backend's params:null handler). Explicit-capture
	// only and workspaceLineage is always {} — see
	// proto/orca/project/v1/project.proto's WorktreeLineageEntry doc
	// comment and this file's package-level scope-cut note for why.
	r.Register("worktree.lineageList", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := projectClient.ListWorktreeLineage(ctx, &projectv1.ListWorktreeLineageRequest{})
		if err != nil {
			return nil, err
		}
		lineage := make(map[string]worktreeLineageView, len(resp.GetLineage()))
		for _, e := range resp.GetLineage() {
			lineage[e.GetWorktreeId()] = toWorktreeLineageView(e)
		}
		return map[string]any{
			"lineage":          lineage,
			"workspaceLineage": map[string]any{},
		}, nil
	})
}

// worktreeLineageView is the wire shape worktree.lineageList returns —
// mirrors frontend/src/shared/types.ts's WorktreeLineage. worktreeInstanceId/
// parentWorktreeInstanceId mirror worktreeId/parentWorktreeId 1:1: backend-go
// has no separate "instance" concept the way the old TS backend's
// per-process worktree registration did.
type worktreeLineageView struct {
	WorktreeID               string  `json:"worktreeId"`
	WorktreeInstanceID       string  `json:"worktreeInstanceId"`
	ParentWorktreeID         *string `json:"parentWorktreeId,omitempty"`
	ParentWorktreeInstanceID *string `json:"parentWorktreeInstanceId,omitempty"`
	Origin                   *string `json:"origin,omitempty"`
	CaptureSource            *string `json:"captureSource,omitempty"`
	CaptureConfidence        *string `json:"captureConfidence,omitempty"`
	TaskID                   *string `json:"taskId,omitempty"`
	OrchestrationRunID       *string `json:"orchestrationRunId,omitempty"`
	CoordinatorHandle        *string `json:"coordinatorHandle,omitempty"`
	CreatedByTerminalHandle  *string `json:"createdByTerminalHandle,omitempty"`
	CreatedAt                int64   `json:"createdAt"`
}

func toWorktreeLineageView(e *projectv1.WorktreeLineageEntry) worktreeLineageView {
	return worktreeLineageView{
		WorktreeID:               e.GetWorktreeId(),
		WorktreeInstanceID:       e.GetWorktreeId(),
		ParentWorktreeID:         e.ParentWorktreeId,
		ParentWorktreeInstanceID: e.ParentWorktreeId,
		Origin:                   e.Origin,
		CaptureSource:            e.CaptureSource,
		CaptureConfidence:        e.CaptureConfidence,
		TaskID:                   e.TaskId,
		OrchestrationRunID:       e.OrchestrationRunId,
		CoordinatorHandle:        e.CoordinatorHandle,
		CreatedByTerminalHandle:  e.CreatedByTerminalHandle,
		CreatedAt:                e.GetCreatedAtUnixMs(),
	}
}

// nonEmptyPtr returns nil for an empty string, else a pointer to it — every
// optional lineage field on CreateWorktreeRequest uses this to distinguish
// "not supplied" from a genuinely empty value (same idiom
// git-gateway-service's grpcclient.nonEmptyPtr uses on the other side of
// this same RPC).
func nonEmptyPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
