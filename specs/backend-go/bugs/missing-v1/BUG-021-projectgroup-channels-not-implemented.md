# BUG-021: `projectGroup.*` channels not implemented in backend-go

**Service:** `project-service` (exists, `RouteWired` at `/v1/projects`; project-group surface REST-wired at `/v1/project-groups`)
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (absent), `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` (falls through to `notImplementedHandler`)
**Severity:** Medium
**Symptom:** Every `projectGroup.*` call from `repos.ts` (sidebar grouping/folder tree) times out with `channel "projectGroup.X" is not yet implemented in backend-go`
**Status:** ✅ Resolved — see TASK-136–140 (5 task(s), all DONE) for implementation evidence.

---

## Description

None of the 7 `projectGroup.*` methods the frontend calls are registered in
`wscompat.Registry`. Confirmed via:

```
$ grep -n '"projectGroup\.' backend-go/services/api-gateway/internal/adapter/wscompat/channels.go
(no matches)
```

`project-service` is confirmed as the owning service — its proto file
already models `ProjectGroup` as "self-referential tree via
parent_group_id" (`backend-go/proto/orca/project/v1/project.proto:40-41`),
and 4 of the 7 methods have a directly matching RPC, already REST-wired at
`/v1/project-groups` (`project_routes.go:44-49`):

- `projectGroup.create` → `ProjectService.CreateProjectGroup`
  (`project.proto:42`), usecase
  `backend-go/services/project-service/internal/usecase/create_project_group.go`,
  REST-wired via `handleCreateProjectGroup` (`project_routes.go:445-465`).
- `projectGroup.update` → `ProjectService.UpdateProjectGroup`
  (`project.proto:43`), usecase `update_project_group.go`, REST-wired via
  `handleUpdateProjectGroup` (`project_routes.go:471-491`).
- `projectGroup.delete` → `ProjectService.DeleteProjectGroup`
  (`project.proto:44`), usecase `delete_project_group.go`, REST-wired via
  `handleDeleteProjectGroup` (`project_routes.go:493-505`).
- `projectGroup.list` → `ProjectService.ListProjectGroups`
  (`project.proto:45`), usecase `list_project_groups.go`, REST-wired via
  `handleListProjectGroups` (`project_routes.go:507-519`).

These 4 are the low-effort case — same pattern as BUG-020's 4
already-working `project.*` methods: `wscompat` just needs thin wrapper
registrations calling the same `projectv1.ProjectServiceClient`.

The other 3 have **no backing RPC or usecase at all**.
`ProjectService`'s full RPC list (`project.proto:12-45`) has no
`MoveProject`, `ScanNested`, or `ImportNested` method, and
`backend-go/services/project-service/internal/usecase/` has no
corresponding usecase files:

- `projectGroup.moveProject` — no RPC/usecase.
- `projectGroup.scanNested` — no RPC/usecase.
- `projectGroup.importNested` — no RPC/usecase.

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `projectGroup.create` | `frontend/src/renderer/src/store/slices/repos.ts:2106` | Backing RPC exists (`CreateProjectGroup`) and is already REST-wired — just needs a `wscompat` registration |
| `projectGroup.delete` | `frontend/src/renderer/src/store/slices/repos.ts:2254` | Backing RPC exists (`DeleteProjectGroup`) and is already REST-wired — just needs a `wscompat` registration |
| `projectGroup.importNested` | `frontend/src/renderer/src/store/slices/repos.ts:2067` | No RPC/usecase anywhere in `project-service`; see local-FS-scan note below |
| `projectGroup.list` | `frontend/src/renderer/src/store/slices/repos.ts:1109` | Backing RPC exists (`ListProjectGroups`) and is already REST-wired — just needs a `wscompat` registration |
| `projectGroup.moveProject` | `frontend/src/renderer/src/store/slices/repos.ts:2365` | No RPC/usecase anywhere in `project-service` |
| `projectGroup.scanNested` | `frontend/src/renderer/src/store/slices/repos.ts:2033` | No RPC/usecase anywhere in `project-service`; see local-FS-scan note below |
| `projectGroup.update` | `frontend/src/renderer/src/store/slices/repos.ts:2224` | Backing RPC exists (`UpdateProjectGroup`) and is already REST-wired — just needs a `wscompat` registration |

---

## Dispatch model

Per `specs/frontend/api/backend-agent-execution-boundary.md:141`: 🟢
Postgres, local-FS assist — CRUD via the one-JSON-blob store
(`store.createProjectGroup` etc. in the old TS backend).
`scanNested`/`importNested` did a local filesystem directory scan **on the
backend host** first, then persisted results — no relay.

**Needs re-examination for backend-go**: scanning the *backend host's*
filesystem for nested project folders is a legacy-desktop-app assumption
(finding folders on the machine running the backend process). In a server
deployment this assumption likely no longer holds — `scanNested`/
`importNested` should probably scan the *requesting user's dev server*
instead, via the Dev Server Agent relay (the same relay pattern
`infra-fleet-service`'s `Relay` RPC already provides,
`backend-go/proto/orca/infrafleet/v1/infrafleet.proto:31`), rather than the
`api-gateway`/`project-service` host's own disk. Whoever implements these 2
methods should decide this architecture question explicitly rather than
reproducing the old backend-host-local-scan behavior by default.

---

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` — confirmed no `projectGroup.*` registrations
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` — `notImplementedHandler`
- `backend-go/services/api-gateway/internal/domain/registry.go:88` — `/v1/projects` → `project-service`, `RouteWired`
- `backend-go/proto/orca/project/v1/project.proto:12-45,227-262` — `ProjectService`'s full RPC surface and `ProjectGroup` messages
- `backend-go/services/api-gateway/internal/adapter/httpgateway/project_routes.go:44-49,445-519` — `mountProjectRoutes`'s `/v1/project-groups` sub-route and the 4 already-working REST handlers
- `backend-go/services/project-service/internal/usecase/` — `create_project_group.go`, `update_project_group.go`, `delete_project_group.go`, `list_project_groups.go` exist; no move/scan-nested/import-nested usecase files
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:31` — `Relay` RPC, the existing Dev Server Agent relay primitive referenced for the re-examination note
- `specs/frontend/api/backend-agent-execution-boundary.md:141` — `projectGroup.*` 🟢/local-FS-assist dispatch classification
- `specs/frontend/api/rpc-catalog.md:380-390` — `projectGroup.*` catalog entries
- `specs/backend-go/bugs/api-v1/BUG-002-missing-channel-registrations.md` — sibling bug report this follows the format of
