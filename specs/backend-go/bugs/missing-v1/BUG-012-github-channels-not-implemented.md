# BUG-012: `github.*` channels not implemented in backend-go

**Service:** `api-gateway` (WS compat layer) / `scm-integration-service` (owning gRPC service)
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Severity:** High — GitHub integration (issues, PRs, GitHub Projects) is core to the product; every one of these calls fails.
**Symptom:** Every `github.*` RPC below (except auth) falls through to `registry.go`'s `notImplementedHandler` and returns an error immediately.
**Status:** ❌ Open

---

## Description

`wscompat/channels.go` registers 13 real channels total (`annotation.*`, `task.{create,get}`,
`git.{status,diff}`, `automation.runNow`, `preflight.check`, `devServer.{list,add}`,
`fleet.health.checkAll`, `crashReports.getLatestPending`, `rateLimits.get` — see file header
comment at `channels.go:16-19`). None of them is a `github.*` channel:

```
grep -n '"github\.' backend-go/services/api-gateway/internal/adapter/wscompat/channels.go
```

returns zero matches for any `r.Register("github....")` call — the only `github` hits in that
file are unrelated Go import paths (`channels.go:27-34`, e.g.
`annotationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/annotation/v1"`), not registered
channels. Every `github.*` method the frontend calls therefore hits
`notImplementedHandler` (`registry.go`) and returns an error immediately.

The only piece of GitHub-adjacent surface that **is** wired in backend-go today is
`scm-integration-service`'s REST proxy at `/v1/scm`
(`backend-go/services/api-gateway/internal/adapter/httpgateway/scm_routes.go`), registered in
`NewDefaultServiceRegistry()` as `RouteWired`
(`backend-go/services/api-gateway/internal/domain/registry.go:91`). That proxy exposes exactly
8 generic, provider-parameterized endpoints (`scm_routes.go:23-32`):
`GET /issues`, `POST /pull-requests`, `GET /pull-requests`, `GET /rate-limit`,
`GET /auth-status`, `POST /oauth/{start,complete}`, `POST /oauth/revoke` — backed by
`ScmIntegrationService`'s 8 RPCs (`backend-go/proto/orca/scmintegration/v1/scmintegration.proto:10-27`):
`ListIssues`, `CreatePullRequest`, `ListPullRequests`, `GetRateLimitStatus`, `GetAuthStatus`,
`StartOAuthFlow`, `CompleteOAuthFlow`, `RevokeAuth`. None of these is wired into a `github.*`
WS channel, and none of the 24 frontend-called methods below except rate-limit/auth has a
1:1 backing RPC yet — see the Notes column.

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `github.mergePR` | `components/PullRequestPage.tsx:3967` | No matching RPC on `ScmIntegrationService`; would need a `MergePullRequest` RPC. |
| `github.prForBranch` | `store/slices/github-pr-refresh-owner-routing.test.ts:157`, `web/web-preload-api.ts:284,385` | `ListPullRequests` (`scmintegration.proto:13`) returns PRs for a repo but has no branch filter — partial match at best, would need a new RPC or a `head_branch` filter added. |
| `github.project.addIssueCommentBySlug` | `components/github-project/slug-dialog/Comments.tsx:241` | No GitHub Projects/issue-comment RPC exists anywhere in `scmintegration.proto`. |
| `github.project.clearItemField` | `store/slices/github.ts:2277` | No GitHub Projects field-write RPC exists. |
| `github.project.deleteIssueCommentBySlug` | `components/github-project/slug-dialog/Comments.tsx:81` | No matching RPC. |
| `github.project.listAccessible` | `components/github-project/ProjectPicker.tsx:71` | No GitHub Projects list RPC — `ScmIntegrationService` has no Projects-v2 concept at all. |
| `github.project.listAssignableUsersBySlug` | `hooks/useGitHubSlugMetadata.ts:182` | No matching RPC. |
| `github.project.listIssueTypesBySlug` | `components/github-project/ProjectCell.tsx:347` | No matching RPC. |
| `github.project.listLabelsBySlug` | `hooks/useGitHubSlugMetadata.ts:87` | No matching RPC. |
| `github.project.listViews` | `components/github-project/ProjectViewWrapper.tsx:80`, `ProjectPicker.tsx:84` | No matching RPC. |
| `github.project.resolveRef` | `components/github-project/ProjectPicker.tsx:98` | No matching RPC. |
| `github.project.updateIssueBySlug` | `store/slices/github.ts:2435` | `Issue` message (`scmintegration.proto:38-43`) exists but there is no Update RPC, and no by-slug addressing scheme. |
| `github.project.updateIssueCommentBySlug` | `components/github-project/slug-dialog/Comments.tsx:103` | No matching RPC. |
| `github.project.updateIssueTypeBySlug` | `store/slices/github.ts:2521` | No matching RPC — GitHub "issue types" concept isn't modeled in the proto. |
| `github.project.updateItemField` | `store/slices/github.ts:2219` | No matching RPC. |
| `github.project.updatePullRequestBySlug` | `store/slices/github.ts:2400` | `PullRequest` message (`scmintegration.proto:66-70`) exists (id/url/state only) but there is no Update RPC. |
| `github.project.viewTable` | `store/slices/github.ts:2119` | No matching RPC — GitHub Projects v2 table view has no server-side representation yet. |
| `github.project.workItemDetailsBySlug` | `components/github-project/slug-dialog/SlugDialogBody.tsx:67` | No matching RPC. |
| `github.rateLimit` | `components/github/github-rate-limit-display.tsx:97`, `web/web-preload-api.ts:312,413` | **Real backing RPC exists**: `GetRateLimitStatus` (`scmintegration.proto:14`, proxied at `GET /v1/scm/rate-limit`, `scm_routes.go:116-132`) — this is a thin wrapper task, not new backend work. |
| `github.removePRReviewers` | `web/web-preload-api.ts:304-305,405` (`web-preload-api.test.ts:2609`) | No reviewer-management RPC exists. |
| `github.repoSlug` | `components/settings/repository-icon-github.ts:47` | No matching RPC — repo-slug resolution isn't modeled. |
| `github.requestPRReviewers` | `web/web-preload-api.ts:304,405` (`web-preload-api.test.ts:2603`) | No reviewer-management RPC exists. |
| `github.setPRAutoMerge` | `components/PullRequestPage.tsx` (action wired via `web-preload-api.ts:302,403`) | No auto-merge RPC exists. |
| `github.updateIssue` | `components/task-page-github-assignee-cells.tsx:197,800` | `Issue` message exists (`scmintegration.proto:38-43`) but no Update RPC. |

