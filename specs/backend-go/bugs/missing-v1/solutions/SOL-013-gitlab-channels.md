# SOL-013: Add `ScmIntegrationService` RPCs + `gitlab.*` wscompat channels

**Resolves:** [BUG-013](../BUG-013-gitlab-channels-not-implemented.md)
**Service:** `scm-integration-service` (new RPCs) + `api-gateway` (new `wscompat` channels)
**Affected files (proposed):**
- `backend-go/proto/orca/scmintegration/v1/scmintegration.proto`
- `backend-go/services/scm-integration-service/internal/usecase/*.go` (new use cases)
- `backend-go/services/scm-integration-service/internal/adapter/external/gitlab/*.go` (GitLab REST client methods)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_gitlab.go` (new file)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (wire the new register call into `RegisterRealChannels`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_gitlab_test.go` (new)
**Status:** 📋 Proposed — not yet implemented

---

## Same pattern as SOL-012, GitLab-provider mirror

This is the GitLab-provider instance of the shape [SOL-012](./SOL-012-github-channels.md)
establishes: `scm-integration-service`'s OAuth flow
(`StartOAuthFlow`/`CompleteOAuthFlow`/`RevokeAuth`/`GetAuthStatus`,
`scmintegration.proto:23-26`) is already provider-generic and already covers
`SCM_PROVIDER_GITLAB` (`scmintegration.proto:32`) — BUG-013 confirms no new
auth design is needed here, only new domain-operation RPCs plus wscompat
wiring, exactly as SOL-012 found for GitHub. Where GitLab's REST API shape
differs materially from GitHub's, it's called out below; where it doesn't,
this document stays terse rather than re-deriving SOL-012's reasoning.

Unlike GitHub, GitLab's Projects-v2-equivalent gap doesn't exist here —
`gitlab.*`'s 4 methods are smaller in scope and entirely REST-backed (GitLab
merge requests and discussion threads have a full REST API; no GraphQL-only
sub-namespace like GitHub Projects v2). This solution is a single proto
addition sized to match.

---

## Design — Proto additions

```protobuf
// ListMergeRequests — gitlab.listMRs. GitLab REST:
// GET /projects/:id/merge_requests. Kept as a SEPARATE RPC from
// ListPullRequests (not a reused/aliased call) because GitLab's MR shape
// carries fields PullRequest (scmintegration.proto:66-70) doesn't have —
// source/target branch, discussion count, approval state — the same gap
// BUG-013 identifies. Introducing a GitLab-specific MergeRequest message
// now avoids two rounds of proto churn: widening PullRequest to fit GitLab
// would either bloat it with GitHub-irrelevant fields or require a second
// "is this field set" convention on top of proto3's existing one.
rpc ListMergeRequests(ListMergeRequestsRequest) returns (ListMergeRequestsResponse);

// ResolveMergeRequestDiscussion — gitlab.resolveMRDiscussion. GitLab REST:
// PUT /projects/:id/merge_requests/:iid/discussions/:discussion_id
// ?resolved=true. GitLab's "discussion" (a threaded comment group on a
// diff line or the MR itself) has no GitHub equivalent modeled in this
// proto yet — GitHub's PR review-comment threads are a materially different
// shape (flat, no explicit resolve state via this API), so this stays a
// GitLab-only RPC rather than a forced provider-generic abstraction.
rpc ResolveMergeRequestDiscussion(ResolveMergeRequestDiscussionRequest) returns (MergeRequestDiscussion);

// GetWorkItemDetails — gitlab.workItemDetails. GitLab REST:
// GET /projects/:id/merge_requests/:iid (for MR-shaped work items) or
// GET /projects/:id/issues/:iid (for issue-shaped ones) — the adapter picks
// based on the item_type field below since GitLab addresses issues and MRs
// by separate iid sequences, not a shared "work item" ID space.
rpc GetWorkItemDetails(GetWorkItemDetailsRequest) returns (WorkItemDetailsGitLab);

message MergeRequest {
  string id = 1;
  string url = 2;
  string state = 3;              // "opened" | "closed" | "merged" | "locked"
  int32 iid = 4;                 // project-scoped internal ID, what the UI/URLs use
  string title = 5;
  string source_branch = 6;
  string target_branch = 7;
  bool draft = 8;
  int32 discussion_count = 9;
  int32 unresolved_discussion_count = 10;
  string merge_status = 11;      // GitLab's own can_be_merged / mergeable / etc.
}

message ListMergeRequestsRequest {
  string tenant_id = 1;
  string repo = 2;               // GitLab project path ("group/project"), no provider field needed — GitLab-only RPC
  string state = 3;              // "opened" | "closed" | "merged" | "all"; empty = "opened"
  string source_branch = 4;      // optional filter
}
message ListMergeRequestsResponse {
  repeated MergeRequest merge_requests = 1;
}

message MergeRequestDiscussion {
  string id = 1;
  bool resolved = 2;
  string resolved_by = 3;
}
message ResolveMergeRequestDiscussionRequest {
  string tenant_id = 1;
  string repo = 2;
  int32 merge_request_iid = 3;
  string discussion_id = 4;
  bool resolved = 5;
}

message GetWorkItemDetailsRequest {
  string tenant_id = 1;
  string repo = 2;
  int32 iid = 3;
  string item_type = 4;          // "merge_request" | "issue"
}
message WorkItemDetailsGitLab {
  string id = 1;
  int32 iid = 2;
  string item_type = 3;
  string title = 4;
  string body = 5;
  string state = 6;
  string url = 7;
  repeated string labels = 8;
}
```

