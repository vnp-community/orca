# BUG-024: `useTask.ts`'s `updateTask` sends `{taskId, patch}` — `task.update`'s real handler decodes flat `{id, title, status, workflowTemplateId}`, never reads `patch` at all

**Service:** `frontend`
**File:** `src/renderer/src/hooks/useTask.ts`
**Severity:** 🔴 Critical — every `task.update` call from this hook has always been a no-op/error
**Status:** ✅ Root cause confirmed. ✅ Fixed (also adds `labels` support for [BL-TG-05](../../../../../docs/logic/task-graph/BL-TG-05-spec-approve-build-loop.md)), ✅ **deployed** (`2026.09.15-task-graph-labels-fix`).

## Root cause

`useTask.ts:9-14`:
```ts
const updateTask = async (patch: Partial<OrcaTask>) => {
  const target = getActiveRuntimeTarget(useAppStore.getState().settings)
  // task.update expects the patch nested under `patch`, not spread at the top level.
  await callRuntimeRpc(target, 'task.update', { taskId, patch })
  useAppStore.getState().updateTask(taskId, patch)
}
```
The comment's claim is wrong. The real handler (`channels_automation_task.go:362-386`) decodes:
```go
type updateArgs struct {
  ID                 string  `json:"id"`
  Title              *string `json:"title"`
  Status             *string `json:"status"`
  WorkflowTemplateID *string `json:"workflowTemplateId"`
}
```
— flat fields, key `id` (not `taskId`), no `patch` wrapper at all. `{taskId, patch}` decodes to `ID: ""` (the `id` key is absent) and every optional field `nil` — `client.UpdateTask` is called with an empty `Id`, which fails `TASK_NOT_FOUND` (or silently updates nothing if task-service ever tolerates an empty id, which it doesn't — `TaskRepository.Get`/`Update` key on `id`). Every caller of this hook's `updateTask` (status changes, title edits) has been broken since written; the local optimistic `useAppStore.getState().updateTask(taskId, patch)` call papers over it in the UI until the next full task reload.

## Fix

Rewrote `updateTask` to map `Partial<OrcaTask>` onto the real flat shape (`id`, `title`, `status`, `workflowTemplateId`, plus a new `labels` field — see below), sending only the keys actually present in `patch` (matching the handler's `*string`-pointer "omit = unchanged" convention — a key genuinely absent from the JSON, not just `undefined`, must not overwrite the field server-side).

**Also adds `labels` end-to-end** for BL-TG-05: `UpdateTaskRequest.labels` (proto, new field 15), `UpdateTaskInput.Labels *[]string` (usecase), `Server.UpdateTask` (grpc handler), `task.update`'s wscompat decode, and the postgres `UpdateTask` write. `nil` = unchanged (matches every other optional field's convention here); a present (possibly empty) array replaces the whole `Task.Labels` list — no partial add/remove semantics, since proto3 `repeated` has no way to distinguish "no key sent" from "empty array" without a wrapper message, and a whole-list-replace is simplest given the caller (frontend) always has the task's current labels loaded already.

## Testing

- `useTask.test.ts`: `updateTask({status: 'review'})` sends `{id, status}`, not `{taskId, patch}`; `updateTask({labels: [...]})` sends `{id, labels}`.
- `channels_automation_task_test.go`: `task.update` with a `labels` array threads through to `UpdateTaskRequest.Labels`.
- `update_task_test.go` (task-service): `UpdateTaskInput.Labels` replaces `Task.Labels`; `nil` leaves it unchanged.

## Related

- [BUG-023](./BUG-023-task-channels-return-raw-snake-case-proto.md) — the response-side sibling bug fixed in the same pass.
