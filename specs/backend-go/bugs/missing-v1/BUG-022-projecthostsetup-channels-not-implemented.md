# BUG-022: `projectHostSetup.*` channels not implemented in backend-go

**Service:** none — checked both `project-service` and `infra-fleet-service`; neither owns a "bind project to dev-server host" concept today
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (absent), `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` (falls through to `notImplementedHandler`)
**Severity:** Medium
**Symptom:** Every `projectHostSetup.*` call from `repos.ts` times out with `channel "projectHostSetup.X" is not yet implemented in backend-go`
**Status:** ❌ Open

---

## Description

None of the 5 `projectHostSetup.*` methods the frontend calls are registered
in `wscompat.Registry`. Confirmed via:

```
$ grep -n '"projectHostSetup\.' backend-go/services/api-gateway/internal/adapter/wscompat/channels.go
(no matches)
```

Per `specs/frontend/api/rpc-catalog.md:58`, this namespace is "Binding a
project to a dev-server host (legacy repo/host-setup model)" — a project↔host
binding concept, so both plausible owners were checked:

- **`project-service`** (`backend-go/proto/orca/project/v1/project.proto`)
  has a `Repo` surface (`AddRepo`/`ListRepos`/`ReorderRepos`/`RemoveRepo`,
  `project.proto:26-29`) and worktree metadata
  (`RecordWorktreeCreated`/`RecordWorktreeRemoved`/`ListWorktrees`/
  `SetWorktreeActivation`/`RenameWorktree`, `project.proto:31-38`), plus
  `RebindDevServer` for the *project-level* `dev_server_id` field
  (`project.proto:16,106-116`) — but nothing named `HostSetup`, and nothing
  that matches `setupExistingFolder`'s "point an existing on-disk folder at a
  host and create a project from it" semantics.
- **`infra-fleet-service`** (`backend-go/proto/orca/infrafleet/v1/infrafleet.proto`)
  owns `RegisterDevServer`, `CreateConnection`, `ResolveConnection`,
  `CreateSshTarget` (`infrafleet.proto:11-31`) — dev-server/connection
  lifecycle, but again nothing named `HostSetup` and nothing that models a
  per-project host-setup record distinct from a `Connection`.

A repo-wide search confirms neither `HostSetup` nor `SetupExistingFolder` (nor
`host_setup`) appears anywhere in `backend-go`:

```
$ grep -rln "HostSetup\|SetupExistingFolder\|host_setup" backend-go/
(no matches)
```

`registry.go`'s `NewDefaultServiceRegistry()`
(`backend-go/services/api-gateway/internal/domain/registry.go:84-99`) has no
routing rule for a host-setup-flavored path either — consistent with no
service owning this today. This is a full-namespace, service-doesn't-have-
this-capability gap, same shape as BUG-009's `files.*` finding — not just a
missing `wscompat` handler over an existing RPC.

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `projectHostSetup.create` | `frontend/src/renderer/src/store/slices/repos.ts:2569` | No RPC/usecase in `project-service` or `infra-fleet-service` |
| `projectHostSetup.delete` | `frontend/src/renderer/src/store/slices/repos.ts:2656` | No RPC/usecase in `project-service` or `infra-fleet-service` |
| `projectHostSetup.list` | `frontend/src/renderer/src/store/slices/repos.ts:443` | No RPC/usecase in `project-service` or `infra-fleet-service` |
| `projectHostSetup.setupExistingFolder` | `frontend/src/renderer/src/store/slices/repos.ts:2516` | No RPC/usecase in `project-service` or `infra-fleet-service` |
| `projectHostSetup.update` | `frontend/src/renderer/src/store/slices/repos.ts:2608` | No RPC/usecase in `project-service` or `infra-fleet-service` |

---

## Dispatch model

`specs/frontend/api/backend-agent-execution-boundary.md` has no row for
`projectHostSetup.*` — confirmed by grep, it is not covered as its own
namespace in that document. Stating this honestly rather than guessing a
classification for it.

**Inference, not a confirmed fact from the source doc**: this namespace is
closely related to `project.*` (🟢 Postgres relational, per
`backend-agent-execution-boundary.md:135`) and `devServer.*` (Postgres CRUD
for registration/listing, relay for connect/disconnect, per this repo's
existing `wscompat/channels.go:277-434` implementation of `devServer.list`/
`devServer.add` against `infra-fleet-service`). Given `projectHostSetup`
binds a project to a dev-server host, it likely follows a similar 🟢
Postgres-CRUD-with-host-reference pattern — but this is an inference from
adjacent namespaces, not a documented dispatch classification, and should be
verified against the legacy TS backend's actual implementation (or product
intent) before being treated as settled.

---

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` — confirmed no `projectHostSetup.*` registrations; `devServer.*`/`fleet.*` registrations at lines 277-434 for the adjacent-pattern comparison
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` — `notImplementedHandler`
- `backend-go/services/api-gateway/internal/domain/registry.go:84-99` — `NewDefaultServiceRegistry()`, no host-setup routing rule
- `backend-go/proto/orca/project/v1/project.proto:16,26-38,106-116` — `project-service`'s repo/worktree/rebind surface, checked and ruled out
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:11-31` — `infra-fleet-service`'s dev-server/connection surface, checked and ruled out
- `specs/frontend/api/backend-agent-execution-boundary.md` — no `projectHostSetup.*` row; `:135` (`project.*`) and `wscompat/channels.go:277-434` (`devServer.*`) used only as adjacent-pattern inference
- `specs/frontend/api/rpc-catalog.md:58,392-400` — `projectHostSetup.*` namespace description and catalog entries
- `specs/backend-go/bugs/api-v1/BUG-002-missing-channel-registrations.md` — sibling bug report this follows the format of
- `specs/backend-go/bugs/missing-v1/BUG-009-files-channels-not-implemented.md` — sibling report with the same "service doesn't have this capability yet" shape
