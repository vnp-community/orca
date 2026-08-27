# SOL-012: Add `ScmIntegrationService` RPCs + `github.*` wscompat channels

**Resolves:** [BUG-012](../BUG-012-github-channels-not-implemented.md)
**Service:** `scm-integration-service` (new RPCs) + `api-gateway` (new `wscompat` channels)
**Affected files (proposed):**
- `backend-go/proto/orca/scmintegration/v1/scmintegration.proto`
- `backend-go/services/scm-integration-service/internal/usecase/*.go` (new use cases)
- `backend-go/services/scm-integration-service/internal/adapter/external/github/*.go` (GitHub REST + GraphQL client methods)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_github.go` (new file)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (wire the new register call into `RegisterRealChannels`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_github_test.go` (new)
**Status:** ✅ Implemented — all 12 task(s) (TASK-071–082) DONE; see each task file's own Status/Verify section for evidence.

---

## The auth foundation already exists — this is an RPC-surface + wiring gap, not an auth redesign

BUG-012 already confirmed `scm-integration-service`'s OAuth flow is real and
provider-agnostic: `StartOAuthFlow`/`CompleteOAuthFlow`/`RevokeAuth`/
`GetAuthStatus` (`scmintegration.proto:23-26`) implement the TDD's §9.1 web
flow, and the doc comment at `scmintegration.proto:7-8` is explicit that this
service is "a direct per-tenant OAuth API client — NOT a CLI shell-out." Every
RPC this solution adds resolves its GitHub token the same way
`CreatePullRequest`/`ListIssues` already do — through
`credentialbroker`'s port (`scm-integration-service.md` §6/§9) — never a new
credential mechanism. `github.startAuthLogin`/`github.revokeAuth` need only a
thin wscompat wrapper over the existing 4 RPCs (see the last section below),
same as `rateLimit`.

The 23 remaining methods split into three shapes. Each gets one representative
Go sketch; the rest follow the same shape via a signature table.

1. **Simple PR/issue mutations** (5 methods) — one new RPC each on
   `ScmIntegrationService`, GitHub REST-backed.
2. **Repo/branch resolution** (2 methods) — read-only lookups, GitHub REST.
3. **GitHub Projects v2** (17 methods, all under `github.project.*`) —
   GitHub's Projects v2 has **no REST API**; it is GraphQL-only. This is the
   one namespace where `scmintegration.proto`'s current one-message-per-thing
   pattern (`Issue`, `PullRequest`) doesn't fit well — Projects v2 items,
   fields, and views are GraphQL nodes with dynamic (per-project-configured)
   field schemas, not a fixed proto message shape. Proposed handling below.

---

## Design — Proto additions, shape 1: PR/issue mutations

Additive RPCs on `ScmIntegrationService`, following the existing
`CreatePullRequestRequest`/`Response` naming convention exactly
(`tenant_id`, `provider`, `repo` as the first three fields on every request,
per `scmintegration.proto:55-64`):

```protobuf
// MergePullRequest — github.mergePR. GitHub REST: PUT /repos/{owner}/{repo}/pulls/{number}/merge.
rpc MergePullRequest(MergePullRequestRequest) returns (MergePullRequestResponse);

// RequestPRReviewers / RemovePRReviewers — github.requestPRReviewers /
// github.removePRReviewers. GitHub REST: POST/DELETE
// /repos/{owner}/{repo}/pulls/{number}/requested_reviewers.
rpc RequestPullRequestReviewers(RequestPullRequestReviewersRequest) returns (PullRequest);
rpc RemovePullRequestReviewers(RemovePullRequestReviewersRequest) returns (PullRequest);

// SetPullRequestAutoMerge — github.setPRAutoMerge. GitHub GraphQL only
// (enablePullRequestAutoMerge mutation) — REST has no auto-merge endpoint,
// same "GraphQL-only" situation as Projects v2 below, but a single mutation
// rather than a whole sub-namespace, so it stays a plain RPC here rather than
// following the Projects v2 pattern.
rpc SetPullRequestAutoMerge(SetPullRequestAutoMergeRequest) returns (PullRequest);

// UpdateIssue — github.updateIssue AND github.project.updateIssueBySlug's
// non-Projects half (see Projects v2 section — updateIssueBySlug's slug
// resolves to this same RPC plus a field-write, not a separate mutation).
rpc UpdateIssue(UpdateIssueRequest) returns (Issue);

message MergePullRequestRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string repo = 3;
  int32 number = 4;
  string merge_method = 5; // "merge" | "squash" | "rebase"
  string commit_title = 6;
  string commit_message = 7;
}
message MergePullRequestResponse {
  PullRequest pull_request = 1;
  bool merged = 2;
  string sha = 3;
}

message RequestPullRequestReviewersRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string repo = 3;
  int32 number = 4;
  repeated string reviewer_logins = 5;
  repeated string team_slugs = 6;
}
message RemovePullRequestReviewersRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string repo = 3;
  int32 number = 4;
  repeated string reviewer_logins = 5;
}

message SetPullRequestAutoMergeRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string repo = 3;
  int32 number = 4;
  bool enabled = 5;
  string merge_method = 6;
}

message UpdateIssueRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string repo = 3;
  int32 number = 4;
  optional string title = 5;
  optional string body = 6;
  optional string state = 7;
  repeated string add_labels = 8;
  repeated string remove_labels = 9;
  repeated string assignees = 10;
}
```

