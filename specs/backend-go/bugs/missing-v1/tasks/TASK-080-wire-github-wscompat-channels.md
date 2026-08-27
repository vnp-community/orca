# TASK-080: Wire `github.*` wscompat channels (PR/issue mutations, repo/branch, auth, rate limit)

**From Solution:** SOL-012 (Design — `wscompat` channel wiring)
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels_github.go` (new), `channels.go`, `cmd/server/main.go`
**Depends on:** TASK-076
**Status:** `[x]` DONE — implemented in worktree `agent-aac2382028c6ce920` (branch `worktree-agent-aac2382028c6ce920`), **committed** as `ce750c490`. `go build`/`go vet`/`gofmt -l` clean, `buf generate`/`buf breaking` clean (additive-only). Pending merge to main + one-line RegisterRealChannels/main.go wiring.

---

## Context

New file, following `channels.go`'s existing `register*Channels(r *Registry,
client ...)` pattern exactly (decode `args[0]`, call the gRPC client, map
the response, return — same shape TASK-012's push-bridge primitives and
`channels.go`'s existing `registerAnnotationChannels` use). Covers every
`github.*` channel except `github.project.*` (TASK-081, same file,
appended). `main.go` already dials `scm-integration-service`'s gRPC client
for REST routes (`scmClient` in `router.go`'s deps) — this task reuses that
same client, no second dial.

---

## Changes to make

### Step 1: New file `channels_github.go`

**File:** `services/api-gateway/internal/adapter/wscompat/channels_github.go`

```go
// Channel handlers backing the frontend's github.* namespace — see
// specs/backend-go/bugs/missing-v1/solutions/SOL-012-github-channels.md.
// github.project.* channels are appended in a separate pass (TASK-081) to
// keep this file focused on the PR/issue-mutation, repo/branch-resolution,
// auth, and rate-limit channels.
package wscompat

import (
	"context"
	"encoding/json"

	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// attachGitHubIdentity is a one-line convenience every handler below uses —
// same AttachIdentity call channels.go's existing handlers make, just
// spelled out once here since every github.* handler needs it.
func attachGitHubIdentity(ctx context.Context, id Identity) context.Context {
	return gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
}

func registerGitHubChannels(r *Registry, client scmintegrationv1.ScmIntegrationServiceClient) {
	// github.rateLimit — real backing RPC already exists (BUG-012's
	// finding); this is the wiring-only piece.
	r.Register("github.rateLimit", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.GetRateLimitStatus(attachGitHubIdentity(rpcCtx, id),
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
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.MergePullRequest(attachGitHubIdentity(rpcCtx, id),
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
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.RequestPullRequestReviewers(attachGitHubIdentity(rpcCtx, id),
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
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.RemovePullRequestReviewers(attachGitHubIdentity(rpcCtx, id),
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
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.SetPullRequestAutoMerge(attachGitHubIdentity(rpcCtx, id),
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
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.UpdateIssue(attachGitHubIdentity(rpcCtx, id), req)
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
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.GetPullRequestForBranch(attachGitHubIdentity(rpcCtx, id),
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
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ResolveRepoSlug(attachGitHubIdentity(rpcCtx, id),
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
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.StartOAuthFlow(attachGitHubIdentity(rpcCtx, id),
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
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.RevokeAuth(attachGitHubIdentity(rpcCtx, id),
			&scmintegrationv1.RevokeAuthRequest{TenantId: id.TenantID, Provider: scmintegrationv1.ScmProvider_SCM_PROVIDER_GITHUB})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
}
```

### Step 2: Wire into `RegisterRealChannels`

**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`

Find:

```go
func RegisterRealChannels(
	r *Registry,
	annotationClient annotationv1.AnnotationServiceClient,
	taskClient taskv1.TaskServiceClient,
	gitClient gitgatewayv1.GitGatewayServiceClient,
	automationClient automationv1.AutomationServiceClient,
	infraFleetClient infrafleetv1.InfraFleetServiceClient,
	rateLimits rateLimitReader,
) {
	registerAnnotationChannels(r, annotationClient)
	registerTaskChannels(r, taskClient)
	registerGitChannels(r, gitClient)
	registerAutomationChannels(r, automationClient)
	registerPreflightChannels(r)
	registerDevServerChannels(r, infraFleetClient)
	registerFleetChannels(r, infraFleetClient)
	registerCrashReportChannels(r)
	registerRateLimitChannels(r, rateLimits)
}
```

Replace with:

```go
func RegisterRealChannels(
	r *Registry,
	annotationClient annotationv1.AnnotationServiceClient,
	taskClient taskv1.TaskServiceClient,
	gitClient gitgatewayv1.GitGatewayServiceClient,
	automationClient automationv1.AutomationServiceClient,
	infraFleetClient infrafleetv1.InfraFleetServiceClient,
	rateLimits rateLimitReader,
	scmClient scmintegrationv1.ScmIntegrationServiceClient,
) {
	registerAnnotationChannels(r, annotationClient)
	registerTaskChannels(r, taskClient)
	registerGitChannels(r, gitClient)
	registerAutomationChannels(r, automationClient)
	registerPreflightChannels(r)
	registerDevServerChannels(r, infraFleetClient)
	registerFleetChannels(r, infraFleetClient)
	registerCrashReportChannels(r)
	registerRateLimitChannels(r, rateLimits)
	registerGitHubChannels(r, scmClient)
}
```

Add the import to `channels.go`'s import block:

```go
	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"
```

### Step 3: Pass `scmClient` from `main.go`

**File:** `services/api-gateway/cmd/server/main.go`

Find:

```go
	wscompat.RegisterRealChannels(wsCompatRegistry, annotationClient, taskClient, gitClient, automationClient, infraFleetClient, rateLimiter)
```

Replace with:

```go
	wscompat.RegisterRealChannels(wsCompatRegistry, annotationClient, taskClient, gitClient, automationClient, infraFleetClient, rateLimiter, scmClient)
```

`scmClient` is already dialed earlier in `main.go` (`scmClient :=
scmintegrationv1.NewScmIntegrationServiceClient(scmConn)`, used today only
by `httpgateway`'s REST routes) — reused here, no second dial. Verify the
`wscompat.RegisterRealChannels(...)` call site is textually **after**
`scmClient`'s declaration in `main.go`; if not, no code change is needed
beyond reordering (Go requires the variable to exist before use, not
necessarily declared immediately above — check with `go build` in Verify).

---

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./...
```

Expected: clean build. `go vet` catches any unused-import or ordering issue
from Step 3 immediately.
