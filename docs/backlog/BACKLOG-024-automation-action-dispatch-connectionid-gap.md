# BACKLOG-024: Every automation action except `create_pr` will fail to dispatch — missing `connectionId` resolution

**Origin:** `specs/frontend/crs/v4/automation/tasks/FE-TASK-AUTO-002-actions-type-plumbing.md` ("Kết quả thực tế", Phát hiện 3)
**Priority:** High — this silently breaks the end-to-end automation feature at runtime even though every layer's own tests pass
**Blocked on:** A new CR/task to resolve automation's `runContext`/`executionTargetId` into `workflow-service`'s required `connectionId` — not yet scoped anywhere
**Owner:** whoever owns the automation ↔ workflow-service ↔ infra-fleet-service integration boundary

---

## What this is

`workflow-service`'s step configs — `AgentStepConfig`, `ShellStepConfig`,
`NotificationStepConfig`, `CommitPushStepConfig` (confirmed by reading
`internal/domain/step.go` directly) — **all require `connectionId`**, a
mandatory field enforced by a shared guard in `relay_client.go:42` that
returns a clear error if it's missing. It tells `workflow-service` which
`infra-fleet-service` connection to dispatch the step's execution to.

`connectionId` here is **infra-fleet-service's connection concept** — not
`repo.connectionId` (an unrelated SSH git-remote connection concept). Easy
to confuse; confirmed they're different things by reading both call sites.

## Why every non-`create_pr` action fails today

`create_pr` dispatches through `scm-integration-service` directly and
doesn't need this field — it works. Every other action type
(`run_agent`/`run_script`/`send_notification`/`commit_push`) goes through
`workflow-service` and needs `connectionId`, but:

- The frontend's `AutomationAction` type (added in FE-TASK-AUTO-002) and
  the UI config forms built on top of it (FE-TASK-AUTO-003/004) have no
  concept of `connectionId` at all.
- No mechanism anywhere in the frontend resolves automation's own
  `runContext`/`executionTargetId` into an `infra-fleet-service`
  `connectionId`.

Net effect: an action built through the UI has the right *shape* (passes
every type-level test) but **fails when workflow-service actually tries to
dispatch it**, because the required field is simply absent.

## What NOT to do

Don't invent a guessed/hardcoded `connectionId` value to make dispatch
"work" — a wrong value fails silently or dispatches to the wrong target,
which is harder to debug than a clear "missing field" error. This was
deliberately left unresolved rather than papered over.

## What unblocks this

A dedicated CR/task that designs how automation's `runContext`/
`executionTargetId` maps to a real `infra-fleet-service` connection (likely
similar to how other dev-server-bound features resolve this today — worth
checking `git-gateway-service`'s or `terminal.create`'s resolution path for
a reusable pattern before designing a new one). Once that lands, no further
UI changes are needed — FE-TASK-AUTO-002/003/004's UI already produces the
right action shape; it just needs this one field threaded through.