**Not in this list but worth noting for scope:** `github.startAuthLogin`/`github.revokeAuth`
(`components/settings/WebModeCliAuthSection.tsx:51`) are *not* orphaned in the same way — see
Dispatch model below. `scm-integration-service` already has a genuine OAuth-flow equivalent
(`StartOAuthFlow`/`CompleteOAuthFlow`/`RevokeAuth`/`GetAuthStatus`,
`scmintegration.proto:23-26`, proxied at `POST /v1/scm/oauth/{start,complete,revoke}` and
`GET /v1/scm/auth-status`, `scm_routes.go:29-31,28`) — it just needs a WS channel wrapper
(or the frontend needs to be pointed at the REST endpoints), not new backend design work.

---

## Dispatch model

Old TS backend (`specs/frontend/api/backend-agent-execution-boundary.md:124`): 🏠 **backend
self-executes** — ran the `gh` CLI via `child_process.execFile` **in the backend process**,
using the backend host's shared OS keychain. This was **not** relayed to the Dev Server Agent,
which was a documented drift from the HLD's Control-Plane/Data-Plane design (a hard guard,
`assertLocalGhCliAllowed()`, was later added to fail safe under `ORCA_MULTI_USER=1` rather than
fix the underlying design gap — same doc, same line).

**Architecturally interesting for backend-go:** `scm-integration-service`'s proto explicitly
states its design is a *direct per-tenant OAuth API client*, **not** a CLI shell-out —
`backend-go/proto/orca/scmintegration/v1/scmintegration.proto:7-8`: "GitHub/GitLab/Bitbucket/
Azure DevOps/Gitea via direct per-tenant OAuth API clients — NOT a CLI shell-out (closes TS Gap
1)." Its TDD doc (`specs/backend-go/tdd/services/scm-integration-service.md:6,20,238-250`)
confirms this exists specifically to close "TS Gap 1" (the old backend's shared-credential
`gh`/`glab` CLI shell-out) with per-`(tenant_id, provider, user_id)`-scoped credentials and
minimum-necessary OAuth scopes. So backend-go's *intended* architecture here is strictly better
than the old backend's for these 24 methods — it just isn't built out yet: none of the GitHub
Projects / PR-merge / reviewer-management / issue-update surface has a corresponding RPC on
`ScmIntegrationService` today (`scmintegration.proto:10-27` lists only 8 RPCs, none of them
GitHub-Projects-shaped).

**EXCEPTION carried over for context only (not a backend-go finding):** in the old backend,
`github.startAuthLogin`/`revokeAuth` were the one part of `github.*` that genuinely relayed to
the Dev Server Agent — `gh auth login` run on the dev server with per-user `GH_CONFIG_DIR`
isolation (`backend-agent-execution-boundary.md:126`). Since backend-go's design is
OAuth-web-flow-based from the start (`scmintegration.proto:16-26`, TDD §9.1), it doesn't need an
Agent-relay equivalent at all — `StartOAuthFlow`/`CompleteOAuthFlow`/`RevokeAuth`/
`GetAuthStatus` already exist server-side and are REST-proxied (`scm_routes.go:29-31,28`); they
just aren't exposed as `github.startAuthLogin`/`github.revokeAuth` WS channels yet.

---

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:1-19` — registered-channel inventory (no `github.*`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` — `notImplementedHandler`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/scm_routes.go:22-267` — the real `/v1/scm` REST proxy and its 8 handlers
- `backend-go/services/api-gateway/internal/domain/registry.go:91` — `/v1/scm` → `scm-integration-service`, `RouteWired`
- `backend-go/proto/orca/scmintegration/v1/scmintegration.proto:7-27,38-159` — `ScmIntegrationService`'s full RPC/message surface
- `specs/backend-go/tdd/services/scm-integration-service.md:6,20,238-250` — design rationale (closes TS Gap 1)
- `specs/frontend/api/backend-agent-execution-boundary.md:124,126,180` — old backend's `github.*` dispatch model and the auth-login exception
- `specs/frontend/api/rpc-catalog.md` — authoritative RPC catalog (`github.*` namespace)
- `specs/backend-go/bugs/api-v1/BUG-002-missing-channel-registrations.md` — sibling report, same bug shape
