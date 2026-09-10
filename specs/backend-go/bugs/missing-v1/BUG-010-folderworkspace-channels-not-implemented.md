# BUG-010: `folderWorkspace.*` channels not implemented in backend-go

**Service:** `project-service` (closest candidate by domain — owns `/v1/projects`, has an analogous "folder-style" grouping concept for `ProjectGroup`, but no `FolderWorkspace` RPCs)
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (absent), `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` (falls through to `notImplementedHandler`)
**Severity:** Medium — used by the repos store for folder-workspace CRUD and path-status checks (adding/managing non-git folder workspaces), a real but secondary path relative to core git-repo workflows.
**Symptom:** Every `folderWorkspace.*` call from `repos.ts` times out with `channel "folderWorkspace.X" is not yet implemented in backend-go — see backend-go/docs/execution-plan.md's frontend-compatibility-layer coverage table`.
**Status:** ✅ Resolved — see TASK-061–067 (7 task(s), all DONE) for implementation evidence.

---

## Description

None of the 5 `folderWorkspace.*` methods the frontend calls are registered
in `wscompat.Registry`. Confirmed via:

```
$ grep -n '"folderWorkspace\.' backend-go/services/api-gateway/internal/adapter/wscompat/channels.go
(no matches)
```

`project-service` is the natural owning candidate — it already owns
`/v1/projects` (`backend-go/services/api-gateway/internal/domain/registry.go:88`,
`RouteWired`) and has a conceptually similar "folder-tree" grouping in
`ProjectGroup` (see `backend-go/services/project-service/internal/usecase/ports.go:131`,
"`ProjectGroupRepository` is the persistence port for the folder-style
[...]", and `create_project_group.go`/`delete_project_group.go`/
`update_project_group.go`/`list_project_groups.go`). However, `ProjectGroup`
is a distinct concept from `folderWorkspace` (project-organizing folders vs.
non-git folder workspaces added to the workspace) — its proto has no
`FolderWorkspace` message or RPC:

```
$ grep -n 'rpc ' backend-go/proto/orca/project/v1/project.proto
12:  rpc CreateProject...
...
42:  rpc CreateProjectGroup(CreateProjectGroupRequest) returns (CreateProjectGroupResponse);
43:  rpc UpdateProjectGroup(UpdateProjectGroupRequest) returns (UpdateProjectGroupResponse);
44:  rpc DeleteProjectGroup(DeleteProjectGroupRequest) returns (DeleteProjectGroupResponse);
45:  rpc ListProjectGroups(ListProjectGroupsRequest) returns (ListProjectGroupsResponse);
```

No `Create/Update/Delete/List/GetPathStatus`-shaped RPC for folder
workspaces exists anywhere in `project-service` or any other backend-go
service/proto package. This is a **service-doesn't-have-this-capability
gap** — `project-service` would need 5 new RPCs (or a repurposing of
`ProjectGroup`, if product intent is for them to merge) before wscompat can
wire this namespace, not just a missing handler over an existing method.

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `folderWorkspace.create` | `frontend/src/renderer/src/store/slices/repos.ts:2134` | |
| `folderWorkspace.delete` | `frontend/src/renderer/src/store/slices/repos.ts:2190` | |
| `folderWorkspace.getPathStatus` | `frontend/src/renderer/src/store/slices/repos.ts:1293,1984` | Called twice — initial add-folder validation and again during host load |
| `folderWorkspace.list` | `frontend/src/renderer/src/store/slices/repos.ts:1153` | |
| `folderWorkspace.update` | `frontend/src/renderer/src/store/slices/repos.ts:2160` | |

None of these are registered anywhere in `channels.go`, confirmed by the grep
above — this is a full-namespace gap.

---

## Dispatch model

Per `specs/frontend/api/backend-agent-execution-boundary.md:141`, the old
TypeScript backend ran this 🟢 **Postgres**-only, with local-filesystem
assist for a sibling namespace: plain CRUD via the one-JSON-blob store
(`store.createFolderWorkspace` and its update/delete/list/getPathStatus
counterparts). There is no relay to the Dev Server Agent for
`folderWorkspace.*` itself — it's pure database CRUD. (The doc notes
`projectGroup.scanNested`/`importNested` do a local filesystem directory
scan on the backend host before persisting — but those are `projectGroup.*`
methods, a different namespace from this one, not part of this report.)

This means backend-go's implementation should be straightforward relational
CRUD in `project-service` (or a new sibling table), without needing to
reason about relay/dispatch-target logic the way `git.*`/`files.*` do.

---

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` — confirmed no `folderWorkspace.*` registrations
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` — `notImplementedHandler`
- `backend-go/services/api-gateway/internal/domain/registry.go:88` — `/v1/projects` → `project-service`, `RouteWired`
- `backend-go/proto/orca/project/v1/project.proto:12-45` — full RPC list, no folder-workspace RPCs
- `backend-go/services/project-service/internal/usecase/ports.go:131` — `ProjectGroupRepository` (closest, but distinct, concept)
- `specs/frontend/api/backend-agent-execution-boundary.md:141` — `folderWorkspace.*` 🟢 dispatch classification
- `specs/frontend/api/rpc-catalog.md:186-194` — `folderWorkspace.*` catalog entries
- `specs/backend-go/bugs/api-v1/BUG-002-missing-channel-registrations.md` — sibling bug report this follows the format of
