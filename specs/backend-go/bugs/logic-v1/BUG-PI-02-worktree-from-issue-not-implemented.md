# BUG-PI-02: No backend-go glue connects an issue to a worktree/branch/agent-prompt

**Business Logic:** [BL-PI-02](../../../../docs/logic/project-integration/BL-PI-02-tao-worktree-tu-task.md) — Tạo Worktree từ GitHub Issue hoặc Linear Task
**Priority (per spec):** P1
**Status:** NOT_IMPLEMENTED
**Severity:** High
**Symptom:** There is no "Create Worktree" action reachable from an issue on the backend — a client wanting this flow must invent the branch name itself, fetch and sanitize the issue body itself, build the agent prompt itself, and separately call issue-status-update itself; none of that is server-side business logic. If the frontend does all of this today, it is 100% frontend-owned orchestration with zero backend-go support or enforcement of BR-PI-04/05/06.

---

## Spec summary

BL-PI-02 lets a user click "Create Worktree" from an issue/Linear-task detail view. The system must: (a) derive a branch name from the issue title/number following a `type/description-issueId` convention (BR-PI-04), (b) confirm the branch doesn't already exist, (c) create the worktree (BL-WT-01), (d) build an agent prompt from the issue's title/description/acceptance-criteria/comments after sanitizing that content (BR-PI-05), (e) start the agent (BL-AG-01) and inject the prompt, (f) update the issue's status to "In Progress" (BL-PI-03, with an opt-out per BR-PI-06), and (g) link the resulting worktree card back to the issue.

## What backend-go has

- A generic worktree-creation primitive exists and works: `CreateWorktree` usecase (`backend-go/services/git-gateway-service/internal/usecase/create_worktree.go:29-70`) runs `git worktree add` via the resolved host executor and records bookkeeping via `project-service`, with saga-style compensation on partial failure.
- The WS channel `worktree.create` (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go:36-54`) accepts `projectId`, `repoId`, `branch`, `baseRef`, and optional lineage-capture fields (`parentWorktreeId`, `origin`, `captureSource`, `taskId`, `orchestrationRunId`, `coordinatorHandle`, `createdByTerminalHandle`) — but `branch` must already be fully formed by the caller; there is no `issueId`/`issueTitle` input anywhere in this signature.
- `WorktreeLineageCapture` (`backend-go/services/git-gateway-service/internal/domain/domain.go:260-268`) has a `TaskID` field, but this is Orca's own internal `task-service` task id, not an external GitHub-issue/Linear-issue identifier — there is no `linked_issue_id`-shaped field anywhere on the worktree domain model or `project.proto` (`backend-go/proto/orca/project/v1/project.proto` has no issue-linkage field at all; only `project_id` exists at line 361).
- `IssueTrackingService.LinkIssue` (`backend-go/services/issue-tracking-service/internal/usecase/link_issue.go:28-83`) exists and durably enqueues an `orca.issuetracking.link.created` event associating an `issue_id` with a Orca **task** id — this is the closest thing to "worktree card link with issue" (step 4), but it links to a `task_id`, not a `worktree_id`, and is a fire-and-forget outbox event with no confirmed consumer found anywhere in `backend-go/` (see BUG-PI-03).

## What's missing

- **No branch-name generation from issue title/number (BR-PI-04)** — no function anywhere in `backend-go/` computes a `type/description-issueId` branch name; `git worktree add`'s branch argument is caller-supplied verbatim (`create_worktree.go`, `channels_worktree.go:40`). A repo-wide grep for `branch.*[Nn]ame.*[Ii]ssue`, `GenerateBranchName`, etc. returns zero hits.
- **No issue-content sanitization or agent-prompt construction (BR-PI-05)** — a repo-wide grep for `sanitiz` across every service returns exactly one unrelated hit (`git-gateway-service/internal/adapter/localgit/executor.go`, nothing issue-related); there is no code anywhere building an agent prompt from an `Issue`'s title/description/labels/comments.
- **No orchestration linking `ListIssues`/`GetWorkItemDetailsBySlug` → `CreateWorktree` → agent-start** — a repo-wide grep for `FromIssue`, `from_issue`, `IssueToWorktree`, `CreateWorktreeFromTask` returns zero hits. `orchestration-service`'s entire usecase package (`create_dispatch_context.go`, `create_gate.go`, `get_dispatch_context_for_task.go`, `resolve_gate.go`, `update_task_status_and_promote.go`) has zero references to "Issue" anywhere.
- **No opt-out toggle for the status update (BR-PI-06)** — since the status-update step itself doesn't exist (see BUG-PI-03), there is naturally no per-project disable flag for it either.
- **No pre-flight "branch doesn't already exist" check tied to this flow** specifically — `CreateWorktree` will simply fail if `git worktree add` fails on a colliding branch, but there's no issue-aware duplicate-branch pre-check as the spec's step 2b implies.

## See also

- `specs/backend-go/bugs/missing-v1/BUG-032-git-channels-partially-implemented.md` — the underlying `git.*`/worktree WS-channel gap analysis this flow would build on top of.

## References

- `backend-go/services/git-gateway-service/internal/usecase/create_worktree.go:1-71`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go:36-75`
- `backend-go/services/git-gateway-service/internal/domain/domain.go:254-268`
- `backend-go/proto/orca/project/v1/project.proto:361`
- `backend-go/services/issue-tracking-service/internal/usecase/link_issue.go:1-83`
- `backend-go/services/orchestration-service/internal/usecase/` (all files) — no `Issue` references
- `docs/logic/project-integration/BL-PI-02-tao-worktree-tu-task.md`