These 3 RPCs are deliberately **not** parameterized by the shared
`ScmProvider` enum the way `ListIssues`/`GetRateLimitStatus` are — they model
GitLab-specific concepts (`iid`, discussions, merge-request-vs-issue
addressing) that don't generalize across providers, matching §4's own
`ErrCapabilityUnsupported` acknowledgment that not every operation is
uniform across the 5 providers. This mirrors SOL-012's GitHub Projects v2
RPCs being GitHub-only rather than forced onto the generic surface — same
reasoning, different provider. `gitlab.rateLimit` stays on the existing
provider-generic `GetRateLimitStatus`, unchanged.

`buf breaking`: all additive, no existing field/message changes.

---

## Design — `usecase/` layer

```go
// internal/usecase/ports.go — a GitLab-only port, same reasoning as
// SOL-012's GitHubProjectsProvider: these 3 operations don't belong on the
// common ScmProvider interface since no other provider implements them.
type GitLabMergeRequestProvider interface {
    ListMergeRequests(ctx context.Context, repo RepoRef, filter MRFilter) ([]domain.MergeRequest, error)
    ResolveDiscussion(ctx context.Context, repo RepoRef, mrIID int32, discussionID string, resolved bool) (domain.MergeRequestDiscussion, error)
    GetWorkItemDetails(ctx context.Context, repo RepoRef, iid int32, itemType string) (domain.WorkItemDetails, error)
}
```

```go
// internal/usecase/list_merge_requests.go
func (uc *ScmUseCase) ListMergeRequests(ctx context.Context, req ListMRInput) ([]domain.MergeRequest, error) {
    // No provider-keyed lookup (uc.providers.For(...)) needed — this RPC is
    // GitLab-only by construction, so the usecase talks to the injected
    // GitLab adapter directly, same shape as SOL-012's
    // UpdateProjectItemField talking to uc.githubProjects directly.
    return uc.gitlabMRs.ListMergeRequests(ctx, req.Repo, domain.MRFilter{
        State:        req.State,
        SourceBranch: req.SourceBranch,
    })
}
```

`adapter/external/gitlab/` reuses the existing GitLab REST HTTP client and
`RateLimit-*` header parsing already built for `GetRateLimitStatus` (§8) —
GitLab exposes one rate-limit bucket per token (unlike GitHub's REST/
GraphQL/search split), so no bucket-key change is needed here, unlike
SOL-012's GitHub GraphQL addition.

---

## Design — `wscompat` channel wiring

New file `channels_gitlab.go`, same `register*Channels` pattern as
`channels_github.go` (SOL-012) and the existing `channels.go` handlers:

