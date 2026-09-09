# TASK-TG-001-06: `task.create` wscompat channel — decode and forward `projectId` (real bug, blocks FE-TASK-001)

**From Solution:** none directly — discovered while implementing [FE-TASK-001](../../../../frontend/crs/v4/task-graph/tasks/FE-TASK-001-new-task-creation-dialog.md); this is a standalone bug in existing code, not a BE-SOL-001 design gap
**Priority:** P0 — silently breaks every task created through the UI once [FE-TASK-001](../../../../frontend/crs/v4/task-graph/tasks/FE-TASK-001-new-task-creation-dialog.md) ships, if landed first
**Service:** `api-gateway` (wscompat) + `task-service` (proto already has the field, unused)
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** None — independent of every BE-SOL-00N task, can land immediately
**Status:** `[ ]` TODO

---

## Context

`registerTaskChannels`'s `task.create` handler
(`channels.go:277-294`) decodes only `{title, parentId}` and never sets
`ProjectId` on the outgoing `CreateTaskRequest`, even though
`CreateTaskRequest` (`task.proto:61-66`) already has a `project_id = 4`
field:

```go
// channels.go:278-294 — current code
r.Register("task.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
    type createArgs struct {
        Title    string `json:"title"`
        ParentID string `json:"parentId"`
    }
    in, err := decodeArg[createArgs](args, 0)
    if err != nil { return nil, err }
    resp, err := client.CreateTask(ctx, &taskv1.CreateTaskRequest{
        TenantId: id.TenantID, Title: in.Title, ParentId: in.ParentID,
    })
    if err != nil { return nil, err }
    return resp.GetTask(), nil
})
```

If a frontend caller sends `projectId` in the JSON payload today, Go's
`encoding/json` silently drops it (unknown field to `createArgs`, no
error) — the created task always has an empty `ProjectId`, and
`useTasks(projectId)`'s client-side filter
(`allTasks.filter(t => t.projectId === projectId)`) makes it **disappear
from the UI immediately**, even after a successful create + refetch. This
is invisible today only because nothing in the frontend calls `task.create`
yet (confirmed: 0 call sites before [FE-TASK-001](../../../../frontend/crs/v4/task-graph/tasks/FE-TASK-001-new-task-creation-dialog.md)).

## Changes to make

```go
// channels.go — task.create handler
type createArgs struct {
    Title     string `json:"title"`
    ParentID  string `json:"parentId"`
    ProjectID string `json:"projectId"` // NEW
}
in, err := decodeArg[createArgs](args, 0)
if err != nil { return nil, err }
resp, err := client.CreateTask(ctx, &taskv1.CreateTaskRequest{
    TenantId: id.TenantID, Title: in.Title, ParentId: in.ParentID, ProjectId: in.ProjectID, // NEW
})
```

`creator_id` is a separate, larger gap — `CreateTaskRequest` has no such
field at all today (it's part of [BE-SOL-003](../solutions/BE-SOL-003-task-access-control-team-scope-and-sharing.md)'s
[TASK-TG-003-02](./TASK-TG-003-02-owner-grant-on-create-task.md), not this
task). Do not add `creator_id` decoding here — that belongs with the
`CreateTaskRequest.creator_id` proto field TASK-TG-003-02 adds.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/wscompat/... -run TestTaskCreate -v
```

Expected: a `task.create` call with `projectId` set produces a task whose
`ProjectId` matches — add a test case if one testing this channel doesn't
already assert on the field (confirm current test coverage before writing
a new one, don't duplicate an existing assertion).

## Coordinate with

[FE-TASK-001](../../../../frontend/crs/v4/task-graph/tasks/FE-TASK-001-new-task-creation-dialog.md) —
its "New Task" dialog is only correct end-to-end once this task lands too;
land this one first or in the same PR, not after.
