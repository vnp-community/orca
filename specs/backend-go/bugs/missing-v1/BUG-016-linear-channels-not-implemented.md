# BUG-016: `linear.*` channels not implemented in backend-go

**Service:** `api-gateway` (WS compat layer) / `issue-tracking-service` (owning gRPC service)
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Severity:** Medium-High
**Symptom:** Every `linear.*` RPC below falls through to `registry.go`'s `notImplementedHandler` and returns an error immediately.
**Status:** ✅ Resolved — see TASK-102–107 (6 task(s), all DONE) for implementation evidence.

---

## Description

`wscompat/channels.go` registers 13 real channels total (`annotation.*`, `task.{create,get}`,
`git.{status,diff}`, `automation.runNow`, `preflight.check`, `devServer.{list,add}`,
`fleet.health.checkAll`, `crashReports.getLatestPending`, `rateLimits.get` — file header comment
at `channels.go:16-19`). None of them is a `linear.*` channel:

```
grep -n '"linear\.' backend-go/services/api-gateway/internal/adapter/wscompat/channels.go
```

returns zero matches. All 19 `linear.*` methods the frontend calls
(`frontend/src/renderer/src/runtime/runtime-linear-client.ts`) fall through to
`notImplementedHandler` (`registry.go`).

Same owning service as `jira.*` (BUG-015): `issue-tracking-service`'s generic,
provider-parameterized REST proxy at `/v1/issues`
(`backend-go/services/api-gateway/internal/adapter/httpgateway/issue_tracking_routes.go:19-25`,
`RouteWired` per `backend-go/services/api-gateway/internal/domain/registry.go`), backed by only 3
RPCs — `ListIssues`, `CreateIssue`, `LinkIssue`
(`backend-go/proto/orca/issuetracking/v1/issuetracking.proto:10-13`). `IssueProvider` includes
`ISSUE_PROVIDER_LINEAR` (`issuetracking.proto:18`), so `ListIssues`/`CreateIssue` already accept a
Linear selector — but the generic `Issue` message it returns (`issuetracking.proto:21-26`: only
`id`, `title`, `state`, `url`) is far thinner than the frontend's `LinearIssue` type
(`frontend/src/shared/types.ts:1598-1618+`: `identifier`, `workspaceId`, `state` object with
`name`/`type`/`color`, `team`, `project`, `subIssues`, `labels`, etc.). There is no
workspace/auth concept in the proto at all — no equivalent of `linear.connect`/`status`/
`selectWorkspace`/`testConnection`, and `issue-tracking-service`'s own `CredentialResolver` doc
comment confirms "no 'connect Jira/Linear' flow exists" yet
(`backend-go/services/issue-tracking-service/internal/usecase/ports.go:42-57`, README "Known
gaps"). Credentials are meant to be resolved per-request via `credential-broker-service`
(Vault-backed), not managed through any of these RPCs
(`backend-go/services/issue-tracking-service/internal/adapter/credential/client.go:1-21,62-80`).

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `linear.status` | `runtime/runtime-linear-client.ts:118` | No matching RPC. No connection/workspace-status concept in `issuetracking.proto`. |
| `linear.testConnection` | `runtime/runtime-linear-client.ts:132` | No matching RPC. |
| `linear.connect` | `runtime/runtime-linear-client.ts:149` | No matching RPC. No "connect" flow exists anywhere in `issue-tracking-service` (`ports.go:42-57` doc comment). |
| `linear.disconnect` | `runtime/runtime-linear-client.ts:168` | No matching RPC. |
| `linear.selectWorkspace` | `runtime/runtime-linear-client.ts:187` | No matching RPC — proto has no multi-workspace concept at all. |
| `linear.searchIssues` | `runtime/runtime-linear-client.ts:207` | No matching RPC — `ListIssues` takes only `project_key` (`issuetracking.proto:28-32`), no free-text search. |
| `linear.listIssues` | `runtime/runtime-linear-client.ts:247` | **Partial backing RPC exists**: `ListIssues` (`issuetracking.proto:10,28-36`) already accepts `ISSUE_PROVIDER_LINEAR` and a `project_key`, but returns the thin generic `Issue` shape, not `LinearIssue`, has no `filter`/`limit`/`workspaceId`/`attributeFilter` params, and no `{items, errors, hasMore}` collection envelope the frontend expects. |
| `linear.createIssue` | `runtime/runtime-linear-client.ts:271` | **Partial backing RPC exists**: `CreateIssue` (`issuetracking.proto:11,38-48`) covers `project_key`/`title`/`description`, but has no `teamId`, `stateId`, `priority`, `assigneeId`, `labelIds`, or `parentIssueId` params the frontend sends. |
| `linear.createProject` | `runtime/runtime-linear-client.ts:418` | No matching RPC — no project-mutation concept at all (proto has no `Project` message). |
| `linear.getIssue` | `runtime/runtime-linear-client.ts:300` | No matching RPC — no get-by-id lookup in the proto. |
| `linear.updateIssue` | `runtime/runtime-linear-client.ts:317` | No matching RPC — no update/mutate RPC exists for issues at all. |
| `linear.addIssueComment` | `runtime/runtime-linear-client.ts:334` | No matching RPC — no comment concept in the proto. |
| `linear.issueComments` | `runtime/runtime-linear-client.ts:350` | No matching RPC. |
| `linear.listTeams` | `runtime/runtime-linear-client.ts:365` | No matching RPC — no team concept in the proto. |
| `linear.getProject` | `runtime/runtime-linear-client.ts:434` | No matching RPC — no project-read concept. |
| `linear.getCustomView` | `runtime/runtime-linear-client.ts:498` | No matching RPC — no custom-view concept at all. |
| `linear.teamStates` | `runtime/runtime-linear-client.ts:560` | No matching RPC — no workflow-state concept. |
| `linear.teamLabels` | `runtime/runtime-linear-client.ts:574` | No matching RPC. |
| `linear.teamMembers` | `runtime/runtime-linear-client.ts:590` | No matching RPC. |

---

## Dispatch model

Old TS backend (`specs/frontend/api/backend-agent-execution-boundary.md:128`): 🏠 **backend-local,
no CLI/agent** — GraphQL SDK calls direct from the backend process to Linear's API. Same
credential-storage story as `jira.*`: per-service encrypted files (`~/.orca/linear-*`, Electron
`safeStorage`) or `WebCredentialStore` in multi-user mode — **not Postgres**. One side effect
noted for context (not in this frontend's `callRuntimeRpc` call list, so not tracked as a missing
channel here): `linear.resolveCurrentIssue` used to write `linkedLinearIssueWorkspaceId` onto the
worktree's blob row (`backend-agent-execution-boundary.md:128`).

backend-go's `issue-tracking-service` follows the same "backend calls Linear directly" shape (a
hand-rolled GraphQL client over `net/http`, bearer auth,
`backend-go/services/issue-tracking-service/internal/adapter/linear/client.go:1-42`) but sources
credentials very differently: per-request resolution against `credential-broker-service` (Vault
KV v2), not encrypted local files
(`backend-go/services/issue-tracking-service/internal/adapter/credential/client.go:1-21,62-80`).
That's an architecturally sound successor to the old file-based scheme, but it only closes the
credential-storage gap — the RPC surface itself (workspaces, teams, comments, custom views,
projects, workflow states) still needs to be built from near-zero: 3 generic RPCs today vs. 19
Linear-shaped frontend methods.

