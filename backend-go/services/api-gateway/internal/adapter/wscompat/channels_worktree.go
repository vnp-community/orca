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
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

func registerWorktreeChannels(r *Registry, gitClient gitgatewayv1.GitGatewayServiceClient, projectClient projectv1.ProjectServiceClient) {
	r.Register("worktree.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		// Field names/shape mirror the ONLY real callers — web-preload-api.ts's
		// worktrees.create and worktrees.ts's createWorktree (both frontend/src)
		// — never {projectId, repoId, branch, baseRef}, which nothing sends.
		// project_id is deliberately not decoded here: CreateWorktree.Execute
		// now resolves it server-side from the repo (see create_worktree.go).
		type createArgs struct {
			RepoID             string  `json:"repo"`
			Name               string  `json:"name"`
			BaseBranch         string  `json:"baseBranch"`
			BranchNameOverride string  `json:"branchNameOverride"`
			DisplayName        string  `json:"displayName"`
			LinkedIssue        *int32  `json:"linkedIssue"`
			LinkedPR           *int32  `json:"linkedPR"`
			LinkedLinearIssue  *string `json:"linkedLinearIssue"`

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
		// branchNameOverride wins when present — worktrees.ts's own
		// retry-name-conflict loop retries both under this same rule.
		branch := in.Name
		if in.BranchNameOverride != "" {
			branch = in.BranchNameOverride
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := gitClient.CreateWorktree(ctx, &gitgatewayv1.CreateWorktreeRequest{
			RepoId: in.RepoID, Branch: branch, BaseRef: in.BaseBranch,
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

		displayName := in.DisplayName
		if displayName == "" {
			displayName = displayNameForDetectedWorktree(branch, resp.GetPath())
		}
		now := time.Now().UnixMilli()
		// {worktree: ...} — the frontend (worktrees.ts's createWorktree) reads
		// result.worktree.id/.path straight off this response; CreateWorktreeResponse
		// itself only carries worktree_id/path/head_sha, so the rest of the rich
		// client Worktree shape is filled in here from the request + safe
		// defaults, same convention as detectedWorktreeView below.
		return map[string]any{
			"worktree": worktreeView{
				ID:                resp.GetWorktreeId(),
				RepoID:            in.RepoID,
				DisplayName:       displayName,
				LinkedIssue:       in.LinkedIssue,
				LinkedPR:          in.LinkedPR,
				LinkedLinearIssue: in.LinkedLinearIssue,
				LastActivityAt:    now,
				CreatedAt:         now,
				Path:              resp.GetPath(),
				Head:              resp.GetHeadSha(),
				Branch:            branch,
			},
		}, nil
	})

	r.Register("worktree.rm", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type rmArgs struct {
			// The frontend always sends toRuntimeWorktreeSelector(worktreeId)
			// (runtime-worktree-selector.ts) under the key "worktree", never
			// "worktreeId" — and always "id:"-prefixed.
			Worktree string `json:"worktree"`
			Force    bool   `json:"force"`
		}
		in, err := decodeArg[rmArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		worktreeID := stripWorktreeSelectorPrefix(in.Worktree)
		if _, err := gitClient.RemoveWorktree(ctx, &gitgatewayv1.RemoveWorktreeRequest{WorktreeId: worktreeID, Force: in.Force}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})

	r.Register("worktree.forceDeleteBranch", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		// forceDeletePreservedBranch (worktrees.ts) sends "branchName", not
		// "branch" — expectedHead is decoded but currently unused: neither
		// ForceDeleteBranchRequest nor ForceDeleteBranch.Execute has a
		// head-check guard yet (a known gap, not addressed by this fix).
		type forceDeleteArgs struct {
			Worktree     string `json:"worktree"`
			BranchName   string `json:"branchName"`
			ExpectedHead string `json:"expectedHead"`
		}
		in, err := decodeArg[forceDeleteArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		worktreeID := stripWorktreeSelectorPrefix(in.Worktree)
		if _, err := gitClient.ForceDeleteBranch(ctx, &gitgatewayv1.ForceDeleteBranchRequest{WorktreeId: worktreeID, Branch: in.BranchName}); err != nil {
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
		// NOTE: ListWorktreesRequest is project-scoped only (no repo_id
		// filter) — callers that send {repo, limit} instead of {projectId}
		// (web-preload-api.ts's worktrees.list/listAllRuntimeWorktrees) hit a
		// pre-existing, already-documented gap (see worktrees.ts's own
		// "worktree.list's Go handler decodes only {projectId}" comment) this
		// fix does not attempt to redesign; only the response's per-item
		// shape is corrected here, matching worktree.create's reshaping.
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
		protoWorktrees := resp.GetWorktrees()
		views := make([]worktreeView, 0, len(protoWorktrees))
		for _, w := range protoWorktrees {
			views = append(views, worktreeViewFromProto(w))
		}
		// {worktrees: [...]} — every caller (web-preload-api.ts's list/
		// listAllRuntimeWorktrees, worktrees.ts's legacy fallback) reads
		// result.worktrees, not a bare array.
		return map[string]any{"worktrees": views}, nil
	})

	r.Register("worktree.set", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		// worktrees.ts always sends {worktree: "id:<uuid>", ...updates} —
		// never "worktreeId". Only "active" is forwarded to
		// SetWorktreeActivationRequest today; other metadata keys
		// (displayName/comment/pushTarget/... from updateMeta's rpcUpdates
		// spread) are decoded here but not yet persisted — a known,
		// pre-existing gap in this RPC's scope, not introduced by this fix.
		type setArgs struct {
			Worktree string `json:"worktree"`
			Active   bool   `json:"active"`
		}
		in, err := decodeArg[setArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		worktreeID := stripWorktreeSelectorPrefix(in.Worktree)
		resp, err := projectClient.SetWorktreeActivation(ctx, &projectv1.SetWorktreeActivationRequest{WorktreeId: worktreeID, Active: in.Active})
		if err != nil {
			return nil, err
		}
		return map[string]any{"worktree": worktreeViewFromProto(resp.GetWorktree())}, nil
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

		return map[string]any{
			"repoId":        in.RepoID,
			"authoritative": true,
			"source":        "git",
			"worktrees":     mergeDetectedWorktrees(in.RepoID, in.ProjectID, onDisk.GetOnDiskWorktrees(), known.GetWorktrees()),
		}, nil
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

// worktreeView is the wire shape worktree.create/worktree.list/worktree.set
// return — mirrors frontend/src/shared/types.ts's rich client Worktree type.
// git-gateway-service's CreateWorktreeResponse and project-service's Worktree
// proto message both carry far fewer fields than the client type requires;
// every field this backend has no data source for yet gets the same safe
// default detectedWorktreeView below already established.
type worktreeView struct {
	ID                string  `json:"id"`
	RepoID            string  `json:"repoId"`
	ProjectID         string  `json:"projectId,omitempty"`
	DisplayName       string  `json:"displayName"`
	Comment           string  `json:"comment"`
	LinkedIssue       *int32  `json:"linkedIssue"`
	LinkedPR          *int32  `json:"linkedPR"`
	LinkedLinearIssue *string `json:"linkedLinearIssue"`
	IsArchived        bool    `json:"isArchived"`
	IsUnread          bool    `json:"isUnread"`
	IsPinned          bool    `json:"isPinned"`
	SortOrder         int32   `json:"sortOrder"`
	LastActivityAt    int64   `json:"lastActivityAt"`
	CreatedAt         int64   `json:"createdAt,omitempty"`
	Path              string  `json:"path"`
	Head              string  `json:"head"`
	Branch            string  `json:"branch"`
	IsBare            bool    `json:"isBare"`
	IsMainWorktree    bool    `json:"isMainWorktree"`
}

// worktreeViewFromProto reshapes project-service's raw Worktree proto
// message (worktree.list/worktree.set's response) into the client's rich
// shape — same treatment worktree.create's response gets, just from a
// different source message.
func worktreeViewFromProto(w *projectv1.Worktree) worktreeView {
	return worktreeView{
		ID:             w.GetId(),
		RepoID:         w.GetRepoId(),
		ProjectID:      w.GetProjectId(),
		DisplayName:    displayNameForDetectedWorktree(w.GetBranch(), w.GetPath()),
		Path:           w.GetPath(),
		Branch:         w.GetBranch(),
		LastActivityAt: w.GetCreatedAtUnixMs(),
		CreatedAt:      w.GetCreatedAtUnixMs(),
	}
}

// stripWorktreeSelectorPrefix undoes toRuntimeWorktreeSelector's "id:" prefix
// convention (frontend/src/renderer/src/runtime/runtime-worktree-selector.ts)
// — every real caller of worktree.set/rm/forceDeleteBranch sends a
// "worktree" arg through that selector, always "id:"-prefixed.
func stripWorktreeSelectorPrefix(selector string) string {
	return strings.TrimPrefix(selector, "id:")
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

// detectedWorktreeView mirrors the frontend's DetectedWorktree type
// (shared/types.ts: Worktree & {ownership, selectedCheckout, visible}) —
// worktree.detectedList's real response shape, replacing the earlier
// {orphanedPaths: [...]} stub this handler shipped with (see the spec
// doc's "Thirtieth"/"Thirty-first"/"Thirty-second" entries for that
// history). Every field this backend has no data source for gets the
// exact same safe default the frontend's OWN legacy-fallback synthesis
// (toLegacyDetectedWorktreeResult in store/slices/worktrees.ts and
// web-preload-api.ts) already established, so a worktree this handler
// discovers is shape-identical to one that path already produced.
type detectedWorktreeView struct {
	ID                string  `json:"id"`
	RepoID            string  `json:"repoId"`
	ProjectID         string  `json:"projectId,omitempty"`
	DisplayName       string  `json:"displayName"`
	Comment           string  `json:"comment"`
	LinkedIssue       *int32  `json:"linkedIssue"`
	LinkedPR          *int32  `json:"linkedPR"`
	LinkedLinearIssue *string `json:"linkedLinearIssue"`
	IsArchived        bool    `json:"isArchived"`
	IsUnread          bool    `json:"isUnread"`
	IsPinned          bool    `json:"isPinned"`
	SortOrder         int32   `json:"sortOrder"`
	LastActivityAt    int64   `json:"lastActivityAt"`
	Path              string  `json:"path"`
	Head              string  `json:"head"`
	Branch            string  `json:"branch"`
	IsBare            bool    `json:"isBare"`
	IsMainWorktree    bool    `json:"isMainWorktree"`
	Ownership         string  `json:"ownership"`
	SelectedCheckout  bool    `json:"selectedCheckout"`
	Visible           bool    `json:"visible"`
}

// mergeDetectedWorktrees combines git-gateway-service's on-disk ground
// truth (real paths, guaranteed to exist right now) with project-service's
// bookkeeping (the worktree's real id/lineage, when Orca created it) —
// per this file's own top-of-file doc comment, this merge is deliberately
// computed here at api-gateway's edge, not inside either owning service.
//
// A bookkept worktree missing from onDisk is deliberately NOT included —
// the frontend's own reconciliation (getRemovedWorktreeIdsAfterAuthoritative-
// Scan in store/slices/worktrees.ts) already purges a bookkept id that
// isn't in this result's ids, comparing against its own separately-fetched
// worktreesByRepo state; duplicating that purge decision here would be a
// second, harder-to-keep-in-sync source of truth for the same decision.
func mergeDetectedWorktrees(repoID, projectID string, onDisk []*gitgatewayv1.DetectedWorktreeGitInfo, known []*projectv1.Worktree) []detectedWorktreeView {
	knownByPath := make(map[string]*projectv1.Worktree, len(known))
	for _, w := range known {
		knownByPath[w.GetPath()] = w
	}

	out := make([]detectedWorktreeView, 0, len(onDisk))
	for i, info := range onDisk {
		view := detectedWorktreeView{
			ProjectID:        projectID,
			Path:             info.GetPath(),
			Head:             info.GetHead(),
			Branch:           info.GetBranch(),
			IsMainWorktree:   i == 0,
			SelectedCheckout: false,
			Visible:          true,
		}
		if w, ok := knownByPath[info.GetPath()]; ok {
			// Bookkept by project-service — a worktree Orca itself created
			// (or previously reconciled). Reuse its real id so the
			// frontend's own already-fetched worktreesByRepo state matches
			// up by id, not just by path.
			view.ID = w.GetId()
			view.RepoID = w.GetRepoId()
			view.Ownership = "orca-managed"
			view.LastActivityAt = w.GetCreatedAtUnixMs()
		} else {
			// On disk, but Orca has no bookkeeping row for it — created
			// outside Orca's own worktree.create flow (a manual
			// `git worktree add`, or an import this pass doesn't attempt to
			// resolve). Synthesize the same id shape every other worktree
			// id in this codebase uses (see shared/worktree-id.ts).
			view.ID = repoID + "::" + info.GetPath()
			view.RepoID = repoID
			view.Ownership = "external"
		}
		view.DisplayName = displayNameForDetectedWorktree(view.Branch, view.Path)
		out = append(out, view)
	}
	return out
}

// displayNameForDetectedWorktree picks a reasonable label for a worktree
// this backend has no explicit display_name for (project.worktrees has no
// such column) — the branch name when one exists (the common, meaningful
// case), else the path's own basename (a detached-HEAD worktree, or one
// git couldn't resolve a branch for).
func displayNameForDetectedWorktree(branch, path string) string {
	if short, ok := strings.CutPrefix(branch, "refs/heads/"); ok && short != "" {
		return short
	}
	if branch != "" {
		return branch
	}
	trimmed := strings.TrimRight(path, "/\\")
	if base := trimmed; base != "" {
		if idx := strings.LastIndexAny(trimmed, "/\\"); idx != -1 {
			base = trimmed[idx+1:]
		}
		if base != "" {
			return base
		}
	}
	return path
}
