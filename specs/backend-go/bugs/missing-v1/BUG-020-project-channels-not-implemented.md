# BUG-020: `project.*` channels not implemented in backend-go

**Service:** `project-service` (exists, `RouteWired` at `/v1/projects`)
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (absent), `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` (falls through to `notImplementedHandler`)
**Severity:** High — project creation/membership is core navigation; `WorkspaceContext.tsx` calls `project.get` on load
**Symptom:** Every `project.*` call from `CreateProjectDialog.tsx`, `WorkspaceContext.tsx`, `MemberManager.tsx`, `ProjectSwitcher.tsx`, `repos.ts` times out with `channel "project.X" is not yet implemented in backend-go`
**Status:** ❌ Open

---

## Description

None of the 7 `project.*` methods the frontend calls are registered in
`wscompat.Registry`. Confirmed via:

```
$ grep -n '"project\.' backend-go/services/api-gateway/internal/adapter/wscompat/channels.go
(no matches)
```

Unlike most other missing namespaces, `project-service` genuinely
**already exists** and is fully wired end-to-end at the REST layer: it is
`RouteWired` at `/v1/projects`
(`backend-go/services/api-gateway/internal/domain/registry.go:88`), with a
real hand-written REST→gRPC proxy in
`backend-go/services/api-gateway/internal/adapter/httpgateway/project_routes.go`
(`mountProjectRoutes`, lines 21-50) that covers the project CRUD, member,
repo, worktree, and project-group surfaces.

4 of the 7 methods have a directly matching RPC, already REST-wired:

- `project.create` → `ProjectService.CreateProject` (`project.proto:12`),
  usecase `backend-go/services/project-service/internal/usecase/create_project.go`,
  REST-wired at `POST /v1/projects` via `handleCreateProject`
  (`project_routes.go:75-98`).
- `project.get` → `ProjectService.GetProject` (`project.proto:13`), usecase
  `get_project.go`, REST-wired at `GET /v1/projects/{id}` via
  `handleGetProject` (`project_routes.go:100-112`).
- `project.list` → `ProjectService.ListProjects` (`project.proto:14`),
  usecase `list_projects.go`, REST-wired at `GET /v1/projects` via
  `handleListProjects` (`project_routes.go:114-136`).
- `project.update` → `ProjectService.UpdateProject` (`project.proto:22`),
  usecase `update_project.go`, REST-wired at `PUT /v1/projects/{id}` via
  `handleUpdateProject` (`project_routes.go:145-168`).

These 4 are the low-effort case: the service, usecase, and REST proxy all
already work — `wscompat` just needs thin wrapper registrations that call
the same `projectv1.ProjectServiceClient` the REST handlers already use.

The other 3 have **no backing RPC at all**. `ProjectService`'s full RPC list
(`project.proto:12-45`) has exactly one member-related RPC, `AddMember`
(`project.proto:15`) — there is no `ListMembers`/`GetMembers`,
`RemoveMember`, or `UpdateMemberRole`. Confirmed at the repository-port
level too: `backend-go/services/project-service/internal/usecase/ports.go`
only declares `AddMember` (line 22) and `GetMembership` (lines 41-44, a
single caller's-own-membership lookup for authorization, not a list) on
`ProjectRepository`/`MembershipRepository` — no member-list, remove, or
role-update methods exist anywhere in the usecase layer:

- `project.getMembers` — no `ListMembers`/`GetMembers` RPC or repository
  method exists.
- `project.removeMember` — no `RemoveMember` RPC or repository method
  exists.
- `project.updateMemberRole` — no `UpdateMemberRole` RPC or repository
  method exists.

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `project.create` | `frontend/src/renderer/src/components/project/CreateProjectDialog.tsx:85` | Backing RPC exists (`CreateProject`) and is already REST-wired — just needs a `wscompat` registration |
| `project.get` | `frontend/src/renderer/src/context/WorkspaceContext.tsx:127` | Backing RPC exists (`GetProject`) and is already REST-wired — just needs a `wscompat` registration |
| `project.getMembers` | `frontend/src/renderer/src/components/project/MemberManager.tsx:44` | No `ListMembers`/`GetMembers` RPC or repository method anywhere in `project-service` |
| `project.list` | `frontend/src/renderer/src/components/project/ProjectSwitcher.tsx:32` | Backing RPC exists (`ListProjects`) and is already REST-wired — just needs a `wscompat` registration |
| `project.removeMember` | `frontend/src/renderer/src/components/project/MemberManager.tsx:66` | No `RemoveMember` RPC or repository method anywhere in `project-service` |
| `project.update` | `frontend/src/renderer/src/store/slices/repos.ts:3074` | Backing RPC exists (`UpdateProject`) and is already REST-wired — just needs a `wscompat` registration |
| `project.updateMemberRole` | `frontend/src/renderer/src/components/project/MemberManager.tsx:59` | No `UpdateMemberRole` RPC or repository method anywhere in `project-service` |

4 of 7 methods (`create`/`get`/`list`/`update`) are lower-effort: the
service-side work is entirely done, only the `wscompat` wrapper is missing.
The 3 member-management methods (`getMembers`/`removeMember`/
`updateMemberRole`) need new RPCs, proto messages, and usecase/repository
methods added to `project-service` before `wscompat` has anything to call.

---

## Dispatch model

Per `specs/frontend/api/backend-agent-execution-boundary.md:135`: 🟢
Postgres relational — `orca_v5_projects`/`orca_v5_project_members`. For
context only (not in this frontend's call list, so not reported as
missing here): `project.agentSpawn` is the one exception in this namespace
— despite living in `project.*`, it delegates entirely to
`ProfileAwareAgentSpawner.spawn()` → Dev Server Agent relay
(`relay.call('agent.exec', ...)`); no project row is touched by the spawn
itself.

---

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` — confirmed no `project.*` registrations
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` — `notImplementedHandler`
- `backend-go/services/api-gateway/internal/domain/registry.go:88` — `/v1/projects` → `project-service`, `RouteWired`
- `backend-go/proto/orca/project/v1/project.proto:11-46` — `ProjectService`'s full RPC surface
- `backend-go/services/api-gateway/internal/adapter/httpgateway/project_routes.go:21-50,75-136,145-168` — `mountProjectRoutes` and the 4 already-working REST handlers
- `backend-go/services/project-service/internal/usecase/ports.go:22,41-44` — `AddMember`/`GetMembership` are the only member-related repository methods
- `backend-go/services/project-service/internal/usecase/` — `create_project.go`, `get_project.go`, `list_projects.go`, `update_project.go` exist; no member-list/remove/role-update usecase files
- `specs/frontend/api/backend-agent-execution-boundary.md:135` — `project.*` 🟢 dispatch classification, `project.agentSpawn` exception noted for context
- `specs/frontend/api/rpc-catalog.md:368-378` — `project.*` catalog entries
- `specs/backend-go/bugs/api-v1/BUG-002-missing-channel-registrations.md` — sibling bug report this follows the format of