---

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:1-19` — registered-channel inventory (no `linear.*`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` — `notImplementedHandler`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/issue_tracking_routes.go:1-129` — the real `/v1/issues` REST proxy and its 3 handlers
- `backend-go/services/api-gateway/internal/domain/registry.go` — `/v1/issues` → `issue-tracking-service`, `RouteWired`
- `backend-go/proto/orca/issuetracking/v1/issuetracking.proto:1-58` — `IssueTrackingService`'s full RPC/message surface (3 RPCs, generic `Issue`/`IssueProvider`)
- `backend-go/services/issue-tracking-service/internal/usecase/ports.go:1-77` — `IssueTrackerProvider`/`CredentialResolver` ports; doc comment confirms no connect flow exists yet
- `backend-go/services/issue-tracking-service/internal/adapter/linear/client.go:1-42` — real Linear GraphQL client (`ListIssues`/`CreateIssue` only)
- `backend-go/services/issue-tracking-service/internal/adapter/credential/client.go:1-80` — Vault/credential-broker-backed credential resolution (replaces old encrypted-file scheme)
- `frontend/src/renderer/src/runtime/runtime-linear-client.ts:113-597` — all 19 frontend call sites
- `frontend/src/shared/types.ts:1598-1618` — `LinearIssue` shape the frontend expects, vs. the proto's thin generic `Issue`
- `specs/frontend/api/backend-agent-execution-boundary.md:128` — old backend's `jira.*`/`linear.*` dispatch model, credential storage, and the `linear.resolveCurrentIssue` side effect
- `specs/frontend/api/rpc-catalog.md:319-337` — authoritative RPC catalog (`linear.*` namespace)
- `specs/backend-go/bugs/api-v1/BUG-002-missing-channel-registrations.md` — original example of this bug shape
- `specs/backend-go/bugs/missing-v1/BUG-015-jira-channels-not-implemented.md` — sibling report, same owning service
