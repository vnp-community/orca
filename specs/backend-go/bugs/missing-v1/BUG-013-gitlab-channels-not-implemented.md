# BUG-013: `gitlab.*` channels not implemented in backend-go

**Service:** `api-gateway` (WS compat layer) / `scm-integration-service` (owning gRPC service)
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Severity:** Medium
**Symptom:** Every `gitlab.*` RPC below (except auth) falls through to `registry.go`'s `notImplementedHandler` and returns an error immediately.
**Status:** ✅ Resolved — see TASK-083–086 (4 task(s), all DONE) for implementation evidence.

---

## Description

`wscompat/channels.go` registers 13 real channels total (`annotation.*`, `task.{create,get}`,
`git.{status,diff}`, `automation.runNow`, `preflight.check`, `devServer.{list,add}`,
`fleet.health.checkAll`, `crashReports.getLatestPending`, `rateLimits.get` — file header comment
at `channels.go:16-19`). None of them is a `gitlab.*` channel:

```
grep -n '"gitlab\.' backend-go/services/api-gateway/internal/adapter/wscompat/channels.go
```

returns zero matches. Every `gitlab.*` method the frontend calls falls through to
`notImplementedHandler` (`registry.go`).

As with `github.*` (BUG-012), the only GitLab-adjacent surface wired in backend-go is
`scm-integration-service`'s generic, provider-parameterized REST proxy at `/v1/scm`
(`backend-go/services/api-gateway/internal/adapter/httpgateway/scm_routes.go`, `RouteWired` per
`backend-go/services/api-gateway/internal/domain/registry.go:91`), backed by `ScmIntegrationService`'s
8 RPCs (`backend-go/proto/orca/scmintegration/v1/scmintegration.proto:10-27`). `ScmProvider` includes
`SCM_PROVIDER_GITLAB` (`scmintegration.proto:32`), so `GetRateLimitStatus` already works
provider-generically for GitLab too — but nothing in the proto models GitLab's merge-request or
discussion-thread concepts.

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `gitlab.listMRs` | `runtime/runtime-gitlab-client.ts:151`, `lib/gitlab-work-item-source-lookup.ts:72`, `web/web-preload-api.ts:357,435` | No matching RPC. `ListPullRequests` (`scmintegration.proto:13`) is GitHub/generic-PR-shaped (`PullRequest{id,url,state}`, `scmintegration.proto:66-70`) and has no MR-specific fields (source/target branch, discussions, approvals). |
| `gitlab.rateLimit` | `components/gitlab/gitlab-rate-limit-display.tsx:65`, `web/web-preload-api.ts:356,434` | **Real backing RPC exists**: `GetRateLimitStatus` (`scmintegration.proto:14`) is provider-generic and already accepts `SCM_PROVIDER_GITLAB` (`scmintegration.proto:32`), proxied at `GET /v1/scm/rate-limit?provider=gitlab` (`scm_routes.go:116-132`). Thin wrapper task, not new backend work. |
| `gitlab.resolveMRDiscussion` | `components/right-sidebar/ChecksPanel.tsx:363`, `runtime/runtime-gitlab-client.ts:137`, `web/web-preload-api.ts:372,451` | No matching RPC — GitLab discussion threads have no server-side representation at all in `scmintegration.proto`. |
| `gitlab.workItemDetails` | `components/right-sidebar/ChecksPanel.tsx:334`, `runtime/runtime-gitlab-client.ts:50`, `web/web-preload-api.ts:365,443` | No matching RPC. |

---

## Dispatch model

Old TS backend (`specs/frontend/api/backend-agent-execution-boundary.md:125`): 🏠 **backend
self-executes** — identical pattern to `github.*` (BUG-012), via the `glab` CLI in the backend
process, same multi-user guard applying uniformly (no bypass found for `gitlab.*`, unlike the
`github.checkOrcaStarred`/`starOrca` legacy-path gap noted for GitHub).

Same architectural note as BUG-012 applies: `scm-integration-service`'s design is a direct
per-tenant OAuth API client, not a CLI shell-out (`scmintegration.proto:7-8`,
`specs/backend-go/tdd/services/scm-integration-service.md:6,20,238-250`) — a strictly better
starting point than the old backend's shared-credential `glab` shell-out, it just doesn't yet
model MRs, discussions, or work-item details.

`gitlab.startAuthLogin`/`revokeAuth` are the same auth exception as GitHub's (relayed to the Dev
Server Agent in the old backend, `backend-agent-execution-boundary.md:126`); backend-go's
OAuth-web-flow equivalent (`StartOAuthFlow`/`CompleteOAuthFlow`/`RevokeAuth`/`GetAuthStatus`,
`scmintegration.proto:23-26`, proxied at `scm_routes.go:29-31,28`) already covers GitLab via the
same provider-generic RPCs — not tracked as a missing channel here since it isn't in this
namespace's assigned method list, but noted for completeness alongside BUG-012's finding.

---

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:1-19` — registered-channel inventory (no `gitlab.*`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` — `notImplementedHandler`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/scm_routes.go:22-267` — the real `/v1/scm` REST proxy and its 8 handlers
- `backend-go/services/api-gateway/internal/domain/registry.go:91` — `/v1/scm` → `scm-integration-service`, `RouteWired`
- `backend-go/proto/orca/scmintegration/v1/scmintegration.proto:7-27,32,66-70` — `ScmIntegrationService`'s full RPC/message surface, `SCM_PROVIDER_GITLAB`
- `specs/backend-go/tdd/services/scm-integration-service.md:6,20,238-250` — design rationale (closes TS Gap 1)
- `specs/frontend/api/backend-agent-execution-boundary.md:125,126,180` — old backend's `gitlab.*` dispatch model and the auth-login exception
- `specs/frontend/api/rpc-catalog.md` — authoritative RPC catalog (`gitlab.*` namespace)
- `specs/backend-go/bugs/missing-v1/BUG-012-github-channels-not-implemented.md` — sibling report, same pattern
- `specs/backend-go/bugs/api-v1/BUG-002-missing-channel-registrations.md` — original example of this bug shape
