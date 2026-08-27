# BUG-PW-04: Workspace Integration — no cross-panel/cross-service wiring exists for any of the four integration flows

**Business Logic:** [BL-PW-04](../../../../docs/logic/project-workspace/BL-PW-04-workspace-integration.md) — Workspace Integration (Agent + Git + Tasks + Workflows)
**Priority (per spec):** P0
**Status:** NOT_IMPLEMENTED
**Severity:** High
**Symptom:** Every individual panel's operations work in isolation (agent runs, git commits, task updates, workflow executions are each independently real), but none of them trigger each other. A developer who finishes an agent run does not see the Git tab or Explorer auto-refresh, a linked task does not advance to "review"; committing with a `#TG-123`-style reference in the message does not close the task or record a PR URL; a workflow's "create-pr" step does not trigger a remote-ref sync or notification. All four of BL-PW-04's named integration flows require backend-go orchestration logic that does not exist anywhere in the codebase today.

---

## Spec summary

BL-PW-04 specifies four cross-panel flows that must fire automatically as side effects of actions in one panel: (1) Agent complete → auto-refresh Git tab + Explorer decorations + advance linked task to "review"; (2) Git commit whose message references a task ID (`#TG-123`) → auto-close that task and record actual hours/PR URL; (3) Workflow "create-pr" step completing → sync remote refs and surface a notification; (4) Task → Agent flow setting the active worktree and pre-filling the agent prompt. It also specifies a `workspaceEvents` event bus (`agent.complete`, `git.commit`, `worktree.switched`) that panels subscribe to.

## What backend-go has

The individual primitives each flow would compose sit in separate services and are independently real:

- `task-service` has a generic `UpdateTask` RPC (`backend-go/proto/orca/task/v1/task.proto:32`) capable of changing a task's status.
- `git-gateway-service.Commit` is real (`backend-go/services/git-gateway-service/internal/usecase/commit.go`, wired via `channels_git.go:49-66`).
- A generic async event-bus/notification pipeline exists at the infra level: `backend-go/common/eventbus/eventbus.go`, `backend-go/common/outbox/outbox.go`, and `notification-service`'s consumer path (`backend-go/services/notification-service/internal/usecase/handle_incoming_event.go:29-45`, `HandleIncomingEvent.Execute` — "translate a consumed domain event into a NotificationEvent, then fan it out"). This is real, working plumbing — but it is subject-agnostic infrastructure, not a wired flow for any of this BL's specific events.
- `workflow-service.HasActiveExecutions` is real (see `BUG-PW-01` in this directory) but is a read-only query, not an event trigger.

## What's missing

Every flow-specific piece of glue is absent — confirmed by direct search, not by absence-of-evidence:

- **No service publishes an `agent.complete`-equivalent domain event.** `orchestration-service` (the service that would know when an agent/automation run finishes) has zero references to `TaskServiceClient`/`taskv1.` anywhere in its codebase (`grep -rln "TaskServiceClient\|taskv1\." backend-go/services/orchestration-service` → no matches). There is no code path that, on agent completion, calls `TaskService.UpdateTask` to move a linked task to "review," and nothing publishes an event another service could react to for this purpose.
- **No commit-message → task-ID regex/auto-close logic exists anywhere.** `grep -rn "recordActualHours\|RecordActualHours\|#TG"` across all of `backend-go` returns zero matches. `git-gateway-service.commit.go` has no reference to task IDs, and `task-service` has no reference to git commits, SHAs, or `pr_url` (`task.proto` has no `pr_url`/PR-tracking field at all — confirmed via `grep -n "pr_url\|PrUrl\|worktree_id\|WorktreeId" backend-go/proto/orca/task/v1/task.proto` returning no matches). The entire "Integration Flow 2: Git Commit → Task Status" flow (task-ID regex detection, `TaskService.update(taskId, {status:'done'})`, `recordActualHours`, `Task.prUrl = PR.url`) has no backend-go implementation.
- **No workflow-step → git-sync coupling.** `workflow-service` has zero references to `gitgateway`/`GitGateway` anywhere (`grep -rln "gitgateway\|GitGateway" backend-go/services/workflow-service` → no matches). "Integration Flow 3: Workflow Step → Git" (auto-fetch after a push step, notification on PR creation) has no implementation.
- **No domain event is ever published for any of these triggers.** A repo-wide search for an actual `.Publish(` call outside `tenant-service`/`notification-service`/`auth-service`'s own internal concerns (`grep -rn "func.*Publish\|\.Publish("` across every service) turns up no git-gateway-service, task-service, orchestration-service, or workflow-service call site that emits a commit/agent-complete/workflow-step event onto the bus. The generic `notification-service` consumer exists to receive such events, but nothing produces the ones this BL needs.
- **Integration Flow 4 (Task → Agent → Workspace)** is largely a frontend state-wiring concern (pre-filling the prompt editor, setting `WorkspaceContext.currentWorktree`), but even its one backend-relevant piece — associating a task with a specific worktree/agent session so a later commit or agent-complete event could be attributed back to it — has no schema or usecase support (`task.proto` has no worktree-linkage field, confirmed above).
- **The `workspaceEvents` bus itself (`agent.complete`, `git.commit`, `worktree.switched`)** is specified as an in-process frontend event emitter, so it is not itself a backend-go gap — but every one of the *backend* actions it would need to originate from (task auto-update, ref sync, notification dispatch) is what's missing.

## See also

No prior `missing-v1`/`api-v1` bug covers this cross-cutting integration gap directly — the closest related reports (`BUG-034-task-channels-not-implemented.md`, `BUG-018-orchestration-channels-not-implemented.md`, `BUG-030-workflow-channels-not-implemented.md`) describe missing WS-channel wiring for those services' own CRUD surfaces, not the cross-service orchestration this BL specifies; check their current status separately since several sibling namespaces in this audit turned out to be far more complete than their `missing-v1` reports describe.

## References

- `docs/logic/project-workspace/BL-PW-04-workspace-integration.md:19-127` — all four integration flows and the `workspaceEvents` bus sketch
- `backend-go/proto/orca/task/v1/task.proto:32` — `UpdateTask` (generic primitive, no auto-trigger)
- `backend-go/services/git-gateway-service/internal/usecase/commit.go` — no task-ID awareness
- `backend-go/common/eventbus/eventbus.go`, `backend-go/common/outbox/outbox.go` — generic infra, not flow-specific
- `backend-go/services/notification-service/internal/usecase/handle_incoming_event.go:29-45` — generic consumer with no producer for this BL's events
- `backend-go/services/workflow-service/internal/usecase/has_active_executions.go` — read-only query, no event emission
- Absence confirmed via: `grep -rln "TaskServiceClient\|taskv1\." backend-go/services/orchestration-service`, `grep -rln "gitgateway\|GitGateway" backend-go/services/workflow-service`, `grep -rn "recordActualHours\|RecordActualHours\|#TG" backend-go`, `grep -n "pr_url\|PrUrl\|worktree_id\|WorktreeId" backend-go/proto/orca/task/v1/task.proto` — all zero matches
