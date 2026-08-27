# BUG-015: `jira.*` channels not implemented in backend-go

**Service:** `api-gateway` (WS compat layer) / `issue-tracking-service` (owning gRPC service)
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Severity:** Medium-High
**Symptom:** Every `jira.*` RPC below falls through to `registry.go`'s `notImplementedHandler` and returns an error immediately.
**Status:** ❌ Open

---

## Description

`wscompat/channels.go` registers 13 real channels total (`annotation.*`, `task.{create,get}`,
`git.{status,diff}`, `automation.runNow`, `preflight.check`, `devServer.{list,add}`,
`fleet.health.checkAll`, `crashReports.getLatestPending`, `rateLimits.get` — file header comment
at `channels.go:16-19`). None of them is a `jira.*` channel:

```
grep -n '"jira\.' backend-go/services/api-gateway/internal/adapter/wscompat/channels.go
```

returns zero matches. All 19 `jira.*` methods the frontend calls
(`frontend/src/renderer/src/runtime/runtime-jira-client.ts`) fall through to
`notImplementedHandler` (`registry.go`).

The only Jira-adjacent surface wired in backend-go is `issue-tracking-service`'s generic,
provider-parameterized REST proxy at `/v1/issues`
(`backend-go/services/api-gateway/internal/adapter/httpgateway/issue_tracking_routes.go:19-25`,
`RouteWired` per `backend-go/services/api-gateway/internal/domain/registry.go`), backed by
`IssueTrackingService`'s 3 RPCs — `ListIssues`, `CreateIssue`, `LinkIssue`
(`backend-go/proto/orca/issuetracking/v1/issuetracking.proto:10-13`). `IssueProvider` includes
`ISSUE_PROVIDER_JIRA` (`issuetracking.proto:17`), so `ListIssues`/`CreateIssue` already accept a
Jira selector — but the generic `Issue` message it returns (`issuetracking.proto:21-26`: only
`id`, `title`, `state`, `url`) is far thinner than the frontend's `JiraIssue` type
(`frontend/src/shared/jira-types.ts:94-111`: `key`, `siteId`, `project`, `issueType`, `status`
object, `labels`, `assignee`, `reporter`, `priority`, timestamps, etc.). There is no site/auth
concept in the proto at all — no equivalent of `jira.connect`/`status`/`selectSite`/
`testConnection`, and `issue-tracking-service`'s own `CredentialResolver` doc comment confirms
"no 'connect Jira/Linear' flow exists" yet
(`backend-go/services/issue-tracking-service/internal/usecase/ports.go:42-57`, README "Known
gaps"). Credentials are meant to be resolved per-request via `credential-broker-service`
(Vault-backed), not stored/managed through any of these RPCs
(`backend-go/services/issue-tracking-service/internal/adapter/credential/client.go:1-21,37-51`).

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `jira.status` | `runtime/runtime-jira-client.ts:54` | No matching RPC. No connection/site-status concept in `issuetracking.proto`. |
| `jira.connect` | `runtime/runtime-jira-client.ts:64` | No matching RPC. No "connect" flow exists anywhere in `issue-tracking-service` (`ports.go:42-57` doc comment). |
| `jira.disconnect` | `runtime/runtime-jira-client.ts:74` | No matching RPC. |
| `jira.selectSite` | `runtime/runtime-jira-client.ts:90` | No matching RPC — proto has no multi-site concept at all. |
| `jira.testConnection` | `runtime/runtime-jira-client.ts:105` | No matching RPC. |
| `jira.searchIssues` | `runtime/runtime-jira-client.ts:124` | No matching RPC. `ListIssues` takes only `project_key` (`issuetracking.proto:28-32`), no JQL/free-text search. |
| `jira.listIssues` | `runtime/runtime-jira-client.ts:137` | **Partial backing RPC exists**: `ListIssues` (`issuetracking.proto:10,28-36`) already accepts `ISSUE_PROVIDER_JIRA` and a `project_key`, but returns the thin generic `Issue` shape, not `JiraIssue`, and has no `filter`/`limit`/`siteId` params the frontend sends. |
| `jira.getIssue` | `runtime/runtime-jira-client.ts:149` | No matching RPC — no get-by-key lookup in the proto. |
| `jira.createIssue` | `runtime/runtime-jira-client.ts:159` | **Partial backing RPC exists**: `CreateIssue` (`issuetracking.proto:11,38-48`) covers `project_key`/`title`/`description`, but has no `issueType`, `assignee`, `priority`, or custom-field params `JiraCreateIssueArgs` carries. |
| `jira.updateIssue` | `runtime/runtime-jira-client.ts:172` | No matching RPC — no update/mutate RPC exists for issues at all. |
| `jira.addIssueComment` | `runtime/runtime-jira-client.ts:185` | No matching RPC — no comment concept in the proto. |
| `jira.issueComments` | `runtime/runtime-jira-client.ts:199` | No matching RPC. |
| `jira.listProjects` | `runtime/runtime-jira-client.ts:209` | No matching RPC — no project-listing RPC. |
| `jira.listIssueTypes` | `runtime/runtime-jira-client.ts:223` | No matching RPC. |
| `jira.listCreateFields` | `runtime/runtime-jira-client.ts:236` | No matching RPC — no create-metadata RPC. |
| `jira.listPriorities` | `runtime/runtime-jira-client.ts:250` | No matching RPC. |
| `jira.listAssignableUsers` | `runtime/runtime-jira-client.ts:269` | No matching RPC — no user-lookup RPC in this service at all. |
| `jira.listTransitions` | `runtime/runtime-jira-client.ts:281` | No matching RPC — no workflow-transition concept. |
| `jira.getProjectStatusOrder` | `runtime/runtime-jira-client.ts:293` | No matching RPC. |

---

## Dispatch model

Old TS backend (`specs/frontend/api/backend-agent-execution-boundary.md:128`): 🏠 **backend-local,
no CLI/agent** — HTTPS REST calls direct from the backend process to Jira Cloud's REST API v3.
Credentials lived in per-service encrypted files (`~/.orca/jira-*`, Electron `safeStorage`) or
`WebCredentialStore` in multi-user mode — **not Postgres**.

backend-go's `issue-tracking-service` follows the same "backend calls Jira directly" shape (real
`net/http` Basic-Auth client, `backend-go/services/issue-tracking-service/internal/adapter/jira/client.go:1-50`)
but sources credentials very differently: per-request resolution against
`credential-broker-service` (Vault KV v2), not encrypted local files
(`backend-go/services/issue-tracking-service/internal/adapter/credential/client.go:1-21,62-80`).
That's an architecturally sound successor to the old file-based scheme, but it only closes the
credential-storage gap — the RPC surface itself (site selection, comments, transitions, create
metadata, user search, status ordering) still needs to be built from near-zero: 3 generic RPCs
today vs. 19 Jira-shaped frontend methods.