`Issue`/`PullRequest` (`scmintegration.proto:38-43,66-70`) stay as-is —
`number` isn't currently a field on either message (only `id`/`url`/`state`),
which is a pre-existing gap: GitHub's REST API addresses everything by
`number` (repo-scoped, e.g. `#42`), not the opaque `id` these messages
currently carry. Propose adding `int32 number = 5;` to both `Issue` and
`PullRequest` as part of this change — every RPC above needs it to address
the target, and `ListIssues`/`ListPullRequests`/`CreatePullRequest`'s existing
callers get a free additive field, no breaking change.

## Design — Proto additions, shape 2: repo/branch resolution

```protobuf
// PullRequestForBranch — github.prForBranch / hostedReview.forBranch's
// branch-filtered case (see SOL-014). ListPullRequests has no branch filter
// today (BUG-012's own finding) — add one as a new RPC rather than an
// optional field on ListPullRequestsRequest, since "find the PR for this
// branch" is a distinct, single-result query shape from "list PRs", not a
// filtered list (frontend calls it to get one specific PR object, not a
// filtered page).
rpc GetPullRequestForBranch(GetPullRequestForBranchRequest) returns (GetPullRequestForBranchResponse);

// ResolveRepoSlug — github.repoSlug. Resolves a repo identifier (e.g. a
// local git remote URL or a partial name) to the canonical "owner/name"
// slug GitHub's API expects everywhere else. Read-only, GitHub REST GET
// /repos/{owner}/{repo} to validate + canonicalize.
rpc ResolveRepoSlug(ResolveRepoSlugRequest) returns (ResolveRepoSlugResponse);

message GetPullRequestForBranchRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string repo = 3;
  string head_branch = 4;
}
message GetPullRequestForBranchResponse {
  PullRequest pull_request = 1; // unset (zero-value) if no open PR for the branch
  bool found = 2;
}

message ResolveRepoSlugRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string candidate = 3; // remote URL, "owner/name", or bare name
}
message ResolveRepoSlugResponse {
  string owner = 1;
  string name = 2;
  string slug = 3; // "owner/name", canonical
}
```

## Design — Proto additions, shape 3: `github.project.*` (GitHub Projects v2, 17 methods)

**Why this doesn't fit the existing message-per-entity pattern**: GitHub
Projects v2 (the only Projects API GitHub still supports — Projects
"classic" is deprecated) is **GraphQL-only**; there is no REST equivalent for
any of these 17 methods. Its items carry a per-project-configurable set of
custom fields (single-select, iteration, number, text — user-defined per
project board), so a fixed proto message per field type would need to
anticipate every field kind GitHub might add. Rather than inventing a
generic "GraphQL node" abstraction inside the proto (over-engineering for a
GitHub-specific concept every other provider lacks — Projects v2 has no
GitLab/Bitbucket/Azure DevOps/Gitea equivalent, confirmed by
`scm-integration-service.md`'s `ErrCapabilityUnsupported` pattern for exactly
this situation), propose a **dedicated `github.project.*` proto sub-surface**
scoped to GitHub only — not routed through the provider-generic
`ScmProvider` enum, since it structurally cannot generalize to other
providers.

