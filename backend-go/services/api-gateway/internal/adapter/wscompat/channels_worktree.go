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

	"github.com/stablyai/orca-go/common/apperrors"
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
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := gitClient.CreateWorktree(ctx, &gitgatewayv1.CreateWorktreeRequest{
			ProjectId: in.ProjectID, RepoId: in.RepoID, Branch: in.Branch, BaseRef: in.BaseRef,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	// worktree.createFromIssue — CreateWorktreeFromIssue's saga (SOL-PI-02),
	// next to worktree.create above. Translates the decoded args' flat
	// provider/repo/number/issueRef shape into the RPC's oneof issue_source
	// (ScmIssueRef vs. TrackerIssueRef).
	r.Register("worktree.createFromIssue", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createFromIssueArgs struct {
			ProjectID        string `json:"projectId"`
			RepoID           string `json:"repoId"`
			BaseRef          string `json:"baseRef"`
			Provider         string `json:"provider"` // "github"|"gitlab"|"jira"|"linear"
			Repo             string `json:"repo"`     // scm only
			Number           int32  `json:"number"`   // scm only
			IssueRef         string `json:"issueRef"` // tracker only
			SkipAgentStart   bool   `json:"skipAgentStart"`
			SkipStatusUpdate bool   `json:"skipStatusUpdate"`
		}
		in, err := decodeArg[createFromIssueArgs](args, 0)
		if err != nil {
			return nil, err
		}

		req := &gitgatewayv1.CreateWorktreeFromIssueRequest{
			ProjectId: in.ProjectID, RepoId: in.RepoID, BaseRef: in.BaseRef,
			SkipAgentStart: in.SkipAgentStart, SkipStatusUpdate: in.SkipStatusUpdate,
		}
		switch in.Provider {
		case "github", "gitlab":
			if in.Repo == "" || in.Number == 0 {
				return nil, apperrors.New(apperrors.KindInvalidArgument, "WORKTREE_FROM_ISSUE_MISSING_SCM_FIELDS", "repo and number are required for github/gitlab", nil)
			}
			req.IssueSource = &gitgatewayv1.CreateWorktreeFromIssueRequest_ScmIssue{
				ScmIssue: &gitgatewayv1.ScmIssueRef{Provider: in.Provider, Repo: in.Repo, Number: in.Number},
			}
		case "jira", "linear":
			if in.IssueRef == "" {
				return nil, apperrors.New(apperrors.KindInvalidArgument, "WORKTREE_FROM_ISSUE_MISSING_TRACKER_REF", "issueRef is required for jira/linear", nil)
			}
			req.IssueSource = &gitgatewayv1.CreateWorktreeFromIssueRequest_TrackerIssue{
				TrackerIssue: &gitgatewayv1.TrackerIssueRef{Provider: in.Provider, IssueRef: in.IssueRef},
			}
		default:
			return nil, apperrors.New(apperrors.KindInvalidArgument, "WORKTREE_FROM_ISSUE_UNKNOWN_PROVIDER", "provider must be one of github/gitlab/jira/linear", nil)
		}

		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := gitClient.CreateWorktreeFromIssue(ctx, req)
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("worktree.rm", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type rmArgs struct {
			WorktreeID string `json:"worktreeId"`
			Force      bool   `json:"force"`
			// AllowOpenPR defaults to false when absent — BR-AT-12's
			// open-PR safety check protects every existing manual-delete
			// caller by default; an intentional behavior change from
			// before this check existed.
			AllowOpenPR bool `json:"allowOpenPr"`
		}
		in, err := decodeArg[rmArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		if _, err := gitClient.RemoveWorktree(ctx, &gitgatewayv1.RemoveWorktreeRequest{WorktreeId: in.WorktreeID, Force: in.Force, AllowOpenPr: in.AllowOpenPR}); err != nil {
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
}