---

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:1-19` — registered-channel inventory (no `jira.*`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` — `notImplementedHandler`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/issue_tracking_routes.go:1-129` — the real `/v1/issues` REST proxy and its 3 handlers
- `backend-go/services/api-gateway/internal/domain/registry.go` — `/v1/issues` → `issue-tracking-service`, `RouteWired`
- `backend-go/proto/orca/issuetracking/v1/issuetracking.proto:1-58` — `IssueTrackingService`'s full RPC/message surface (3 RPCs, generic `Issue`/`IssueProvider`)
- `backend-go/services/issue-tracking-service/internal/usecase/ports.go:1-77` — `IssueTrackerProvider`/`CredentialResolver` ports; doc comment confirms no connect flow exists yet
- `backend-go/services/issue-tracking-service/internal/adapter/jira/client.go:1-50` — real Jira Cloud REST v3 client (`ListIssues`/`CreateIssue` only)
- `backend-go/services/issue-tracking-service/internal/adapter/credential/client.go:1-80` — Vault/credential-broker-backed credential resolution (replaces old encrypted-file scheme)
- `frontend/src/renderer/src/runtime/runtime-jira-client.ts:51-297` — all 19 frontend call sites
- `frontend/src/shared/jira-types.ts:1-111` — `JiraIssue`/`JiraSite`/`JiraTransition`/etc. shapes the frontend expects, vs. the proto's thin generic `Issue`
- `specs/frontend/api/backend-agent-execution-boundary.md:128` — old backend's `jira.*`/`linear.*` dispatch model and credential storage
- `specs/frontend/api/rpc-catalog.md:295-313` — authoritative RPC catalog (`jira.*` namespace)
- `specs/backend-go/bugs/api-v1/BUG-002-missing-channel-registrations.md` — original example of this bug shape
- `specs/backend-go/bugs/missing-v1/BUG-016-linear-channels-not-implemented.md` — sibling report, same owning service
