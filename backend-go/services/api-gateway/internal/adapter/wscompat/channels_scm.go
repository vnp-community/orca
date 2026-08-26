// Channel handlers backing the frontend's github.*, github.project.*,
// gitlab.*, and hostedReview.* namespaces — see
// specs/backend-go/bugs/missing-v1/solutions/SOL-012-github-channels.md,
// SOL-013-gitlab-channels.md, and SOL-014-hostedreview-channels.md.
//
// All four namespaces are wired from this single file (rather than the
// per-namespace channels_github.go/channels_gitlab.go/
// channels_hostedreview.go split those solutions describe) because other
// groups are adding channels to channels.go in parallel worktrees during
// this pass — see this repo's integration-plan note for TASK-071..090.
// registerSCMChannels is the single entry point the eventual integration
// pass wires into channels.go's RegisterRealChannels (one call,
// registerSCMChannels(r, scmClient), plus a scmClient parameter threaded
// through from main.go — see this package's own composition notes).
package wscompat

import (
	"context"
	"encoding/json"
	"time"

	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// scmRPCTimeout bounds every outbound gRPC call this file makes to
// scm-integration-service — no channel here should be able to hang a
// WebSocket connection indefinitely on a slow upstream provider API call.
const scmRPCTimeout = 30 * time.Second

// attachSCMIdentity is a one-line convenience every handler below uses —
// same AttachIdentity call channels.go's existing handlers make, just
// spelled out once here since every github.*/gitlab.*/hostedReview.*
// handler needs it.
func attachSCMIdentity(ctx context.Context, id Identity) context.Context {
	return gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
}

// registerSCMChannels wires every github.*/github.project.*/gitlab.*/
// hostedReview.* channel this pass gives real backend-go implementations
// to, against scm-integration-service's gRPC client. Called once from
// main.go's composition root — see channels.go's RegisterRealChannels for
// where the integration pass adds `registerSCMChannels(r, scmClient)`.
func registerSCMChannels(r *Registry, client scmintegrationv1.ScmIntegrationServiceClient) {
	registerGitHubChannels(r, client)
	registerGitHubProjectChannels(r, client)
	registerGitLabChannels(r, client)
	registerHostedReviewChannels(r, client)
}

// ── github.* (PR/issue mutations, repo/branch resolution, auth, rate limit) ──

func registerGitHubChannels(r *Registry, client scmintegrationv1.ScmIntegrationServiceClient) {
	// github.rateLimit — real backing RPC already exists (BUG-012's
	// finding); this is the wiring-only piece.
	r.Register("github.rateLimit", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.GetRateLimitStatus(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.GetRateLimitStatusRequest{TenantId: id.TenantID, Provider: scmintegrationv1.ScmProvider_SCM_PROVIDER_GITHUB})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.mergePR", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type mergeArgs struct {
			Repo          string `json:"repo"`
			Number        int32  `json:"number"`
			MergeMethod   string `json:"mergeMethod"`
			CommitTitle   string `json:"commitTitle"`
			CommitMessage string `json:"commitMessage"`
		}
		in, err := decodeArg[mergeArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.MergePullRequest(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.MergePullRequestRequest{
				TenantId: id.TenantID, Provider: scmintegrationv1.ScmProvider_SCM_PROVIDER_GITHUB,
				Repo: in.Repo, Number: in.Number, MergeMethod: in.MergeMethod,
				CommitTitle: in.CommitTitle, CommitMessage: in.CommitMessage,
			})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.requestPRReviewers", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type reqArgs struct {
			Repo           string   `json:"repo"`
			Number         int32    `json:"number"`
			ReviewerLogins []string `json:"reviewerLogins"`
			TeamSlugs      []string `json:"teamSlugs"`
		}
		in, err := decodeArg[reqArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.RequestPullRequestReviewers(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.RequestPullRequestReviewersRequest{
				TenantId: id.TenantID, Provider: scmintegrationv1.ScmProvider_SCM_PROVIDER_GITHUB,
				Repo: in.Repo, Number: in.Number, ReviewerLogins: in.ReviewerLogins, TeamSlugs: in.TeamSlugs,
			})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.removePRReviewers", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type removeArgs struct {
			Repo           string   `json:"repo"`
			Number         int32    `json:"number"`
			ReviewerLogins []string `json:"reviewerLogins"`
		}
		in, err := decodeArg[removeArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.RemovePullRequestReviewers(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.RemovePullRequestReviewersRequest{
				TenantId: id.TenantID, Provider: scmintegrationv1.ScmProvider_SCM_PROVIDER_GITHUB,
				Repo: in.Repo, Number: in.Number, ReviewerLogins: in.ReviewerLogins,
			})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.setPRAutoMerge", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type autoMergeArgs struct {
			Repo        string `json:"repo"`
			Number      int32  `json:"number"`
			Enabled     bool   `json:"enabled"`
			MergeMethod string `json:"mergeMethod"`
		}
		in, err := decodeArg[autoMergeArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.SetPullRequestAutoMerge(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.SetPullRequestAutoMergeRequest{
				TenantId: id.TenantID, Provider: scmintegrationv1.ScmProvider_SCM_PROVIDER_GITHUB,
				Repo: in.Repo, Number: in.Number, Enabled: in.Enabled, MergeMethod: in.MergeMethod,
			})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.updateIssue", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateIssueArgs struct {
			Repo         string   `json:"repo"`
			Number       int32    `json:"number"`
			Title        *string  `json:"title"`
			Body         *string  `json:"body"`
			State        *string  `json:"state"`
			AddLabels    []string `json:"addLabels"`
			RemoveLabels []string `json:"removeLabels"`
			Assignees    []string `json:"assignees"`
		}
		in, err := decodeArg[updateIssueArgs](args, 0)
		if err != nil {
			return nil, err
		}
		req := &scmintegrationv1.UpdateIssueRequest{
			TenantId: id.TenantID, Provider: scmintegrationv1.ScmProvider_SCM_PROVIDER_GITHUB,
			Repo: in.Repo, Number: in.Number,
			AddLabels: in.AddLabels, RemoveLabels: in.RemoveLabels, Assignees: in.Assignees,
		}
		if in.Title != nil {
			req.Title = in.Title
		}
		if in.Body != nil {
			req.Body = in.Body
		}
		if in.State != nil {
			req.State = in.State
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.UpdateIssue(attachSCMIdentity(rpcCtx, id), req)
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.prForBranch", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type forBranchArgs struct {
			Repo       string `json:"repo"`
			HeadBranch string `json:"headBranch"`
		}
		in, err := decodeArg[forBranchArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.GetPullRequestForBranch(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.GetPullRequestForBranchRequest{
				TenantId: id.TenantID, Provider: scmintegrationv1.ScmProvider_SCM_PROVIDER_GITHUB,
				Repo: in.Repo, HeadBranch: in.HeadBranch,
			})
		if err != nil {
			return nil, err
		}
		if !resp.GetFound() {
			return nil, nil
		}
		return resp.GetPullRequest(), nil
	})

	r.Register("github.repoSlug", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type slugArgs struct {
			Candidate string `json:"candidate"`
		}
		in, err := decodeArg[slugArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.ResolveRepoSlug(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.ResolveRepoSlugRequest{
				TenantId: id.TenantID, Provider: scmintegrationv1.ScmProvider_SCM_PROVIDER_GITHUB, Candidate: in.Candidate,
			})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	// github.startAuthLogin / github.revokeAuth — thin wrappers over the
	// OAuth RPCs BUG-012 confirmed already exist; no new proto for these two.
	r.Register("github.startAuthLogin", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type startArgs struct {
			RedirectURI string `json:"redirectUri"`
		}
		in, err := decodeArg[startArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.StartOAuthFlow(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.StartOAuthFlowRequest{
				TenantId: id.TenantID, UserId: id.UserID,
				Provider: scmintegrationv1.ScmProvider_SCM_PROVIDER_GITHUB, RedirectUri: in.RedirectURI,
			})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.revokeAuth", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.RevokeAuth(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.RevokeAuthRequest{TenantId: id.TenantID, Provider: scmintegrationv1.ScmProvider_SCM_PROVIDER_GITHUB})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
}

// ── github.project.* (GitHub Projects v2) ────────────────────────────────
//
// No provider field on any request message here (TASK-077): this whole
// sub-surface is GitHub-only by construction, unlike every handler above.

func registerGitHubProjectChannels(r *Registry, client scmintegrationv1.ScmIntegrationServiceClient) {
	r.Register("github.project.listAccessible", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.ListAccessibleProjects(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.ListAccessibleProjectsRequest{TenantId: id.TenantID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.resolveRef", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type resolveArgs struct {
			Owner  string `json:"owner"`
			Number int32  `json:"number"`
		}
		in, err := decodeArg[resolveArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.ResolveProjectRef(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.ResolveProjectRefRequest{TenantId: id.TenantID, Owner: in.Owner, Number: in.Number})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.listViews", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type viewsArgs struct {
			ProjectSlug string `json:"projectSlug"`
		}
		in, err := decodeArg[viewsArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.ListProjectViews(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.ListProjectViewsRequest{TenantId: id.TenantID, ProjectSlug: in.ProjectSlug})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.viewTable", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type tableArgs struct {
			ProjectSlug string `json:"projectSlug"`
			ViewID      string `json:"viewId"`
			PageToken   string `json:"pageToken"`
			PageSize    int32  `json:"pageSize"`
		}
		in, err := decodeArg[tableArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.ViewProjectTable(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.ViewProjectTableRequest{
				TenantId: id.TenantID, ProjectSlug: in.ProjectSlug, ViewId: in.ViewID, PageToken: in.PageToken, PageSize: in.PageSize,
			})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.updateItemField", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateFieldArgs struct {
			ProjectSlug string `json:"projectSlug"`
			ItemID      string `json:"itemId"`
			FieldID     string `json:"fieldId"`
			Kind        string `json:"kind"`
			Value       string `json:"value"`
		}
		in, err := decodeArg[updateFieldArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.UpdateProjectItemField(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.UpdateProjectItemFieldRequest{
				TenantId: id.TenantID, ProjectSlug: in.ProjectSlug, ItemId: in.ItemID,
				Field: &scmintegrationv1.ProjectFieldValue{FieldId: in.FieldID, Kind: in.Kind, Value: in.Value},
			})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.clearItemField", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type clearFieldArgs struct {
			ProjectSlug string `json:"projectSlug"`
			ItemID      string `json:"itemId"`
			FieldID     string `json:"fieldId"`
		}
		in, err := decodeArg[clearFieldArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.ClearProjectItemField(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.ClearProjectItemFieldRequest{TenantId: id.TenantID, ProjectSlug: in.ProjectSlug, ItemId: in.ItemID, FieldId: in.FieldID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.workItemDetailsBySlug", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type slugArgs struct {
			ItemSlug string `json:"itemSlug"`
		}
		in, err := decodeArg[slugArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.GetWorkItemDetailsBySlug(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.GetWorkItemDetailsBySlugRequest{TenantId: id.TenantID, ItemSlug: in.ItemSlug})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.updateIssueBySlug", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			ItemSlug     string   `json:"itemSlug"`
			Title        *string  `json:"title"`
			Body         *string  `json:"body"`
			State        *string  `json:"state"`
			AddLabels    []string `json:"addLabels"`
			RemoveLabels []string `json:"removeLabels"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		req := &scmintegrationv1.UpdateIssueBySlugRequest{TenantId: id.TenantID, ItemSlug: in.ItemSlug, AddLabels: in.AddLabels, RemoveLabels: in.RemoveLabels}
		if in.Title != nil {
			req.Title = in.Title
		}
		if in.Body != nil {
			req.Body = in.Body
		}
		if in.State != nil {
			req.State = in.State
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.UpdateIssueBySlug(attachSCMIdentity(rpcCtx, id), req)
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.updatePullRequestBySlug", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			ItemSlug string  `json:"itemSlug"`
			Title    *string `json:"title"`
			Body     *string `json:"body"`
			State    *string `json:"state"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		req := &scmintegrationv1.UpdatePullRequestBySlugRequest{TenantId: id.TenantID, ItemSlug: in.ItemSlug}
		if in.Title != nil {
			req.Title = in.Title
		}
		if in.Body != nil {
			req.Body = in.Body
		}
		if in.State != nil {
			req.State = in.State
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.UpdatePullRequestBySlug(attachSCMIdentity(rpcCtx, id), req)
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.updateIssueTypeBySlug", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type typeArgs struct {
			ItemSlug  string `json:"itemSlug"`
			IssueType string `json:"issueType"`
		}
		in, err := decodeArg[typeArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.UpdateIssueTypeBySlug(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.UpdateIssueTypeBySlugRequest{TenantId: id.TenantID, ItemSlug: in.ItemSlug, IssueType: in.IssueType})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.listIssueTypesBySlug", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type slugArgs struct {
			ItemSlug string `json:"itemSlug"`
		}
		in, err := decodeArg[slugArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.ListIssueTypesBySlug(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.ListIssueTypesBySlugRequest{TenantId: id.TenantID, ItemSlug: in.ItemSlug})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.listAssignableUsersBySlug", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type slugArgs struct {
			ItemSlug string `json:"itemSlug"`
		}
		in, err := decodeArg[slugArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.ListAssignableUsersBySlug(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.ListAssignableUsersBySlugRequest{TenantId: id.TenantID, ItemSlug: in.ItemSlug})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.listLabelsBySlug", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type slugArgs struct {
			ItemSlug string `json:"itemSlug"`
		}
		in, err := decodeArg[slugArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.ListLabelsBySlug(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.ListLabelsBySlugRequest{TenantId: id.TenantID, ItemSlug: in.ItemSlug})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.addIssueCommentBySlug", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type commentArgs struct {
			ItemSlug string `json:"itemSlug"`
			Body     string `json:"body"`
		}
		in, err := decodeArg[commentArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.AddIssueCommentBySlug(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.AddIssueCommentBySlugRequest{TenantId: id.TenantID, ItemSlug: in.ItemSlug, Body: in.Body})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.updateIssueCommentBySlug", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type commentArgs struct {
			ItemSlug  string `json:"itemSlug"`
			CommentID string `json:"commentId"`
			Body      string `json:"body"`
		}
		in, err := decodeArg[commentArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.UpdateIssueCommentBySlug(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.UpdateIssueCommentBySlugRequest{TenantId: id.TenantID, ItemSlug: in.ItemSlug, CommentId: in.CommentID, Body: in.Body})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("github.project.deleteIssueCommentBySlug", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type commentArgs struct {
			ItemSlug  string `json:"itemSlug"`
			CommentID string `json:"commentId"`
		}
		in, err := decodeArg[commentArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		_, err = client.DeleteIssueCommentBySlug(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.DeleteIssueCommentBySlugRequest{TenantId: id.TenantID, ItemSlug: in.ItemSlug, CommentId: in.CommentID})
		if err != nil {
			return nil, err
		}
		return nil, nil
	})
}

// ── gitlab.* ──────────────────────────────────────────────────────────────

func registerGitLabChannels(r *Registry, client scmintegrationv1.ScmIntegrationServiceClient) {
	// gitlab.rateLimit — real backing RPC already exists (BUG-013's
	// finding), provider-generic, same as github.rateLimit.
	r.Register("gitlab.rateLimit", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.GetRateLimitStatus(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.GetRateLimitStatusRequest{TenantId: id.TenantID, Provider: scmintegrationv1.ScmProvider_SCM_PROVIDER_GITLAB})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("gitlab.listMRs", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listArgs struct {
			Repo         string `json:"repo"`
			State        string `json:"state"`
			SourceBranch string `json:"sourceBranch"`
		}
		in, err := decodeArg[listArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.ListMergeRequests(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.ListMergeRequestsRequest{
				TenantId: id.TenantID, Repo: in.Repo, State: in.State, SourceBranch: in.SourceBranch,
			})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("gitlab.resolveMRDiscussion", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type resolveArgs struct {
			Repo            string `json:"repo"`
			MergeRequestIID int32  `json:"mergeRequestIid"`
			DiscussionID    string `json:"discussionId"`
			Resolved        bool   `json:"resolved"`
		}
		in, err := decodeArg[resolveArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.ResolveMergeRequestDiscussion(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.ResolveMergeRequestDiscussionRequest{
				TenantId: id.TenantID, Repo: in.Repo, MergeRequestIid: in.MergeRequestIID,
				DiscussionId: in.DiscussionID, Resolved: in.Resolved,
			})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("gitlab.workItemDetails", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type detailsArgs struct {
			Repo     string `json:"repo"`
			IID      int32  `json:"iid"`
			ItemType string `json:"itemType"`
		}
		in, err := decodeArg[detailsArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.GetWorkItemDetails(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.GetWorkItemDetailsRequest{
				TenantId: id.TenantID, Repo: in.Repo, Iid: in.IID, ItemType: in.ItemType,
			})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	// gitlab.startAuthLogin / gitlab.revokeAuth — same thin OAuth wrapper as
	// github.startAuthLogin/revokeAuth, SCM_PROVIDER_GITLAB instead of
	// SCM_PROVIDER_GITHUB.
	r.Register("gitlab.startAuthLogin", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type startArgs struct {
			RedirectURI string `json:"redirectUri"`
		}
		in, err := decodeArg[startArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.StartOAuthFlow(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.StartOAuthFlowRequest{
				TenantId: id.TenantID, UserId: id.UserID,
				Provider: scmintegrationv1.ScmProvider_SCM_PROVIDER_GITLAB, RedirectUri: in.RedirectURI,
			})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("gitlab.revokeAuth", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.RevokeAuth(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.RevokeAuthRequest{TenantId: id.TenantID, Provider: scmintegrationv1.ScmProvider_SCM_PROVIDER_GITLAB})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
}

// ── hostedReview.* ────────────────────────────────────────────────────────

func registerHostedReviewChannels(r *Registry, client scmintegrationv1.ScmIntegrationServiceClient) {
	r.Register("hostedReview.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			Provider   string `json:"provider"`
			Repo       string `json:"repo"`
			Title      string `json:"title"`
			Body       string `json:"body"`
			HeadBranch string `json:"headBranch"`
			BaseBranch string `json:"baseBranch"`
			RequestID  string `json:"requestId"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.CreatePullRequest(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.CreatePullRequestRequest{
				TenantId: id.TenantID, Provider: parseWSProvider(in.Provider),
				Repo: in.Repo, Title: in.Title, Body: in.Body,
				HeadBranch: in.HeadBranch, BaseBranch: in.BaseBranch, RequestId: in.RequestID,
			})
		if err != nil {
			return nil, err
		}
		return resp.GetPullRequest(), nil
	})

	// hostedReview.forBranch — uses the same provider-generic single-result
	// GetPullRequestForBranch RPC github.prForBranch uses, just with an
	// explicit Provider instead of a hardcoded one.
	r.Register("hostedReview.forBranch", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type forBranchArgs struct {
			Provider   string `json:"provider"`
			Repo       string `json:"repo"`
			HeadBranch string `json:"headBranch"`
		}
		in, err := decodeArg[forBranchArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.GetPullRequestForBranch(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.GetPullRequestForBranchRequest{
				TenantId: id.TenantID, Provider: parseWSProvider(in.Provider),
				Repo: in.Repo, HeadBranch: in.HeadBranch,
			})
		if err != nil {
			return nil, err
		}
		if !resp.GetFound() {
			return nil, nil
		}
		return resp.GetPullRequest(), nil
	})

	r.Register("hostedReview.getCreationEligibility", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type eligibilityArgs struct {
			Provider   string `json:"provider"`
			Repo       string `json:"repo"`
			HeadBranch string `json:"headBranch"`
			BaseBranch string `json:"baseBranch"`
		}
		in, err := decodeArg[eligibilityArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.CheckHostedReviewEligibility(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.CheckHostedReviewEligibilityRequest{
				TenantId: id.TenantID, Provider: parseWSProvider(in.Provider),
				Repo: in.Repo, HeadBranch: in.HeadBranch, BaseBranch: in.BaseBranch,
			})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
}

// parseWSProvider mirrors httpgateway.parseSCMProvider (scm_routes.go) —
// duplicated rather than imported since wscompat and httpgateway are
// separate adapter packages per 03-clean-architecture-guidelines.md's
// layering (both are "adapter", neither should depend on the other); a
// future cleanup could hoist this into a small shared internal package if a
// third caller appears, but two isn't yet a pattern.
func parseWSProvider(v string) scmintegrationv1.ScmProvider {
	switch v {
	case "github":
		return scmintegrationv1.ScmProvider_SCM_PROVIDER_GITHUB
	case "gitlab":
		return scmintegrationv1.ScmProvider_SCM_PROVIDER_GITLAB
	case "bitbucket":
		return scmintegrationv1.ScmProvider_SCM_PROVIDER_BITBUCKET
	case "azure_devops":
		return scmintegrationv1.ScmProvider_SCM_PROVIDER_AZURE_DEVOPS
	case "gitea":
		return scmintegrationv1.ScmProvider_SCM_PROVIDER_GITEA
	default:
		return scmintegrationv1.ScmProvider_SCM_PROVIDER_UNSPECIFIED
	}
}