```protobuf
// GitHub Projects v2 (GraphQL-only, GitHub-specific — no other provider has
// an equivalent, see scm-integration-service.md §4's ErrCapabilityUnsupported
// pattern). tenant_id/user_id resolve the same per-tenant GitHub OAuth token
// as every other RPC on this service; there is no separate Projects-v2 auth.
rpc ListAccessibleProjects(ListAccessibleProjectsRequest) returns (ListAccessibleProjectsResponse);
rpc ResolveProjectRef(ResolveProjectRefRequest) returns (ResolveProjectRefResponse);
rpc ListProjectViews(ListProjectViewsRequest) returns (ListProjectViewsResponse);
rpc ViewProjectTable(ViewProjectTableRequest) returns (ViewProjectTableResponse);
rpc UpdateProjectItemField(UpdateProjectItemFieldRequest) returns (ProjectItem);
rpc ClearProjectItemField(ClearProjectItemFieldRequest) returns (ProjectItem);
rpc GetWorkItemDetailsBySlug(GetWorkItemDetailsBySlugRequest) returns (WorkItemDetails);
rpc UpdateIssueBySlug(UpdateIssueBySlugRequest) returns (WorkItemDetails);
rpc UpdatePullRequestBySlug(UpdatePullRequestBySlugRequest) returns (WorkItemDetails);
rpc UpdateIssueTypeBySlug(UpdateIssueTypeBySlugRequest) returns (WorkItemDetails);
rpc ListIssueTypesBySlug(ListIssueTypesBySlugRequest) returns (ListIssueTypesBySlugResponse);
rpc ListAssignableUsersBySlug(ListAssignableUsersBySlugRequest) returns (ListAssignableUsersBySlugResponse);
rpc ListLabelsBySlug(ListLabelsBySlugRequest) returns (ListLabelsBySlugResponse);
rpc AddIssueCommentBySlug(AddIssueCommentBySlugRequest) returns (ProjectComment);
rpc UpdateIssueCommentBySlug(UpdateIssueCommentBySlugRequest) returns (ProjectComment);
rpc DeleteIssueCommentBySlug(DeleteIssueCommentBySlugRequest) returns (google.protobuf.Empty);

// A generic key/value field write, since Projects v2 fields are
// per-project-defined (text/number/date/single-select/iteration) — the
// GraphQL mutation (updateProjectV2ItemFieldValue) itself takes a typed
// union, so the adapter (not the proto) is responsible for picking the
// right GraphQL input shape from this string+kind pair.
message ProjectFieldValue {
  string field_id = 1;
  string kind = 2;  // "text" | "number" | "date" | "single_select" | "iteration"
  string value = 3; // string-encoded; adapter parses per kind
}

message ProjectItem {
  string id = 1;
  string title = 2;
  string content_type = 3; // "issue" | "pull_request" | "draft_issue"
  string content_url = 4;
  repeated ProjectFieldValue fields = 5;
}

// "BySlug" methods all take a project_slug (owner/project_number, resolved
// via ResolveProjectRef) + an item/issue/PR identifier — mirrors the
// frontend's own by-slug addressing scheme (BUG-012's finding: "no by-slug
// addressing scheme" exists server-side yet; this is where it's added).
message WorkItemDetails {
  string slug = 1;
  string title = 2;
  string body = 3;
  string state = 4;
  string url = 5;
  repeated ProjectFieldValue fields = 6;
}

// (ListAccessibleProjectsRequest/Response, ResolveProjectRefRequest/Response,
// ListProjectViewsRequest/Response, ViewProjectTableRequest/Response,
// ListIssueTypesBySlugRequest/Response, ListAssignableUsersBySlugRequest/
// Response, ListLabelsBySlugRequest/Response, ProjectComment, and each
// "BySlug" request message follow the same tenant_id + project_slug (+
// item_slug where applicable) shape as WorkItemDetails above — omitted here
// for brevity, sketched individually at implementation time.)
```

### Signature table — the 17 `github.project.*` methods

