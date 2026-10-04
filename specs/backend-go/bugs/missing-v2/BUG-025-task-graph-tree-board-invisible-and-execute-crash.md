# BUG-025: 3 distinct bugs found live-testing BL-TG-05 — Tree/Board hid every task, "Run with Agent" crashed with no worktree selected

**Service:** `api-gateway` (bug 1), `frontend` (bugs 2, 3)
**Severity:** 🔴 Critical — Tree/Board (the two default Task Graph views) showed 0 tasks even with real tasks in the database; "Run with Agent"/"Generate Spec" threw before ever reaching the network
**Status:** ✅ All 3 confirmed via source read + live DB query + the user's own error traces (not guessed). ✅ Fixed + unit-tested. Not yet deployed — see "Deploy plan" (per explicit user request: write this doc and verify carefully before shipping another guess).

## Context — why this needed a careful pass, not another quick patch

Earlier in this session, BUG-023 fixed `task.*` channels returning raw snake_case proto, and a follow-up fixed a root task's `parentId` marshaling as `""` instead of absent. Both were deployed. The user then reported the Tree/Board views **still** showed nothing, plus a new crash — correctly pushing back that this needed a real investigation, not another guess. This doc is that investigation, with every claim traceable to a specific file/line, a live DB query, or the user's own pasted stack trace.

## Bug 1 — a root task's `parentId` needed `null`, not omitted, not `""`

**Confirmed via source read of `frontend/src/renderer/src/components/task/TaskTreeView.tsx:11-24`:**
```ts
function renderLevel(tasks: OrcaTask[], parentId: string | null, ...) {
  return tasks.filter((t) => t.parentId === parentId) // top call: renderLevel(tasks, null, 0, ...)
}
```
This is a **strict-equality** comparison against the literal `null`, not a truthiness/undefined check.

**The 2-stage mistake, both confirmed by re-deriving what `encoding/json` actually produces:**
1. Original `taskView.ParentID string \`json:"parentId"\`` (no omitempty) → a root task (backend `ParentId == ""`) marshals to `"parentId": ""`. `"" !== null` → filtered out of every Tree level. **Never appears anywhere in Tree.**
2. First fix attempt added `omitempty` → for a root task, the key is dropped entirely → decodes in JS as `task.parentId === undefined`. **`undefined !== null` too** — same symptom, still hidden. This was deployed and did NOT fix the reported issue (confirmed: the user re-tested after this deploy and still saw nothing in Tree).

**Real fix:** `ParentID` must be a `*string` (pointer), left `nil` for a root task (no `omitempty` on this one field) — Go's `encoding/json` marshals a nil pointer field without `omitempty` as `"parentId": null`, an explicit JSON `null`, which is what the strict equality check actually needs. A non-root task's `ParentID` still marshals as a normal string (`"parentId": "abc-123"`), matching a child-lookup call (`renderLevel(tasks, task.id, ...)`).