```go
// backend-go/services/api-gateway/internal/adapter/wscompat/channels_gitlab.go
package wscompat

import (
    "context"
    "encoding/json"

    scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"
    gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
    "github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

func registerGitLabChannels(r *Registry, client scmintegrationv1.ScmIntegrationServiceClient) {
    // gitlab.rateLimit — real backing RPC already exists (BUG-013's
    // finding), provider-generic, same as github.rateLimit in SOL-012.
    r.Register("gitlab.rateLimit", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.GetRateLimitStatus(gatewaygrpc.AttachIdentity(rpcCtx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID}),
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
        resp, err := client.ListMergeRequests(gatewaygrpc.AttachIdentity(rpcCtx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID}),
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
            Repo             string `json:"repo"`
            MergeRequestIID  int32  `json:"mergeRequestIid"`
            DiscussionID     string `json:"discussionId"`
            Resolved         bool   `json:"resolved"`
        }
        in, err := decodeArg[resolveArgs](args, 0)
        if err != nil {
            return nil, err
        }
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.ResolveMergeRequestDiscussion(gatewaygrpc.AttachIdentity(rpcCtx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID}),
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
        resp, err := client.GetWorkItemDetails(gatewaygrpc.AttachIdentity(rpcCtx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID}),
            &scmintegrationv1.GetWorkItemDetailsRequest{
                TenantId: id.TenantID, Repo: in.Repo, Iid: in.IID, ItemType: in.ItemType,
            })
        if err != nil {
            return nil, err
        }
        return resp, nil
    })

    // gitlab.startAuthLogin / gitlab.revokeAuth — same thin OAuth wrapper as
    // SOL-012's github.startAuthLogin/revokeAuth, SCM_PROVIDER_GITLAB
    // instead of SCM_PROVIDER_GITHUB. Not in BUG-013's tracked method list
    // (noted there for completeness) but included here since it's zero
    // additional design cost once SOL-012's wrapper exists — copy the same
    // 2 handlers with the provider constant swapped.
}
```

`channels.go`'s `RegisterRealChannels` gains one more call, alongside
SOL-012's `registerGitHubChannels` — both share the same `scmClient`
parameter, no second gRPC dial:

```go
func RegisterRealChannels(
    r *Registry,
    // ... existing params ...
    scmClient scmintegrationv1.ScmIntegrationServiceClient,
) {
    // ... existing register*Channels calls ...
    registerGitHubChannels(r, scmClient)
    registerGitLabChannels(r, scmClient)
}
```

---

## Test plan

- `services/scm-integration-service/internal/usecase/list_merge_requests_test.go` — fake `GitLabMergeRequestProvider`, assert filter mapping (`state`/`source_branch`).
- `services/scm-integration-service/internal/adapter/external/gitlab/discussions_test.go` — httptest fixture for `PUT .../discussions/:id?resolved=true`, assert request shape and response mapping.
- `services/api-gateway/internal/adapter/wscompat/channels_gitlab_test.go` — one test per channel (4), mirroring `channels_github_test.go`'s shape from SOL-012: fake `ScmIntegrationServiceClient`, assert arg decode + response passthrough.
- Contract test: `gitlab.rateLimit` channel and `GET /v1/scm/rate-limit?provider=gitlab` both resolve to `GetRateLimitStatus` — same regression-guard pattern as SOL-012's GitHub contract test.
- `buf breaking` in CI — all additions here are additive.

## References

- `specs/backend-go/bugs/missing-v1/BUG-013-gitlab-channels-not-implemented.md` — the 4-method gap this solution closes
- [`SOL-012-github-channels.md`](./SOL-012-github-channels.md) — the shared pattern this solution mirrors (provider-generic OAuth reuse, `wscompat` wiring shape, provider-specific-port reasoning)
- `specs/backend-go/tdd/services/scm-integration-service.md` §3,4,6,8,9 — RPC surface, `ScmProvider` port, per-provider rate-limit bucket note, per-tenant credential guarantee
- `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md` — usecase/port layering
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md` — gRPC conventions, `buf breaking`
- `backend-go/proto/orca/scmintegration/v1/scmintegration.proto` — current 8-RPC surface this solution extends, `SCM_PROVIDER_GITLAB`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` — `register*Channels` pattern to mirror
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` — `ChannelHandler`, `decodeArg`, `Identity`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/scm_routes.go` — existing REST proxy pattern, `parseSCMProvider`
- `specs/frontend/api/rpc-catalog.md` — authoritative `gitlab.*` method catalog
