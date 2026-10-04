# BUG-023: `task.*` wscompat channels return raw proto (snake_case) — same bug class as BUG-020/022, whole Task Graph feature area affected

**Service:** `api-gateway`
**Files:** `internal/adapter/wscompat/channels.go` (`registerTaskChannels`), `channels_automation_task.go` (`registerTaskCRUDChannels`)
**Severity:** 🔴 Critical — every multi-word `Task`/`Comment` field frontend reads is `undefined` today
**Status:** ✅ Root cause confirmed. ✅ Fixed for `task.create`/`task.get`/`task.update`/`task.addComment`/`task.listComments`/**`task.list`** (added `task.list` in a follow-up pass — live-reproduced: a freshly created task disappeared from the Tree/Board view because `useTasks.ts`'s `t.projectId === projectId` filter silently matched nothing against raw snake_case `project_id`) + unit-tested. `task.getSubtree`/`task.getDependencies`/`task.addEdge`/`task.grant`/`task.resolvePermission`/`task.aiDecompose`/`task.aiApply`/`task.execute` left unfixed — see "Related, not fixed here".

## Root cause

Same bug class as [BUG-020](./BUG-020-git-status-wscompat-channel-returns-raw-proto-not-frontend-shape.md)/[BUG-022](./BUG-022-aiprovider-resolve-returns-raw-snake-case-proto.md): `taskv1.Task`'s generated Go struct (`proto/gen/go/orca/task/v1/task.pb.go:390-436`) has plain `encoding/json` struct tags in **snake_case** for every multi-word field (`json:"tenant_id"`, `json:"parent_id"`, `json:"prompt_template"`, `json:"ai_context"`, `json:"pr_url"`, `json:"task_number"`, `json:"workflow_template_id"`, `json:"agent_session_id"`, ...) — only single-word fields (`id`, `title`, `status`, `labels`) happen to look camelCase. `task.create`/`task.get`/`task.update` all `return resp.GetTask(), nil` — the raw proto, unmapped.

`taskv1.AddCommentResponse{id, author_id, content, created_at}` has the identical problem for `task.addComment`/`task.listComments`.

Confirmed live against `frontend/src/shared/task-types.ts`'s `OrcaTask`, which expects `aiContext`, `promptTemplate`, `taskNumber`, `prUrl`, `workflowTemplateId` (camelCase) — every one of these is `undefined` on every `task.get`/`task.create`/`task.update` response today. `TaskDetail.tsx`/`TaskPromptEditor.tsx` (real, already-built UI reading `task.promptTemplate`, `task.description`, etc.) have been silently broken since they were written, never caught because nothing in this codebase's test suite exercises the real wire JSON shape end-to-end.

## Fix

Added `taskView`/`toTaskView(*taskv1.Task) taskView` (camelCase mirror, same pattern as `gitStatusResultView`/`providerAccountView`) and `commentView`/`toCommentView`, applied to `task.create`, `task.get`, `task.update`, `task.addComment`, `task.listComments`.

## Follow-up bug introduced by this fix, found + fixed live while testing BL-TG-05

`taskView`'s fields were all plain (no `omitempty`) except a few pointer/date fields — meaning a
root task's empty `ParentID` marshaled to `"parentId": ""`, not an absent key. `OrcaTask.parentId`
is typed optional (`parentId?: string`) precisely because `TaskTreeView.tsx`'s root-level render
compares `t.parentId === null` — `"" !== null`, so **every newly created root task silently
disappeared from the default Tree view** (confirmed visible in DAG view, which doesn't filter by
parent, but absent from Tree/Board). Live-reproduced: user created 2 tasks, saw them in DAG, saw
nothing in Tree (the default view). Fixed by adding `omitempty` to every `taskView` field the
frontend's `OrcaTask` type marks optional (`projectId`, `parentId`, `description`, `reporterId`,
`assigneeId`, `aiContext`, `promptTemplate`, `workflowTemplateId`, `taskNumber`, `prUrl`,
`workflowExecId`, `ownerId`, `worktreeId`, `agentSessionId`, `shareToken`) — required fields
(`title`, `type`, `status`, `priority`, `labels`, `visibility`, `progressPercent`) keep no
`omitempty`, since those are never legitimately absent. Regression test:
`TestTaskGetChannel_RootTaskOmitsParentIDFromWire`.

## Related, NOT fixed here

`task.list` (`ListTasksResponse{tasks: []Task, next_page_token}`), `task.getSubtree`, `task.getDependencies` (returns `[]Task` directly), `task.addEdge`, `task.grant`, `task.resolvePermission`, `task.aiDecompose`, `task.aiApply`, `task.execute` (`TaskServiceExecuteResponse{execution_ref, async}` — `execution_ref` is snake) all have the same raw-proto issue — deliberately left unfixed in this pass, scoped to exactly what [BL-TG-05](../../../../docs/logic/task-graph/BL-TG-05-spec-approve-build-loop.md) needs. Needs its own follow-up audit before any of these are relied on.

## Related

- [BUG-020](./BUG-020-git-status-wscompat-channel-returns-raw-proto-not-frontend-shape.md), [BUG-022](./BUG-022-aiprovider-resolve-returns-raw-snake-case-proto.md) — same bug class, found earlier in this session in `git.*`/`aiProvider.*`.
- [BUG-024](./BUG-024-use-task-update-wrong-request-shape.md) — the sibling request-side bug found in the same investigation (`useTask.ts`'s `updateTask` sends a shape the handler can't decode at all).