**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`'s `taskView`/`toTaskView`.

## Bug 2 — `TaskStatus` never included `'open'`, the real backend default

**Confirmed via live DB query** (both tasks the user created during testing):
```
title      | status
-----------+-------
test task  | open
test 2     | open
```
`task-service`'s `domain.Status` (`backend-go/services/task-service/internal/domain/task.go:27-38`) has `StatusOpen = "open"` as its own documented first-class default value — every task created without an explicit status starts here.

But the **frontend never learned this value exists**, in 4 separate places:
1. `frontend/src/shared/task-types.ts`'s `TaskStatus` union type — 7 values, no `'open'`.
2. `TaskBoardView.tsx`'s `STATUS_ORDER` (the literal list of Board columns) — `columnTasks = tasks.filter(t => effectiveStatus(t) === status)` for each of 7 known statuses. **An `'open'` task matches NONE of them — it renders in zero columns, i.e., invisible on the Board**, independent of Bug 1 (this alone would hide it from Board even with Bug 1 fixed).
3. `TaskStatusBadge.tsx`'s `STATUS_CONFIG` — has a safe `|| STATUS_CONFIG.todo` fallback, so this one only mis-labels the badge ("Todo" instead of "Open"), doesn't hide anything — the least severe of the 4.
4. `TaskDetail.tsx`'s `TASK_STATUSES` (the status-change `<select>` in Task Detail) — an open task's dropdown wouldn't show its real current value as one of its own options.

**Real fix:** add `'open'` to all 4 (type, `STATUS_ORDER`, `STATUS_CONFIG`, `TASK_STATUSES`) plus `TASK_STATUS_PROGRESS` (a `Record<TaskStatus, number>` TypeScript then correctly flagged as missing the new key at compile time — caught by `tsc`, not guessed).

**Not touched:** `TaskGraph.tsx`'s status-filter `<select>` (`all/todo/in_progress/done` only) is ALSO missing several statuses (`backlog`/`review`/`blocked`/`cancelled`/`open`) — a pre-existing, separate gap that doesn't hide anything under the default "All Status" filter. Left alone — out of this bug's scope, flagged for awareness only.

## Bug 3 — "Run with Agent" crashed with `TypeError: Cannot read properties of null (reading 'path')`

**Confirmed via the user's own pasted stack trace** (`TaskDetail-BGhcKXzu.js`) and source read of both call sites:
- `TaskDetail.tsx:104` (pre-fix): `worktreePath: currentWorktree!.path`
- `TaskPromptEditor.tsx:62` (pre-fix): `worktreePath: currentWorktree!.path`

`currentWorktree` comes from `useWorkspace()` — the **globally selected worktree in the sidebar**, a concept entirely independent of which worktree a given *task* owns. The Tasks tab has no worktree-selection requirement of its own, so `currentWorktree` is legitimately `null` while working with Tasks. The `!` non-null assertion is a compile-time-only annotation — it does nothing at runtime, so `null.path` threw exactly the reported `TypeError`.

**Confirmed dead parameter, not just a null-safety gap:** `task.execute`'s real wscompat handler (`channels_automation_task.go`'s `executeArgs{TaskID, RequestID, Prompt}`) **never decodes a `worktreePath` field at all** — `SimpleExecutor.Execute` (`simple_executor.go`) resolves the task's own worktree path itself, server-side, via `ProjectExecutionResolver.ResolveConnection(ctx, tenantID, task.ProjectID)`. The client-computed `worktreePath` was always silently discarded even on the rare occasions `currentWorktree` happened to be non-null — it did nothing useful and only added a crash risk.

**Real fix:** delete `worktreePath`/`currentWorktree` from both call sites entirely — not a null-guard, an outright removal of dead, crash-prone code.

## Testing

- `TestTaskGetChannel_RootTaskParentIDIsExplicitNull` / `TestTaskGetChannel_ChildTaskParentIDIsAString` (backend) — assert the wire has `"parentId": null` for a root task and a plain string for a child.
- `task-spec-build-loop.test.ts`, `TaskPromptEditor.test.tsx`, `TaskDetail.test.tsx` (frontend) — updated to assert `worktreePath` is never sent, plus new tests rendering both components with `currentWorktree: null` proving no crash.
- `tsc --noEmit` — clean (the `TASK_STATUS_PROGRESS` gap was caught this way, not missed).
- Manual, still needed after deploy: reload `b15.openledger.vn`, open Tasks tab, confirm the 2 existing tasks (`test task`, `test 2`) now appear in both Tree and Board, click one, click "Run with Agent" with no worktree selected in the sidebar and confirm no crash (the dev-server-connection error is a separate, already-explained project-configuration gap, not this bug).

## Deploy plan

Backend (`channels.go` + its test file) and frontend (`task-types.ts`, `TaskBoardView.tsx`, `TaskStatusBadge.tsx`, `TaskDetail.tsx`, `TaskPromptEditor.tsx` + their test files) ship together in one `build-local.sh` + `sync-to-server.sh` round, verified by `go build/vet/test` and `tsc --noEmit` + `vitest run` **before** the deploy command runs, per this session's own standing discipline — not skipped this time.

## Related

- [BUG-023](./BUG-023-task-channels-return-raw-snake-case-proto.md) — the raw-proto/`omitempty` fixes this bug's "Bug 1" is a direct continuation of (2 iterations on the same field, both now recorded here so the full story is in one place).