| Frontend method | Proposed RPC | GitHub GraphQL op |
|---|---|---|
| `listAccessible` | `ListAccessibleProjects` | `viewer.projectsV2` / `organization.projectsV2` |
| `resolveRef` | `ResolveProjectRef` | `organization.projectV2(number:)` |
| `listViews` | `ListProjectViews` | `projectV2.views` |
| `viewTable` | `ViewProjectTable` | `projectV2.items` + field values |
| `updateItemField` | `UpdateProjectItemField` | `updateProjectV2ItemFieldValue` mutation |
| `clearItemField` | `ClearProjectItemField` | `clearProjectV2ItemFieldValue` mutation |
| `workItemDetailsBySlug` | `GetWorkItemDetailsBySlug` | `node(id:)` on issue/PR |
| `updateIssueBySlug` | `UpdateIssueBySlug` | REST `PATCH /repos/{o}/{r}/issues/{n}` (issue mutation itself is REST; only the Projects field-value write, if any, is GraphQL) |
| `updatePullRequestBySlug` | `UpdatePullRequestBySlug` | REST `PATCH /repos/{o}/{r}/pulls/{n}` |
| `updateIssueTypeBySlug` | `UpdateIssueTypeBySlug` | REST `PATCH /repos/{o}/{r}/issues/{n}` (`type` field) |
| `listIssueTypesBySlug` | `ListIssueTypesBySlug` | REST `GET /orgs/{org}/issue-types` |
| `listAssignableUsersBySlug` | `ListAssignableUsersBySlug` | REST `GET /repos/{o}/{r}/assignees` |
| `listLabelsBySlug` | `ListLabelsBySlug` | REST `GET /repos/{o}/{r}/labels` |
| `addIssueCommentBySlug` | `AddIssueCommentBySlug` | REST `POST /repos/{o}/{r}/issues/{n}/comments` |
| `updateIssueCommentBySlug` | `UpdateIssueCommentBySlug` | REST `PATCH .../comments/{id}` |
| `deleteIssueCommentBySlug` | `DeleteIssueCommentBySlug` | REST `DELETE .../comments/{id}` |
| *(item read for `resolveRef`/`viewTable`)* | *(covered above)* | `projectV2.items(first:)` paginated |

Note several "BySlug" methods (comments, issue-type, assignable users,
labels) are actually plain **REST** calls scoped by the slug's repo — only
the true Projects-board operations (`listAccessible`, `resolveRef`,
`listViews`, `viewTable`, `updateItemField`, `clearItemField`) require
GraphQL. Grouping all 17 under one proto sub-surface still makes sense
because they share the by-slug addressing convention and the frontend's
`github.project.*` namespace boundary — but the GitHub adapter internally
dispatches each to REST or GraphQL per the table above, not uniformly.

`buf breaking` stays clean: every addition here is a new RPC/message, no
existing field changes except the additive `number` field on `Issue`/
`PullRequest` noted above.

---

## Design — `usecase/` layer

Per `03-clean-architecture-guidelines.md`, usecases depend on the
`ScmProvider` port (`scm-integration-service.md` §4), never a concrete GitHub
client. One representative sketch for shape 1 (`MergePullRequest`) and one
for shape 3 (`UpdateProjectItemField`) — the rest of shapes 1/2 are
structurally identical to `MergePullRequest`; the rest of shape 3 to
`UpdateProjectItemField`.

```go
// internal/usecase/ports.go — extends the existing ScmProvider interface
// (scm-integration-service.md §4) with the new operations. Providers that
// can't support an operation (GitLab/Bitbucket/Azure DevOps/Gitea for every
// github.project.* method) return domain.ErrCapabilityUnsupported — checked
// explicitly here, never a panic or a silent no-op.
type ScmProvider interface {
    // ... existing methods ...
    MergePullRequest(ctx context.Context, repo RepoRef, number int32, input MergeInput) (domain.PullRequest, error)
    RequestPullRequestReviewers(ctx context.Context, repo RepoRef, number int32, reviewers, teams []string) (domain.PullRequest, error)
    UpdateIssue(ctx context.Context, repo RepoRef, number int32, patch IssuePatch) (domain.Issue, error)
    GetPullRequestForBranch(ctx context.Context, repo RepoRef, headBranch string) (domain.PullRequest, bool, error)
    ResolveRepoSlug(ctx context.Context, candidate string) (owner, name string, err error)
}

// GitHubProjectsProvider is a SEPARATE, narrower port — only the GitHub
// adapter implements it (scm-integration-service.md §4's "each provider
// implements a common port" principle extends here: Projects v2 isn't part
// of the common ScmProvider surface at all, since no other provider has it,
// so it doesn't belong on the interface every provider must satisfy).
type GitHubProjectsProvider interface {
    ListAccessibleProjects(ctx context.Context, tenantID string) ([]domain.Project, error)
    ViewProjectTable(ctx context.Context, projectSlug string) (domain.ProjectTable, error)
    UpdateItemField(ctx context.Context, projectSlug, itemID string, field domain.ProjectFieldValue) (domain.ProjectItem, error)
    // ... remaining 14 methods, same shape ...
}
```

