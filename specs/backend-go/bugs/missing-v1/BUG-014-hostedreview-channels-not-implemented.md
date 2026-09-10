# BUG-014: `hostedReview.*` channels not implemented in backend-go

**Service:** `api-gateway` (WS compat layer) / `scm-integration-service` (owning gRPC service)
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Severity:** Medium
**Symptom:** Every `hostedReview.*` RPC below falls through to `registry.go`'s `notImplementedHandler` and returns an error immediately.
**Status:** ✅ Resolved — see TASK-087–090 (4 task(s), all DONE) for implementation evidence.

---

## Description

`wscompat/channels.go` registers 13 real channels total (`annotation.*`, `task.{create,get}`,
`git.{status,diff}`, `automation.runNow`, `preflight.check`, `devServer.{list,add}`,
`fleet.health.checkAll`, `crashReports.getLatestPending`, `rateLimits.get` — file header comment
at `channels.go:16-19`). None of them is a `hostedReview.*` channel:

```
grep -n '"hostedReview\.' backend-go/services/api-gateway/internal/adapter/wscompat/channels.go
```

returns zero matches. Every `hostedReview.*` method falls through to `notImplementedHandler`
(`registry.go`).

Per `rpc-catalog.md`, `hostedReview.*` is the provider-agnostic (GitHub/GitLab/Bitbucket/Azure
DevOps/Gitea) hosted PR/MR surface — a thin dispatcher over the same per-provider capabilities
`github.*`/`gitlab.*` expose. The only piece of that surface wired in backend-go today is
`scm-integration-service`'s generic REST proxy at `/v1/scm`
(`backend-go/services/api-gateway/internal/adapter/httpgateway/scm_routes.go`, `RouteWired` per
`backend-go/services/api-gateway/internal/domain/registry.go:91`), which is provider-parameterized
via `ScmProvider` (`scmintegration.proto:29-36`, covering all 5 providers) and does have direct
RPC matches for two of the three methods here.

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `hostedReview.create` | `store/slices/hosted-review.ts:279` (see also comment at `components/workspace/git/PullRequestForm.tsx:14`) | **Plausible backing RPC exists**: `CreatePullRequest` (`scmintegration.proto:12`, request/response at `scmintegration.proto:55-74`), proxied at `POST /v1/scm/pull-requests` (`scm_routes.go:68-95`) and provider-generic via `ScmProvider`. Needs a WS wrapper that maps `hostedReview.create`'s args to `CreatePullRequestRequest`, not new backend RPC design. |
| `hostedReview.forBranch` | `store/slices/hosted-review.ts:360,368` (see also comment at `components/workspace/git/PullRequestList.tsx:7`) | **Plausible backing RPC exists**: `ListPullRequests` (`scmintegration.proto:13`, request/response at `scmintegration.proto:76-84`), proxied at `GET /v1/scm/pull-requests` (`scm_routes.go:97-114`). Same caveat as `github.prForBranch` (BUG-012): `ListPullRequests` has no branch filter today, only `repo` — would need a filter param added, or client-side filtering, to be branch-scoped. |
| `hostedReview.getCreationEligibility` | `store/slices/hosted-review.ts:253,262` | No matching RPC. This is a pre-flight check (can a PR/MR be created for the current branch state — auth connected, branch pushed, no existing open review, etc.) with no equivalent on `ScmIntegrationService`; would likely compose `GetAuthStatus` (`scmintegration.proto:23`) plus new logic, not a single existing RPC. |

---

## Dispatch model

Old TS backend (`specs/frontend/api/backend-agent-execution-boundary.md:127`): 🏠 **mixed** —
GitHub/GitLab-branch reviews went through the same self-executed CLI path as `github.*`/`gitlab.*`
(BUG-012/BUG-013's `gh`/`glab` `child_process.execFile` pattern, backend-host shared OS keychain);
Bitbucket/Azure DevOps/Gitea reviews went through per-user HTTP clients authenticated with
`WebCredentialStore` tokens — the same store backing the `credentials.*` namespace
(`backend-agent-execution-boundary.md:151`: AES-256-GCM `.enc` files on the backend host's own
filesystem, per-user, gated behind `ORCA_MULTI_USER=1`, explicitly excluding GitHub/GitLab which
use the shared CLI keychain instead). If a `credentials.*` bug report exists for backend-go, it
covers the sibling gap for these three non-CLI providers' credential storage — `hostedReview.*`'s
Bitbucket/Azure DevOps/Gitea paths depend on that same credential surface being implemented.

`pr_created` stats were written to a local JSON file (`orca-stats.json`), not Postgres
(`backend-agent-execution-boundary.md:127`) — noted here only as a "don't replicate this" flag:
if `hostedReview.*` usage stats are tracked at all in backend-go, they belong in Postgres per
this repo's general data-model conventions, not a local file.

Same architectural note as BUG-012/BUG-013: `scm-integration-service`'s design is a direct
per-tenant OAuth API client, not a CLI shell-out (`scmintegration.proto:7-8`,
`specs/backend-go/tdd/services/scm-integration-service.md:6,20,238-250`) — for the two methods
with a plausible existing RPC (`create`, `forBranch`) this is a strictly better foundation than
the old backend's mixed CLI/HTTP-client split, since `CreatePullRequest`/`ListPullRequests`
already work uniformly across all 5 providers instead of branching on CLI-vs-HTTP-client per
provider.

---

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:1-19` — registered-channel inventory (no `hostedReview.*`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` — `notImplementedHandler`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/scm_routes.go:22-114` — the real `/v1/scm` REST proxy, `pull-requests` handlers
- `backend-go/services/api-gateway/internal/domain/registry.go:91` — `/v1/scm` → `scm-integration-service`, `RouteWired`
- `backend-go/proto/orca/scmintegration/v1/scmintegration.proto:7-29,55-84` — `ScmIntegrationService`'s `CreatePullRequest`/`ListPullRequests` RPCs and `ScmProvider` enum
- `specs/backend-go/tdd/services/scm-integration-service.md:6,20,238-250` — design rationale (closes TS Gap 1)
- `specs/frontend/api/backend-agent-execution-boundary.md:127,151` — old backend's `hostedReview.*` dispatch model, `pr_created` local-file stats, and `credentials.*` linkage
- `specs/frontend/api/rpc-catalog.md` — authoritative RPC catalog (`hostedReview.*` namespace)
- `specs/backend-go/bugs/missing-v1/BUG-012-github-channels-not-implemented.md`, `BUG-013-gitlab-channels-not-implemented.md` — sibling reports, same pattern
- `specs/backend-go/bugs/api-v1/BUG-002-missing-channel-registrations.md` — original example of this bug shape
