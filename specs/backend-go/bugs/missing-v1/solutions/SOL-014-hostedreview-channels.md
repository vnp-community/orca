# SOL-014: Wire `hostedReview.*` to existing PR RPCs + add `CheckHostedReviewEligibility`

**Resolves:** [BUG-014](../BUG-014-hostedreview-channels-not-implemented.md)
**Service:** `scm-integration-service` (one new RPC + one new-ish field) + `api-gateway` (new `wscompat` channels)
**Affected files (proposed):**
- `backend-go/proto/orca/scmintegration/v1/scmintegration.proto`
- `backend-go/services/scm-integration-service/internal/usecase/check_hosted_review_eligibility.go` (new)
- `backend-go/services/scm-integration-service/internal/usecase/ports.go` (extend `ScmProvider`, reuse `CredentialResolver`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_hostedreview.go` (new file)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (wire the new register call into `RegisterRealChannels`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_hostedreview_test.go` (new)
**Status:** 📋 Proposed — not yet implemented

---

## Two of three methods are wiring-only — the design already exists

BUG-014 already found `hostedReview.create` and `hostedReview.forBranch` map
onto RPCs that exist and are provider-agnostic today:
`CreatePullRequest`/`ListPullRequests` (`scmintegration.proto:12-13`), both
parameterized by the shared `ScmProvider` enum (`scmintegration.proto:29-36`,
covering GitHub/GitLab/Bitbucket/Azure DevOps/Gitea uniformly per §3's own
framing: `hostedReview.*` is a thin dispatcher over the same per-provider
capabilities `github.*`/`gitlab.*` expose"). Confirming that reading against
the TDD: §3's RPC list frames `CreatePullRequest`/`ListPullRequests` as the
provider-generic pull-request/merge-request surface, and §6 confirms
`usecase/` code depends only on the `ScmProvider` interface, dispatched by
`provider` field — there is nothing GitHub-specific baked into either RPC's
request shape. `hostedReview.create`/`forBranch` therefore need **no new
backend design**, only the `wscompat` wrapper this document sketches, same
as `github.rateLimit`/`gitlab.rateLimit` in
[SOL-012](./SOL-012-github-channels.md)/[SOL-013](./SOL-013-gitlab-channels.md).

`hostedReview.getCreationEligibility` has no backing RPC anywhere — this
document proposes one new RPC for it, `CheckHostedReviewEligibility`, which
**is already named and scoped in the TDD** (`scm-integration-service.md`
§3: `rpc CheckHostedReviewEligibility(CheckHostedReviewEligibilityRequest)
returns (HostedReviewEligibility);`, alongside `CreateHostedReview`) — this
is a gap-closing task against an already-specified RPC, not a new invention,
matching SOL-001's "design already exists" framing exactly. The TDD's §6
also anticipates its implementation shape directly:
`check_hosted_review_eligibility.go # fans out across configured providers`.

---

## Design — Confirming `CreatePullRequest`/`ListPullRequests` are provider-agnostic

Per the "when done" instruction to confirm this rather than assume it:

```protobuf
// scmintegration.proto:55-64 — CreatePullRequestRequest
message CreatePullRequestRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;   // <- provider is a request FIELD, not baked into the RPC name or a GitHub-specific type
  string repo = 3;
  string title = 4;
  string body = 5;
  string head_branch = 6;
  string base_branch = 7;
  string request_id = 8;
}
```

`repo` is a plain string (not a `GitHubRepoRef`/`owner`+`name` pair), `title`/
`body`/branches are provider-neutral PR/MR concepts, and `provider` is the
shared `ScmProvider` enum — confirmed provider-agnostic, no changes needed to
either message for `hostedReview.create`/`forBranch` to work as-is.

One caveat carried over from BUG-012/BUG-014's own finding: `ListPullRequests`
has no branch filter (`scmintegration.proto:76-84` only takes `repo`).
[SOL-012](./SOL-012-github-channels.md) already proposes the fix as a new,
provider-generic `GetPullRequestForBranch` RPC (not a GitHub-only one — its
request/response messages there carry no GitHub-specific fields) — this
solution's `hostedReview.forBranch` channel calls that same RPC rather than
duplicating it, since "find the PR/MR for this branch" is exactly the same
query shape regardless of which namespace asks. If SOL-012 isn't implemented
first, `hostedReview.forBranch` can fall back to `ListPullRequests` +
client-side filtering by `head_branch`, at the cost of over-fetching; the
RPC addition is the better long-term shape and should not be skipped.

---

## Design — Proto addition: `CheckHostedReviewEligibility`

```protobuf
// CheckHostedReviewEligibility — hostedReview.getCreationEligibility.
// Already named in scm-integration-service.md §3; not yet in
// scmintegration.proto. A pre-flight check, not a mutation: does
// CreatePullRequest have a reasonable chance of succeeding for this
// repo+branch right now. Composes existing signals rather than adding new
// provider-adapter surface:
//   1. GetAuthStatus's connected check (already exists, scmintegration.proto:23)
//   2. A HEAD-branch existence check against the provider (new adapter call,
//      but a read, same shape as ListIssues/ListPullRequests — no new
//      capability class)
//   3. GetPullRequestForBranch (SOL-012) to detect an already-open PR/MR for
//      the branch, surfaced as a reason rather than silently blocking
rpc CheckHostedReviewEligibility(CheckHostedReviewEligibilityRequest) returns (HostedReviewEligibility);

message CheckHostedReviewEligibilityRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string repo = 3;
  string head_branch = 4;
  string base_branch = 5;
}

message HostedReviewEligibility {
  bool eligible = 1;
  // ineligible_reason is set (and eligible=false) for exactly one of these,
  // in priority order — auth comes first since every other check is
  // meaningless without a usable credential:
  //   "NOT_CONNECTED"        - GetAuthStatus.connected == false
  //   "BRANCH_NOT_FOUND"     - head_branch doesn't exist on the provider yet (nothing pushed)
  //   "REVIEW_ALREADY_EXISTS" - GetPullRequestForBranch already found one
  string ineligible_reason = 2;
  // existing_pull_request is set only when ineligible_reason ==
  // "REVIEW_ALREADY_EXISTS" — lets the frontend link straight to it instead
  // of just reporting a blocked state.
  PullRequest existing_pull_request = 3;
}
```

This does **not** introduce a new `ScmProvider`-port method category the way
SOL-012/SOL-013's provider-specific ports did — eligibility is a composition
of 3 existing/already-proposed capabilities (`GetAuthStatus`,
a branch-exists check, `GetPullRequestForBranch`), living entirely in
`usecase/`, per the TDD's own `check_hosted_review_eligibility.go` package
note ("fans out across configured providers"). The one new adapter surface
is the branch-exists check:

```protobuf
// Extends the ScmProvider common port (scm-integration-service.md §4) — a
// plain existence read every provider's REST API supports uniformly
// (GET a ref/branch by name, 200 vs 404), so it belongs on the shared
// interface, unlike SOL-012/SOL-013's provider-specific additions.
```

```go
// internal/usecase/ports.go
type ScmProvider interface {
    // ... existing + SOL-012's additions ...
    BranchExists(ctx context.Context, repo RepoRef, branch string) (bool, error)
}
```

`buf breaking`: additive only.

---

## Design — `usecase/` layer

```go
// internal/usecase/check_hosted_review_eligibility.go
func (uc *ScmUseCase) CheckHostedReviewEligibility(ctx context.Context, req EligibilityInput) (domain.HostedReviewEligibility, error) {
    provider, err := uc.providers.For(req.Provider)
    if err != nil {
        return domain.HostedReviewEligibility{}, err
    }

    // 1. Auth — cheapest check, and every subsequent check is meaningless
    // without it, so fail fast here rather than attempting a branch lookup
    // with no usable credential.
    status, err := uc.authStatus.GetAuthStatus(ctx, req.TenantID, req.Provider)
    if err != nil {
        return domain.HostedReviewEligibility{}, err
    }
    if !status.Connected {
        return domain.HostedReviewEligibility{Eligible: false, IneligibleReason: "NOT_CONNECTED"}, nil
    }

    // 2. Branch existence.
    exists, err := provider.BranchExists(ctx, req.Repo, req.HeadBranch)
    if err != nil {
        return domain.HostedReviewEligibility{}, err
    }
    if !exists {
        return domain.HostedReviewEligibility{Eligible: false, IneligibleReason: "BRANCH_NOT_FOUND"}, nil
    }

    // 3. Existing open PR/MR for this branch — SOL-012's GetPullRequestForBranch.
    existing, found, err := provider.GetPullRequestForBranch(ctx, req.Repo, req.HeadBranch)
    if err != nil {
        return domain.HostedReviewEligibility{}, err
    }
    if found {
        return domain.HostedReviewEligibility{
            Eligible: false, IneligibleReason: "REVIEW_ALREADY_EXISTS", ExistingPullRequest: existing,
        }, nil
    }

    return domain.HostedReviewEligibility{Eligible: true}, nil
}
```

`uc.authStatus` and `uc.providers.For(...)` are the same ports/composition
already backing `GetAuthStatus`/`CreatePullRequest` — no new dependency
injected here beyond `BranchExists`, which every `ScmProvider` adapter
implements once, alongside its existing REST calls.

---

## Design — `wscompat` channel wiring

New file `channels_hostedreview.go`, same `register*Channels` shape as
`channels_github.go`/`channels_gitlab.go`
([SOL-012](./SOL-012-github-channels.md)/[SOL-013](./SOL-013-gitlab-channels.md)).
The one difference from those two: `hostedReview.*` calls must take a
`provider` argument explicitly (the frontend's provider-agnostic namespace
has no fixed provider to hardcode, unlike `github.*`/`gitlab.*`'s channels
which hardcode `SCM_PROVIDER_GITHUB`/`SCM_PROVIDER_GITLAB`).

```go
// backend-go/services/api-gateway/internal/adapter/wscompat/channels_hostedreview.go
package wscompat

import (
    "context"
    "encoding/json"

    scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"
    gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
    "github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
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
        resp, err := client.CreatePullRequest(gatewaygrpc.AttachIdentity(rpcCtx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID}),
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

    // hostedReview.forBranch — prefers SOL-012's GetPullRequestForBranch
    // once available; shown here against ListPullRequests + client-side
    // filter as the fallback shape if SOL-012 lands later.
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
        // Preferred (post-SOL-012): a single-result, branch-scoped RPC.
        resp, err := client.GetPullRequestForBranch(gatewaygrpc.AttachIdentity(rpcCtx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID}),
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
        resp, err := client.CheckHostedReviewEligibility(gatewaygrpc.AttachIdentity(rpcCtx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID}),
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

`channels.go`'s `RegisterRealChannels` gains the third call on the same
`scmClient`, alongside SOL-012/SOL-013's:

```go
func RegisterRealChannels(
    r *Registry,
    // ... existing params ...
    scmClient scmintegrationv1.ScmIntegrationServiceClient,
) {
    // ... existing register*Channels calls ...
    registerGitHubChannels(r, scmClient)
    registerGitLabChannels(r, scmClient)
    registerHostedReviewChannels(r, scmClient)
}
```

---

## Test plan

- `services/scm-integration-service/internal/usecase/check_hosted_review_eligibility_test.go` — fake `ScmProvider` + fake auth-status port; table-driven over the 4 outcomes (`NOT_CONNECTED`, `BRANCH_NOT_FOUND`, `REVIEW_ALREADY_EXISTS`, eligible), asserting priority order (auth checked before branch, branch before existing-PR).
- `services/scm-integration-service/internal/adapter/external/*/branch_exists_test.go` — one per provider adapter (or at minimum GitHub+GitLab), httptest fixture for the 200-vs-404 branch-ref lookup.
- `services/api-gateway/internal/adapter/wscompat/channels_hostedreview_test.go` — 3 tests (one per channel), fake `ScmIntegrationServiceClient`, mirroring `channels_github_test.go`'s shape from SOL-012. Include a case asserting `hostedReview.forBranch` returns `nil` (not an error) when `found == false`.
- Contract test: `hostedReview.create` channel and `POST /v1/scm/pull-requests` (`scm_routes.go`) both resolve to `CreatePullRequest` with identical response shape — same regression-guard pattern SOL-001/SOL-012 use elsewhere.
- `buf breaking` in CI — `CheckHostedReviewEligibility` and `BranchExists`'s request/response additions are additive only.

## References

- `specs/backend-go/bugs/missing-v1/BUG-014-hostedreview-channels-not-implemented.md` — the 3-method gap this solution closes, including the "2 of 3 already have plausible RPCs" finding
- [`SOL-012-github-channels.md`](./SOL-012-github-channels.md) — `GetPullRequestForBranch` RPC this solution's `hostedReview.forBranch` reuses; shared `wscompat` wiring pattern
- [`SOL-013-gitlab-channels.md`](./SOL-013-gitlab-channels.md) — sibling solution, same `register*Channels`/`RegisterRealChannels` wiring shape
- `specs/backend-go/tdd/services/scm-integration-service.md` §3,4,6 — `CheckHostedReviewEligibility`/`CreateHostedReview` already named in the target RPC list; `check_hosted_review_eligibility.go`'s "fans out across configured providers" package note; `ScmProvider` common port
- `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md` — usecase/port layering, adapter-package boundaries (why `parseWSProvider` isn't imported from `httpgateway`)
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md` — gRPC conventions, `buf breaking`
- `backend-go/proto/orca/scmintegration/v1/scmintegration.proto:7-29,55-84` — `CreatePullRequest`/`ListPullRequests`/`ScmProvider` confirmed provider-agnostic
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` — `register*Channels` pattern to mirror
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` — `ChannelHandler`, `decodeArg`, `Identity`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/scm_routes.go` — existing REST proxy, `handleCreatePullRequest`/`handleListPullRequests`, `parseSCMProvider`
- `specs/frontend/api/rpc-catalog.md` — `hostedReview.*` namespace description ("Provider-agnostic hosted PR/MR read+create")