```go
// internal/usecase/merge_pull_request.go
func (uc *ScmUseCase) MergePullRequest(ctx context.Context, req MergePRInput) (domain.PullRequest, error) {
    provider, err := uc.providers.For(req.Provider) // keyed lookup, main.go's composition root, §6
    if err != nil {
        return domain.PullRequest{}, err
    }
    // credential resolution happens INSIDE provider.MergePullRequest via the
    // credentialbroker adapter injected at construction — usecase code never
    // touches a token directly, per §9's structural-guarantee requirement.
    return provider.MergePullRequest(ctx, req.Repo, req.Number, MergeInput{
        Method:  req.MergeMethod,
        Title:   req.CommitTitle,
        Message: req.CommitMessage,
    })
}

// internal/usecase/update_project_item_field.go — GitHub-only usecase; the
// gRPC adapter routes UpdateProjectItemField calls here directly rather than
// through the provider-keyed uc.providers.For(...) lookup shape.MergePullRequest
// uses, since this RPC has no cross-provider fan-out to do.
func (uc *ScmUseCase) UpdateProjectItemField(ctx context.Context, req UpdateFieldInput) (domain.ProjectItem, error) {
    if req.Provider != domain.ProviderGitHub {
        return domain.ProjectItem{}, domain.ErrCapabilityUnsupported
    }
    return uc.githubProjects.UpdateItemField(ctx, req.ProjectSlug, req.ItemID, domain.ProjectFieldValue{
        FieldID: req.FieldID, Kind: req.Kind, Value: req.Value,
    })
}
```

`adapter/external/github/` implementation notes (per §6/§8 of the TDD): the
REST-backed methods reuse the existing GitHub REST HTTP client and
rate-limit-header parsing already built for `ListIssues`/`CreatePullRequest`;
the GraphQL-backed Projects v2 methods need a second client (GitHub's single
GraphQL endpoint, `POST /graphql`) inside the same `github` adapter package —
still one `ScmProvider`(+`GitHubProjectsProvider`) implementation, per §6's
"no shared base class" rule, just two HTTP client instances backing it.
GitHub's GraphQL rate-limit response shape differs from REST's header-based
one (a `rateLimit { cost remaining resetAt }` field in the response body,
not headers) — the adapter's rate-limit-cache write (§8) must handle both
shapes, keyed by a distinct `bucket` value (`"rest"` vs `"graphql"`) in
`rate_limit_cache`, consistent with §4's "GitHub has separate REST/GraphQL/
search buckets" note.

---

## Design — `wscompat` channel wiring

New file `channels_github.go`, following `channels.go`'s existing
`register*Channels(r *Registry, client ...)` pattern exactly — decode
`args[0]` into a typed struct, call the gRPC client, map the response,
return. One handler shown per shape; the rest follow identically.

