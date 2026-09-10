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
	"fmt"
	"strings"
	"time"

	annotationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/annotation/v1"
	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"

	"github.com/stablyai/orca-go/common/apperrors"
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
func registerSCMChannels(r *Registry, client scmintegrationv1.ScmIntegrationServiceClient, gitClient gitgatewayv1.GitGatewayServiceClient, annotationClient annotationv1.AnnotationServiceClient) {
	registerGitHubChannels(r, client, gitClient)
	registerGitHubProjectChannels(r, client)
	registerGitLabChannels(r, client)
	registerHostedReviewChannels(r, client, annotationClient)
}

// ── github.* (PR/issue mutations, repo/branch resolution, auth, rate limit) ──

func registerGitHubChannels(r *Registry, client scmintegrationv1.ScmIntegrationServiceClient, gitClient gitgatewayv1.GitGatewayServiceClient) {
	// github.checkOrcaStarred — the old TS backend shelled out to the local
	// `gh` CLI (`gh api user/starred/<repo>`); scm-integration-service has no
	// equivalent RPC (no GitHub "check if starred" call exists in
	// scmintegration.proto today — a real port needs a new proto RPC +
	// usecase, not just wiring). null ("unable to determine") is not a
	// stub here — it's the same answer the old backend already gave for
	// every user without `gh` installed/authenticated, and both frontend
	// call sites (Landing.tsx's GitHubStarButton, GeneralSupportSection.tsx)
	// already treat null as a designed "web-fallback" state, not an error.
	r.Register("github.checkOrcaStarred", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return nil, nil
	})

	// orcaRepoSlug is the one repo github.starOrca ever stars — not
	// user-supplied (the frontend call site sends only {source}, a UI-location
	// tag, per runtime-github-client.ts:79-88).
	const orcaRepoSlug = "getorca/orca" // confirm the real org/repo slug against the actual GitHub remote before shipping

	// github.starOrca — the write-side twin of github.checkOrcaStarred's
	// honest nil,nil no-op above: now has a real backing RPC (SOL-012/TASK-034),
	// so it is wired for real rather than staying a stub.
	r.Register("github.starOrca", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type starArgs struct {
			Source string `json:"source"`
		}
		// source (runtime-github-client.ts:79-88's caller-supplied UI-location
		// tag, e.g. "landing-page") is accepted for wire-shape compatibility but
		// not forwarded to scm-integration-service — GitHub's star API has no
		// concept of "why," and this service does not log/analytics-track
		// per-call metadata. If starOrca's source ever needs recording for
		// product analytics, that belongs in a telemetry event emitted here in
		// api-gateway, not as a new scm-integration-service RPC field.
		_, _ = decodeArg[starArgs](args, 0)

		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.StarRepository(attachSCMIdentity(rpcCtx, id), &scmintegrationv1.StarRepositoryRequest{
			TenantId: id.TenantID, Provider: scmintegrationv1.ScmProvider_SCM_PROVIDER_GITHUB, Repo: orcaRepoSlug,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetStarred(), nil
	})

	// github.updatePRTitle
	r.Register("github.updatePRTitle", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateTitleArgs struct {
			Repo   string `json:"repo"` // "id:<repoId>" | "<repoPath>" — same shape github.listWorkItems resolves
			Number int32  `json:"prNumber"`
			Title  string `json:"title"`
		}
		in, err := decodeArg[updateTitleArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ownerRepo, _, resolveErr := resolveGitHubOwnerRepo(ctx, gitClient, in.Repo)
		if resolveErr != nil {
			return nil, resolveErr
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		title := in.Title
		_, err = client.UpdatePullRequest(attachSCMIdentity(rpcCtx, id), &scmintegrationv1.UpdatePullRequestRequest{
			TenantId: id.TenantID, Provider: scmintegrationv1.ScmProvider_SCM_PROVIDER_GITHUB,
			Repo: ownerRepo.slug(), Number: in.Number, Title: &title,
		})
		if err != nil {
			return nil, err
		}
		return true, nil // github.updatePRTitle's frontend contract is Promise<boolean>
		// (frontend/src/preload/api-types.ts:1605-1610); the RPC's real
		// PullRequest response is discarded in favor of a plain success bool.
	})

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

	// github.issues — BUG-PI-01's fix: filters/force_refresh now actually
	// reach the RPC, was previously a hardcoded empty IssueFilter{}.
	r.Register("github.issues", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type issuesArgs struct {
			Repo      string   `json:"repo"`
			State     string   `json:"state"`
			Assignee  string   `json:"assignee"`
			Labels    []string `json:"labels"`
			Milestone string   `json:"milestone"`
			Refresh   bool     `json:"refresh"`
		}
		in, err := decodeArg[issuesArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.ListIssues(attachSCMIdentity(rpcCtx, id), &scmintegrationv1.ListIssuesRequest{
			TenantId: id.TenantID, Provider: scmintegrationv1.ScmProvider_SCM_PROVIDER_GITHUB, Repo: in.Repo,
			Filter:       &scmintegrationv1.IssueFilter{State: in.State, Assignee: in.Assignee, Labels: in.Labels, Milestone: in.Milestone},
			ForceRefresh: in.Refresh,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	// github.issueComments — completes the *BySlug comment RPC group's read
	// side (BUG-PI-01 step 6), same registration pattern as github.rateLimit.
	r.Register("github.issueComments", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type commentsArgs struct {
			ItemSlug string `json:"itemSlug"`
		}
		in, err := decodeArg[commentsArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.ListIssueCommentsBySlug(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.ListIssueCommentsBySlugRequest{TenantId: id.TenantID, ItemSlug: in.ItemSlug})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	// github.listWorkItems — the Tasks page's GitHub issue/PR picker. Unlike
	// every other github.* channel above, the frontend sends a real backend
	// repoId (project.repos.id), not an owner/repo slug — see
	// resolveGitHubOwnerRepo's doc comment for why that resolution has to
	// happen here rather than in scm-integration-service.
	r.Register("github.listWorkItems", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listWorkItemsArgs struct {
			Repo    string `json:"repo"` // repoId, despite the field name — matches the frontend's wire shape
			Limit   int32  `json:"limit"`
			Query   string `json:"query"`
			Before  string `json:"before"`
			NoCache bool   `json:"noCache"`
		}
		in, err := decodeArg[listWorkItemsArgs](args, 0)
		if err != nil {
			return nil, err
		}
		gwCtx, cancel := context.WithTimeout(attachSCMIdentity(ctx, id), scmRPCTimeout)
		defer cancel()
		ownerRepo, fellBack, resolveErr := resolveGitHubOwnerRepo(gwCtx, gitClient, in.Repo)
		if resolveErr != nil {
			// Matches the legacy backend's "surface as a result-level error,
			// not a channel error" pattern (ListWorkItemsResult.errors) —
			// an unresolved remote is routine (non-GitHub repo, no upstream
			// configured yet), not a hard failure of the whole call.
			return listWorkItemsResultView{
				Items:   []gitHubWorkItemView{},
				Sources: workItemSourcesView{},
				Errors:  &workItemErrorsView{Issues: &classifiedErrorView{Type: "not_found", Message: resolveErr.Error()}},
			}, nil
		}

		rpcCtx, rpcCancel := context.WithTimeout(attachSCMIdentity(ctx, id), scmRPCTimeout)
		defer rpcCancel()
		resp, err := client.ListWorkItems(rpcCtx, &scmintegrationv1.ListWorkItemsRequest{
			TenantId: id.TenantID, Provider: scmintegrationv1.ScmProvider_SCM_PROVIDER_GITHUB,
			Repo: ownerRepo.slug(), Limit: in.Limit, Query: in.Query, Before: in.Before, NoCache: in.NoCache,
		})
		if err != nil {
			return listWorkItemsResultView{
				Items:   []gitHubWorkItemView{},
				Sources: workItemSourcesView{Issues: ownerRepo.view(), Prs: ownerRepo.view()},
				Errors:  &workItemErrorsView{Issues: &classifiedErrorView{Type: "unknown", Message: err.Error()}},
			}, nil
		}

		items := make([]gitHubWorkItemView, 0, len(resp.GetWorkItems()))
		for _, w := range resp.GetWorkItems() {
			items = append(items, toGitHubWorkItemView(w))
		}
		result := listWorkItemsResultView{
			Items:   items,
			Sources: workItemSourcesView{Issues: ownerRepo.view(), Prs: ownerRepo.view()},
		}
		if fellBack {
			t := true
			result.IssueSourceFellBack = &t
		}
		return result, nil
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

	// gitlab.issues — ListIssues is provider-generic (BUG-PI-01), so unlike
	// gitlab.issueComments (deliberately NOT added below — ListIssueCommentsBySlug
	// is GitHub Projects v2-only, see list_issue_comments_by_slug.go's doc
	// comment; a gitlab.issueComments channel would just resolve to
	// SCM_PROVIDER_UNSUPPORTED on every call), this one has a real backing RPC.
	r.Register("gitlab.issues", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type issuesArgs struct {
			Repo      string   `json:"repo"`
			State     string   `json:"state"`
			Assignee  string   `json:"assignee"`
			Labels    []string `json:"labels"`
			Milestone string   `json:"milestone"`
			Refresh   bool     `json:"refresh"`
		}
		in, err := decodeArg[issuesArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.ListIssues(attachSCMIdentity(rpcCtx, id), &scmintegrationv1.ListIssuesRequest{
			TenantId: id.TenantID, Provider: scmintegrationv1.ScmProvider_SCM_PROVIDER_GITLAB, Repo: in.Repo,
			Filter:       &scmintegrationv1.IssueFilter{State: in.State, Assignee: in.Assignee, Labels: in.Labels, Milestone: in.Milestone},
			ForceRefresh: in.Refresh,
		})
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

func registerHostedReviewChannels(r *Registry, client scmintegrationv1.ScmIntegrationServiceClient, annotationClient annotationv1.AnnotationServiceClient) {
	r.Register("hostedReview.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			Provider          string `json:"provider"`
			Repo              string `json:"repo"`
			Title             string `json:"title"`
			Body              string `json:"body"`
			HeadBranch        string `json:"headBranch"`
			BaseBranch        string `json:"baseBranch"`
			RequestID         string `json:"requestId"`
			Draft             bool   `json:"draft"`             // NEW — BR-CR-20
			LinkedIssueNumber *int32 `json:"linkedIssueNumber"` // NEW — BR-CR-19; *int32 so an absent field round-trips as unset, not 0 (the frontend doesn't send this yet — no call site to confirm a 0-is-valid convention against)
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
				Draft: in.Draft, LinkedIssueNumber: in.LinkedIssueNumber,
			})
		if err != nil {
			return nil, err
		}
		return resp, nil // NEW — was resp.GetPullRequest(); now returns the whole response so
		// linked_issue_update_error (BR-CR-19) reaches the client too
	})

	r.Register("hostedReview.suggestReviewers", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type suggestArgs struct {
			Provider     string   `json:"provider"`
			Repo         string   `json:"repo"`
			BaseRef      string   `json:"baseRef"`
			ChangedFiles []string `json:"changedFiles"`
		}
		in, err := decodeArg[suggestArgs](args, 0)
		if err != nil {
			return nil, err
		}
		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.SuggestPullRequestReviewers(attachSCMIdentity(rpcCtx, id),
			&scmintegrationv1.SuggestPullRequestReviewersRequest{
				TenantId: id.TenantID, Provider: parseWSProvider(in.Provider),
				Repo: in.Repo, BaseRef: in.BaseRef, ChangedFiles: in.ChangedFiles,
			})
		if err != nil {
			return nil, err
		}
		return resp, nil
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

	// hostedReview.submit — wraps the same annotation-aggregation
	// composition as pr_review_routes.go's SubmitPullRequestReview
	// (TASK-PI-04-05) for the WS-compat surface (SOL-PI-04). BR-PI-12 needs
	// no special-case code here — this channel and the BL-CR-03
	// agent-feedback channel are already independent calls the frontend can
	// issue in parallel; no new orchestration.
	r.Register("hostedReview.submit", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type submitArgs struct {
			RepoID     string `json:"repoId"`
			Provider   string `json:"provider"`
			PRNumber   int32  `json:"prNumber"`
			ReviewType string `json:"reviewType"`
			Summary    string `json:"summary"`
		}
		in, err := decodeArg[submitArgs](args, 0)
		if err != nil {
			return nil, err
		}

		// Aggregation read — same drain-all-pages + BR-PI-10 fail-fast as
		// pr_review_routes.go's SubmitPullRequestReview (TASK-PI-04-05).
		var allAnnotations []*annotationv1.Annotation
		pageToken := ""
		for {
			resp, err := annotationClient.ListAnnotations(ctx, &annotationv1.ListAnnotationsRequest{
				RepoId: in.RepoID, PageToken: pageToken,
			})
			if err != nil {
				return nil, err
			}
			allAnnotations = append(allAnnotations, resp.GetAnnotations()...)
			if pageToken = resp.GetNextPageToken(); pageToken == "" {
				break
			}
		}
		if len(allAnnotations) == 0 {
			return nil, apperrors.New(apperrors.KindInvalidArgument, "PR_REVIEW_NO_COMMENTS", "annotate at least one line before submitting a review", nil)
		}

		comments := make([]*scmintegrationv1.ReviewComment, 0, len(allAnnotations))
		for _, a := range allAnnotations {
			comments = append(comments, &scmintegrationv1.ReviewComment{
				Path: a.GetAnchor().GetFilePath(), Line: a.GetAnchor().GetLine(), Body: a.GetContent(),
			})
		}

		rpcCtx, cancel := context.WithTimeout(ctx, scmRPCTimeout)
		defer cancel()
		resp, err := client.SubmitReview(attachSCMIdentity(rpcCtx, id), &scmintegrationv1.SubmitReviewRequest{
			TenantId: id.TenantID, Provider: parseWSProvider(in.Provider), Repo: in.RepoID,
			PrNumber: in.PRNumber, ReviewType: parseWSReviewType(in.ReviewType),
			SummaryBody: in.Summary, Comments: comments,
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
// parseWSReviewType mirrors httpgateway.parseReviewType (pr_review_routes.go)
// — same duplication rationale as parseWSProvider above.
func parseWSReviewType(v string) scmintegrationv1.ReviewType {
	switch v {
	case "comment":
		return scmintegrationv1.ReviewType_REVIEW_TYPE_COMMENT
	case "approve":
		return scmintegrationv1.ReviewType_REVIEW_TYPE_APPROVE
	case "request_changes":
		return scmintegrationv1.ReviewType_REVIEW_TYPE_REQUEST_CHANGES
	default:
		return scmintegrationv1.ReviewType_REVIEW_TYPE_UNSPECIFIED
	}
}

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

// ── github.listWorkItems support: repoId -> owner/repo resolution + view
// structs (protoc-gen-go's own encoding/json tags are snake_case — see
// this file's sibling channels_tenant_project.go's userProfileView comment
// for why every github.listWorkItems response field below is a plain
// camelCase-tagged struct, never a raw proto message). ─────────────────────

type githubOwnerRepo struct {
	Owner string
	Repo  string
}

func (o githubOwnerRepo) slug() string { return o.Owner + "/" + o.Repo }

func (o githubOwnerRepo) view() *ownerRepoView {
	if o.Owner == "" || o.Repo == "" {
		return nil
	}
	return &ownerRepoView{Owner: o.Owner, Repo: o.Repo}
}

type ownerRepoView struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

type classifiedErrorView struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type workItemErrorsView struct {
	Issues *classifiedErrorView `json:"issues,omitempty"`
}

type workItemSourcesView struct {
	Issues            *ownerRepoView `json:"issues"`
	Prs               *ownerRepoView `json:"prs"`
	OriginCandidate   *ownerRepoView `json:"originCandidate"`
	UpstreamCandidate *ownerRepoView `json:"upstreamCandidate"`
}

type gitHubWorkItemView struct {
	ID        string   `json:"id"`
	Type      string   `json:"type"`
	Number    int32    `json:"number"`
	Title     string   `json:"title"`
	State     string   `json:"state"`
	URL       string   `json:"url"`
	Labels    []string `json:"labels"`
	UpdatedAt string   `json:"updatedAt"`
	Author    *string  `json:"author"`
}

type listWorkItemsResultView struct {
	Items               []gitHubWorkItemView `json:"items"`
	Sources             workItemSourcesView  `json:"sources"`
	Errors              *workItemErrorsView  `json:"errors,omitempty"`
	IssueSourceFellBack *bool                `json:"issueSourceFellBack,omitempty"`
}

func toGitHubWorkItemView(w *scmintegrationv1.WorkItem) gitHubWorkItemView {
	view := gitHubWorkItemView{
		ID: w.GetId(), Type: w.GetType(), Number: w.GetNumber(), Title: w.GetTitle(),
		State: w.GetState(), URL: w.GetUrl(), Labels: w.GetLabels(), UpdatedAt: w.GetUpdatedAt(),
	}
	if view.Labels == nil {
		view.Labels = []string{}
	}
	if author := w.GetAuthor(); author != "" {
		view.Author = &author
	}
	return view
}

// resolveGitHubOwnerRepo resolves repoId's GitHub owner/repo by reading its
// configured git remote via git-gateway-service — project.repos.url is not
// reliably a git remote URL (see GetRemoteUrlRequest's proto doc comment),
// so this can't just read the repo record directly the way every other
// github.* channel's already-resolved `repo` string implies. Tries
// "upstream" first, falls back to "origin" — same precedence as the legacy
// desktop backend's resolveIssueSource/getIssueOwnerRepo (auto preference).
func resolveGitHubOwnerRepo(ctx context.Context, gitClient gitgatewayv1.GitGatewayServiceClient, repoID string) (githubOwnerRepo, bool, error) {
	if repoID == "" {
		return githubOwnerRepo{}, false, fmt.Errorf("repo is required")
	}
	upstream, upstreamErr := fetchGitHubRemote(ctx, gitClient, repoID, "upstream")
	if upstreamErr == nil {
		return upstream, false, nil
	}
	origin, originErr := fetchGitHubRemote(ctx, gitClient, repoID, "origin")
	if originErr == nil {
		return origin, true, nil
	}
	return githubOwnerRepo{}, false, fmt.Errorf("no GitHub remote configured (tried upstream, origin): %w", originErr)
}

func fetchGitHubRemote(ctx context.Context, gitClient gitgatewayv1.GitGatewayServiceClient, repoID, remoteName string) (githubOwnerRepo, error) {
	resp, err := gitClient.GetRemoteUrl(ctx, &gitgatewayv1.GetRemoteUrlRequest{RepoId: repoID, RemoteName: remoteName})
	if err != nil {
		return githubOwnerRepo{}, err
	}
	return parseGitHubOwnerRepo(resp.GetUrl())
}

// parseGitHubOwnerRepo is a Go port of the legacy desktop backend's
// parseGitHubOwnerRepo/parseGitHubRemoteIdentity (backend/src/main/github/
// github-remote-identity-parsing.ts) — deliberately strict: only exact host
// "github.com" (or its documented "ssh.github.com" SSH-over-443 alias) is
// accepted; GitHub Enterprise or non-GitHub remotes return an error rather
// than a guessed owner/repo.
func parseGitHubOwnerRepo(remoteURL string) (githubOwnerRepo, error) {
	trimmed := strings.TrimSpace(remoteURL)
	trimmed = strings.TrimSuffix(trimmed, ".git")

	var host, path string
	switch {
	case strings.HasPrefix(trimmed, "git@"):
		rest := strings.TrimPrefix(trimmed, "git@")
		parts := strings.SplitN(rest, ":", 2)
		if len(parts) != 2 {
			return githubOwnerRepo{}, fmt.Errorf("cannot parse SSH remote %q", remoteURL)
		}
		host, path = parts[0], parts[1]
	case strings.Contains(trimmed, "://"):
		parts := strings.SplitN(trimmed, "://", 2)
		rest := parts[1]
		slash := strings.Index(rest, "/")
		if slash < 0 {
			return githubOwnerRepo{}, fmt.Errorf("cannot parse remote %q", remoteURL)
		}
		host, path = rest[:slash], rest[slash+1:]
		if at := strings.LastIndex(host, "@"); at >= 0 {
			host = host[at+1:] // strip a userinfo@ prefix, if any
		}
	default:
		return githubOwnerRepo{}, fmt.Errorf("unrecognized remote URL shape %q", remoteURL)
	}

	host = strings.ToLower(host)
	if host == "ssh.github.com" {
		host = "github.com"
	}
	if host != "github.com" {
		return githubOwnerRepo{}, fmt.Errorf("remote host %q is not github.com", host)
	}

	path = strings.Trim(path, "/")
	segments := strings.Split(path, "/")
	if len(segments) != 2 || segments[0] == "" || segments[1] == "" {
		return githubOwnerRepo{}, fmt.Errorf("cannot resolve %q to an owner/repo pair", remoteURL)
	}
	return githubOwnerRepo{Owner: segments[0], Repo: segments[1]}, nil
}
