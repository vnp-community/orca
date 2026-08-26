# TASK-089: Wire `hostedReview.*` wscompat channels

**From Solution:** SOL-014 (Design — `wscompat` channel wiring)
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels_hostedreview.go` (new), `channels.go`
**Depends on:** TASK-080, TASK-085, TASK-088
**Status:** `[x]` DONE — implemented in worktree `agent-aac2382028c6ce920` (branch `worktree-agent-aac2382028c6ce920`), **committed** as `ce750c490`. `go build`/`go vet`/`gofmt -l` clean, `buf generate`/`buf breaking` clean (additive-only). Pending merge to main + one-line RegisterRealChannels/main.go wiring.

---

## Context

`hostedReview.*` calls take a `provider` argument explicitly (the frontend's
provider-agnostic namespace has no fixed provider to hardcode, unlike
`github.*`/`gitlab.*`), so this file adds a `parseWSProvider` string→enum
helper — a wscompat-local duplicate of `httpgateway.parseSCMProvider`
(`scm_routes.go`), not imported across the adapter-package boundary per
`03-clean-architecture-guidelines.md`. `hostedReview.forBranch` uses
TASK-072's `GetPullRequestForBranch` directly (SOL-012 is assumed
implemented first, per this repo's task ordering — TASK-072 through
TASK-081 precede this task).

---

## Changes to make

### Step 1: New file `channels_hostedreview.go`

**File:** `services/api-gateway/internal/adapter/wscompat/channels_hostedreview.go`

```go
// Channel handlers backing the frontend's hostedReview.* namespace — see
// specs/backend-go/bugs/missing-v1/solutions/SOL-014-hostedreview-channels.md.
package wscompat

import (
	"context"
	"encoding/json"

	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"
)

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
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.CreatePullRequest(attachGitHubIdentity(rpcCtx, id),
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

	// hostedReview.forBranch — uses TASK-072's GetPullRequestForBranch, the
	// same provider-generic single-result RPC github.prForBranch uses
	// (TASK-080), just with an explicit Provider instead of a hardcoded one.
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
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.GetPullRequestForBranch(attachGitHubIdentity(rpcCtx, id),
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
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.CheckHostedReviewEligibility(attachGitHubIdentity(rpcCtx, id),
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
```

### Step 2: Wire into `RegisterRealChannels`

**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`

Find (added by TASK-085):

```go
	registerGitHubChannels(r, scmClient)
	registerGitLabChannels(r, scmClient)
}
```

Replace with:

```go
	registerGitHubChannels(r, scmClient)
	registerGitLabChannels(r, scmClient)
	registerHostedReviewChannels(r, scmClient)
}
```

No `main.go` change needed — `scmClient` is already passed into
`RegisterRealChannels` by TASK-080.

---

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./internal/adapter/wscompat/... && go vet ./internal/adapter/wscompat/...
```

Expected: clean build. Every `hostedReview.*` method in
`specs/frontend/api/rpc-catalog.md` now resolves to a real channel instead
of `notImplementedHandler`.