```go
// backend-go/services/api-gateway/internal/adapter/wscompat/channels_github.go
package wscompat

import (
    "context"
    "encoding/json"

    scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"
    gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
    "github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

func registerGitHubChannels(r *Registry, client scmintegrationv1.ScmIntegrationServiceClient) {
    // github.rateLimit — already has a real backing RPC (BUG-012's finding);
    // this is the wiring-only piece.
    r.Register("github.rateLimit", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.GetRateLimitStatus(gatewaygrpc.AttachIdentity(rpcCtx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID}),
            &scmintegrationv1.GetRateLimitStatusRequest{TenantId: id.TenantID, Provider: scmintegrationv1.ScmProvider_SCM_PROVIDER_GITHUB})
        if err != nil {
            return nil, err
        }
        return resp, nil
    })

    // github.mergePR — shape 1 representative.
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
        resp, err := client.MergePullRequest(gatewaygrpc.AttachIdentity(rpcCtx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID}),
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

    // github.project.updateItemField — shape 3 representative; the rest of
    // github.project.* follow this exact decode/call/return shape against
    // their own RPC.
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
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.UpdateProjectItemField(gatewaygrpc.AttachIdentity(rpcCtx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID}),
            &scmintegrationv1.UpdateProjectItemFieldRequest{
                TenantId: id.TenantID, ProjectSlug: in.ProjectSlug, ItemId: in.ItemID,
                Field: &scmintegrationv1.ProjectFieldValue{FieldId: in.FieldID, Kind: in.Kind, Value: in.Value},
            })
        if err != nil {
            return nil, err
        }
        return resp, nil
    })

    // github.startAuthLogin / github.revokeAuth — thin wrappers over the
    // OAuth RPCs BUG-012 confirmed already exist; no new proto here.
    r.Register("github.startAuthLogin", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type startArgs struct{ RedirectURI string `json:"redirectUri"` }
        in, err := decodeArg[startArgs](args, 0)
        if err != nil {
            return nil, err
        }
        resp, err := client.StartOAuthFlow(gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID}),
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
        resp, err := client.RevokeAuth(gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID}),
            &scmintegrationv1.RevokeAuthRequest{TenantId: id.TenantID, Provider: scmintegrationv1.ScmProvider_SCM_PROVIDER_GITHUB})
        if err != nil {
            return nil, err
        }
        return resp, nil
    })

    // ... remaining 18 github.* / github.project.* channels, same shape ...
}
```

`channels.go`'s `RegisterRealChannels` gains one new parameter and call:

```go
func RegisterRealChannels(
    r *Registry,
    // ... existing params ...
    scmClient scmintegrationv1.ScmIntegrationServiceClient,
) {
    // ... existing register*Channels calls ...
    registerGitHubChannels(r, scmClient)
}
```

`main.go`'s composition root dials `scm-integration-service`'s gRPC client
(likely already dialed for `httpgateway.mountSCMRoutes`, per
`scm_routes.go`'s existing pattern — reuse the same client, don't dial
twice) and passes it into `RegisterRealChannels`.

---

## Test plan

- `services/scm-integration-service/internal/usecase/merge_pull_request_test.go` — fake `ScmProvider`, assert `MergePullRequest` maps input/output correctly and propagates `ErrCapabilityUnsupported` when the fake returns it.
- `services/scm-integration-service/internal/adapter/external/github/projects_test.go` — GraphQL request/response fixtures (httptest server) for `ViewProjectTable`/`UpdateItemField`, verifying the field-kind-to-GraphQL-input mapping.
- `services/scm-integration-service/internal/adapter/external/github/rate_limit_test.go` — assert `rate_limit_cache` writes use distinct `"rest"`/`"graphql"` buckets per §8.
- `services/api-gateway/internal/adapter/wscompat/channels_github_test.go` — one test per channel, mirroring `channels_test.go`'s existing shape: fake `ScmIntegrationServiceClient`, assert the channel decodes args correctly and returns the client's response unmodified.
- Contract test: `github.rateLimit` channel and `GET /v1/scm/rate-limit?provider=github` (`scm_routes.go`) both resolve to `GetRateLimitStatus` — assert identical response shape (regression guard against the WS and REST surfaces drifting, same pattern SOL-001 uses for `/admin/api/audit` vs `/v1/auth/audit-log`).
- `buf breaking` in CI against `scmintegration.proto` — all additions here are additive; must pass with zero breaking changes flagged.

## References

- `specs/backend-go/bugs/missing-v1/BUG-012-github-channels-not-implemented.md` — the 24-method gap this solution closes, including the existing-OAuth-flow finding
- `specs/backend-go/tdd/services/scm-integration-service.md` §3,4,6,8,9 — RPC surface, `ScmProvider` port, package layout, rate-limit non-functional requirements, per-tenant credential guarantee
- `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md` — usecase/port layering
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md` — gRPC conventions, `buf breaking`
- `backend-go/proto/orca/scmintegration/v1/scmintegration.proto` — current 8-RPC surface this solution extends
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` — `register*Channels` pattern to mirror
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` — `ChannelHandler`, `decodeArg`, `Identity`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/scm_routes.go` — existing REST proxy pattern for the same service, `parseSCMProvider`
- `specs/frontend/api/rpc-catalog.md` — authoritative `github.*` method catalog
