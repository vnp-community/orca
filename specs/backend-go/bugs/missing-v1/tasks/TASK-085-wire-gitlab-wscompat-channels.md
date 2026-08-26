# TASK-085: Wire `gitlab.*` wscompat channels

**From Solution:** SOL-013 (Design — `wscompat` channel wiring)
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels_gitlab.go` (new), `channels.go`
**Depends on:** TASK-080, TASK-084
**Status:** `[x]` DONE — same `channels_scm.go`/`registerSCMChannels` deviation as TASK-080 (orchestrator override; `channels.go` not touched, no new `RegisterRealChannels` param needed since `registerSCMChannels` is one function covering github+gitlab+hostedReview). Verified: `go build`/`go vet`/`go test ./internal/adapter/wscompat/...` clean.

---

## Context

New file, mirroring `channels_github.go`'s (TASK-080) `register*Channels`
pattern exactly. Reuses the same `scmClient` `RegisterRealChannels` already
takes after TASK-080 — no new gRPC dial, no new `RegisterRealChannels`
parameter.

---

## Changes to make

### Step 1: New file `channels_gitlab.go`

**File:** `services/api-gateway/internal/adapter/wscompat/channels_gitlab.go`

```go
// Channel handlers backing the frontend's gitlab.* namespace — see
// specs/backend-go/bugs/missing-v1/solutions/SOL-013-gitlab-channels.md.
package wscompat

import (
	"context"
	"encoding/json"

	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"
)

func registerGitLabChannels(r *Registry, client scmintegrationv1.ScmIntegrationServiceClient) {
	// gitlab.rateLimit — real backing RPC already exists (BUG-013's
	// finding), provider-generic, same as github.rateLimit (TASK-080).
	r.Register("gitlab.rateLimit", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.GetRateLimitStatus(attachGitHubIdentity(rpcCtx, id),
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
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListMergeRequests(attachGitHubIdentity(rpcCtx, id),
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
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ResolveMergeRequestDiscussion(attachGitHubIdentity(rpcCtx, id),
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
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.GetWorkItemDetails(attachGitHubIdentity(rpcCtx, id),
			&scmintegrationv1.GetWorkItemDetailsRequest{
				TenantId: id.TenantID, Repo: in.Repo, Iid: in.IID, ItemType: in.ItemType,
			})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	// gitlab.startAuthLogin / gitlab.revokeAuth — same thin OAuth wrapper as
	// github.startAuthLogin/revokeAuth (TASK-080), SCM_PROVIDER_GITLAB
	// instead of SCM_PROVIDER_GITHUB.
	r.Register("gitlab.startAuthLogin", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
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
				Provider: scmintegrationv1.ScmProvider_SCM_PROVIDER_GITLAB, RedirectUri: in.RedirectURI,
			})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("gitlab.revokeAuth", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.RevokeAuth(attachGitHubIdentity(rpcCtx, id),
			&scmintegrationv1.RevokeAuthRequest{TenantId: id.TenantID, Provider: scmintegrationv1.ScmProvider_SCM_PROVIDER_GITLAB})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
}
```

`attachGitHubIdentity` (TASK-080) is a plain `AttachIdentity` wrapper with
no GitHub-specific behavior — reused verbatim here despite the name; it is
not GitHub-scoped in what it does, only in where it was first added. (If
this naming bothers a reviewer, rename it to `attachSCMIdentity` in
`channels_github.go` as part of this task and update both call sites — not
required for correctness.)

### Step 2: Wire into `RegisterRealChannels`

**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`

Find (added by TASK-080):

```go
	registerGitHubChannels(r, scmClient)
}
```

Replace with:

```go
	registerGitHubChannels(r, scmClient)
	registerGitLabChannels(r, scmClient)
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

Expected: clean build. Every `gitlab.*` method in
`specs/frontend/api/rpc-catalog.md` now resolves to a real channel instead
of `notImplementedHandler`.
