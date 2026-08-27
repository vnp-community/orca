# Missing-v1 Bug Reports — `backend-go` vs. `specs/frontend/api/`

This directory catalogs every gap found by auditing `backend-go` (the Go
microservices rewrite) against the frontend's real, code-grounded API
contract — `specs/frontend/api/rpc-catalog.md` (262 WebSocket RPC methods),
`http-endpoints.md` (plain `fetch()` routes), and
`backend-agent-execution-boundary.md` (execution-model reference used to
classify each namespace's Dispatch model below). `ipc-surface.md` was also
reviewed — it introduces no backend gaps, since the RPC-transport-carrying
namespaces it documents (`window.api.runtime`/`runtimeEnvironments`) are
desktop-local IPC, not something a backend implements.

> **Status (updated 2026-08-27): ✅ 35/36 reports resolved, 1 partial.**
> This was originally a documentation-only pass (no code, no `solutions/`/
> `tasks/` files) — that's no longer true: `solutions/` (36 `SOL-XXX` design
> docs) and `tasks/` (188 `TASK-XXX` execution units) now exist, and per
> `tasks/README.md`'s own tracking, 187/188 tasks are `[x]` DONE. Only
> `BUG-006`/`SOL-006` (`browser.*`) is partial — agent + backend-go layers
> are done and tested end-to-end, but frontend dispatch to the new relay
> path is a separate, still-open design decision (see `TASK-036`'s "Status
> by layer" section). Each `BUG-XXX`/`SOL-XXX` file's own `**Status:**` line
> is the authoritative per-item status — the tables below are this report's
> original point-in-time snapshot and were NOT re-audited row-by-row for
> exact "X/Y methods missing" counts; treat each linked report as ground
> truth over these summary numbers, same caveat this file already carried.

## The headline number

`backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
wires only **8 of the 262 real, frontend-called RPC methods** end-to-end
(`annotation.create`/`list`, `git.status`/`diff`, `automation.runNow`,
`preflight.check`, `devServer.list`, `fleet.health.checkAll`). Two more
channels are registered under the `task.*` prefix (`task.create`/`task.get`)
but **don't match any method the frontend actually calls** — see BUG-034.
Every other channel name — roughly **250 methods across 31 namespaces** —
falls through to `registry.go`'s `notImplementedHandler`.

Separately, none of `http-endpoints.md`'s `/admin/api/*` (11 routes),
`/auth/sso/:provider`, or `/api/push-*` (3 routes) exist at their documented
paths (BUG-001–003).

## Methodology

Each report was written by first reading the relevant frontend spec
(`rpc-catalog.md` for the method list + call sites), then verifying against
the actual current `backend-go` source — `grep`-confirming the channel
really isn't registered in `wscompat/channels.go`, then checking the likely
owning service's `.proto` and `internal/usecase/` for a matching gRPC method
before writing. This distinguishes two different kinds of gap, called out
per-namespace below:

- **Channel-wiring-only gap** — the service already has the gRPC method
  (often already REST-wired at `/v1/*`); it just needs a `wscompat` handler.
  Low effort.
- **Capability gap** — no service has this RPC at all yet; needs real
  proto + usecase + adapter work before a channel can do anything useful.

Every report cites real `file:line`, matching the discipline
`../api-v1/BUG-002-missing-channel-registrations.md` set as precedent.

## HTTP endpoint gaps (`http-endpoints.md`)

| ID | Title | Severity | Status |
|----|-------|----------|--------|
| [BUG-001](./BUG-001-admin-console-rest-surface-missing.md) | `/admin/api/*` admin console REST surface does not exist | High | ✅ Resolved |
| [BUG-002](./BUG-002-auth-sso-route-missing.md) | `GET /auth/sso/:provider` not registered (404 instead of 501 stub) | Low | ✅ Resolved |
| [BUG-003](./BUG-003-web-push-endpoints-path-and-auth-mismatch.md) | `/api/push-*` at wrong path, wrong auth gate, missing `unsubscribe` | Medium | ✅ Resolved |

## RPC channel gaps (`rpc-catalog.md`) — fully unimplemented namespaces

| ID | Namespace | Methods missing | Severity | Owning service (per report) |
|----|-----------|:---:|----------|------------------------------|
| [BUG-004](./BUG-004-accounts-channels-not-implemented.md) | `accounts.*` | 4/4 | Medium | **none found** |
| [BUG-005](./BUG-005-aiprovider-channels-not-implemented.md) | `aiProvider.*` | 6/6 | High | `ai-provider-service` (proto too thin) |
| [BUG-006](./BUG-006-browser-channels-not-implemented.md) | `browser.*` | 15/15 | Medium | **none found** |
| [BUG-007](./BUG-007-credentials-channels-not-implemented.md) | `credentials.*` | 4/4 | Medium | `credential-broker-service` (not gateway-reachable; shape mismatch) |
| [BUG-008](./BUG-008-emulator-channels-not-implemented.md) | `emulator.*` | 8/8 | Medium | **none found** |
| [BUG-009](./BUG-009-files-channels-not-implemented.md) | `files.*` | 18/18 | High | **none** (git-gateway-service closest, no file I/O RPCs) |
| [BUG-010](./BUG-010-folderworkspace-channels-not-implemented.md) | `folderWorkspace.*` | 5/5 | Medium | `project-service` (no matching RPC) |
| [BUG-011](./BUG-011-host-channels-not-implemented.md) | `host.*` | 4/4 | Low | **none found** |
| [BUG-012](./BUG-012-github-channels-not-implemented.md) | `github.*` | 24/24 | High | `scm-integration-service` (only `rateLimit` backed) |
| [BUG-013](./BUG-013-gitlab-channels-not-implemented.md) | `gitlab.*` | 4/4 | Medium | `scm-integration-service` (only `rateLimit` backed) |
| [BUG-014](./BUG-014-hostedreview-channels-not-implemented.md) | `hostedReview.*` | 3/3 | Medium | `scm-integration-service` (2/3 backed, wiring-only) |
| [BUG-015](./BUG-015-jira-channels-not-implemented.md) | `jira.*` | 19/19 | Medium-High | `issue-tracking-service` (proto far too thin) |
| [BUG-016](./BUG-016-linear-channels-not-implemented.md) | `linear.*` | 19/19 | Medium-High | `issue-tracking-service` (proto far too thin) |
| [BUG-017](./BUG-017-nativechat-channels-not-implemented.md) | `nativeChat.*` | 1/1 | Low | **none found** |
| [BUG-018](./BUG-018-orchestration-channels-not-implemented.md) | `orchestration.*` | 1/1 | Low-Medium | `orchestration-service` (no read/"show" RPC) |
| [BUG-019](./BUG-019-profile-channels-not-implemented.md) | `profile.*` | 5/6 | High | `tenant-service` (`getResolved` backed, wiring-only) |
| [BUG-020](./BUG-020-project-channels-not-implemented.md) | `project.*` | 3/7 | High | `project-service` (4/7 backed, wiring-only) |
| [BUG-021](./BUG-021-projectgroup-channels-not-implemented.md) | `projectGroup.*` | 3/7 | Medium | `project-service` (4/7 backed, wiring-only) |
| [BUG-022](./BUG-022-projecthostsetup-channels-not-implemented.md) | `projectHostSetup.*` | 5/5 | Medium | **none found** |
| [BUG-023](./BUG-023-repo-channels-not-implemented.md) | `repo.*` | 13/13 | High | split — `project-service` (4, unwired) + **none** (8) |
| [BUG-024](./BUG-024-ssh-channels-not-implemented.md) | `ssh.*` | 4/4 | High | `infra-fleet-service` (only write-only `CreateSshTarget` exists) |
| [BUG-025](./BUG-025-status-channels-not-implemented.md) | `status.*` | 1/1 | Low-Medium | none (no drop-in substitute) |
| [BUG-026](./BUG-026-workspace-channels-not-implemented.md) | `workspace.*` | 1/1 | Medium | **none found** |
| [BUG-027](./BUG-027-workspaceports-channels-not-implemented.md) | `workspacePorts.*` | 1/2 | Medium | `infra-fleet-service` (`scan` backed, wiring-only; `kill` has nothing) |
| [BUG-028](./BUG-028-team-channels-not-implemented.md) | `team.*` | 2/5 | Medium | `tenant-service` (3/5 backed, wiring-only) |
| [BUG-029](./BUG-029-terminal-channels-not-implemented.md) | `terminal.*` | 10/10 | High | **none** (`infra-fleet-service` has only generic `Relay`) |
| [BUG-030](./BUG-030-workflow-channels-not-implemented.md) | `workflow.*` | 1/4 | High | `workflow-service` (3/4 backed, wiring-only) |
| [BUG-031](./BUG-031-worktree-channels-not-implemented.md) | `worktree.*` | 6/8 | High | split — `project-service` (2, unwired) + **none** (`git-gateway-service` has zero worktree RPCs) |

## RPC channel gaps — partially implemented namespaces

| ID | Namespace | Methods missing | Already wired | Severity |
|----|-----------|:---:|---|----------|
| [BUG-032](./BUG-032-git-channels-partially-implemented.md) | `git.*` | 32/34 (28 with no RPC at all; `commit`/`push`/`pull`/`generateCommitMessage` are wiring-only) | `status`, `diff` | High |
| [BUG-033](./BUG-033-automation-channels-partially-implemented.md) | `automation.*` | 5/6 (`create`/`runs` wiring-only; `delete`/`list`/`update` unbuilt to the repository layer) | `runNow` | Medium |
| [BUG-034](./BUG-034-task-channels-not-implemented.md) | `task.*` | 7/7 — **0 of the frontend's real calls are reachable** despite 2 channels registered under this prefix | `create`/`get` (dead code — not called by the frontend, see below) | High |

## Transport-level gaps (not visible to `rpc-catalog.md`'s methodology)

| ID | Title | Severity | Status |
|----|-------|----------|--------|
| [BUG-035](./BUG-035-ws-server-push-not-implemented.md) | Server→client `push` frames never sent — `wscompat` is request/response only | High | ✅ Resolved |
| [BUG-036](./BUG-036-git-relay-methods-unreachable-on-agent.md) | `git.*` relay calls target agent methods that don't exist on the transport backend-go uses — breaks even the 2 channels marked "wired" | **Critical** | ✅ Resolved |

`BUG-036` was found by tracing the full frontend→backend→agent→backend→frontend
data flow for `git.status`/`git.diff` end to end against the real `agent/`
source (not just backend-go), per an explicit request to verify that flow
against `docs/`'s design (`ADR-012`, `agent-rpc-catalog-git-fs.md`,
`gaps-and-findings.md`). It's the only finding in this directory that
means something already marked "real" is actually broken, not just
unimplemented — read it before starting any `git.*` relay work.

`rpc-catalog.md` only scans `callRuntimeRpc(...)` client-invoke call sites,
so it structurally can't see push-only features. Confirmed directly against
`handler.go`: there's no `push` case in the read loop, and no connection
registry anything else could write to — `notifications.subscribe`,
`runtime.clientEvents.subscribe`, and any "live update without polling" UI
depend on this and currently degrade silently (no error, update just never
arrives) rather than failing loudly like every other gap in this directory.

## Notable cross-cutting findings

- **`task.create`/`task.get` are dead code.** `wscompat` registers these two
  channels against real `task-service` RPCs, but neither appears anywhere in
  `rpc-catalog.md`'s actual `task.*` call list (`aiApply`, `aiDecompose`,
  `delete`, `execute`, `getDependencies`, `list`, `update`). From the
  frontend's point of view, `task.*` is a **0-of-7 gap dressed up as "2
  channels wired."** See BUG-034.
- **`worktree.*` and `repo.*` split across two services, and
  `git-gateway-service` has zero worktree RPCs** — nothing in backend-go can
  currently create or remove an on-disk worktree. See BUG-023, BUG-031.
- **No owning service exists at all** for `accounts.*`, `browser.*`,
  `emulator.*`, `host.*`, `nativeChat.*`, `projectHostSetup.*`, most of
  `files.*`, `terminal.*`, and 8 of `repo.*`'s 13 methods. These need new
  proto/usecase work, not just a `wscompat` wrapper — several reports flag
  that the old TypeScript backend's design (driving emulators/browsers/PTYs
  or reading transcript files **on the backend host itself**) may not be the
  right target architecture for backend-go's multi-tenant deployment either,
  and should relay to the Dev Server Agent instead of being ported as-is.
- **Quick wins — RPC already exists, only the `wscompat` handler is
  missing:** `project.{create,get,list,update}`, `projectGroup.{create,update,delete,list}`,
  `team.{create,addMember,listMembers}`, `workflow.{execute,cancel,template.create}`,
  `git.{commit,push,pull,generateCommitMessage}`, `automation.{create,runs}`,
  `profile.getResolved`, `workspacePorts.scan`, `github.rateLimit`,
  `gitlab.rateLimit`, `hostedReview.{create,forBranch}`, `aiProvider.create`.
  Wiring just these ~25 methods closes a meaningful chunk of the gap for
  comparatively little effort — see each namespace's report for exact
  `file:line` citations.
- **`automation.runNow`'s "wired" status may be less real than it looks.**
  `automation.proto`'s doc comment claims it now delegates to
  `workflow-service.ExecuteAdHocStep`, closing the old TS backend's
  "no working execution path" gap — but this is unverified at runtime; flagged
  in BUG-033 as worth an explicit check, not assumed.
- **`task.execute`'s backing `Execute` RPC exists, but its executors don't.**
  `SimpleExecutor`/`ComplexExecutor` are themselves stubs — wiring the
  channel alone won't make execution functional. See BUG-034.

## What this doesn't cover

- **Param/response shape correctness** — like `rpc-catalog.md` itself, this
  is a name/existence-level audit. A channel being "wired" doesn't mean its
  wire shape matches the frontend's expectations byte-for-byte (see
  `channels.go`'s own header comment on this exact caveat).
- **`ipc-surface.md`** — reviewed, introduces no backend gaps (see intro).
- Total counts above (~250 missing methods) are a manual tally across 34
  independently-researched reports and may be off by a couple — treat each
  report's own table as ground truth over this index's summary numbers.
