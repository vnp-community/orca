# Agent Tasks — Workflow (v4, F36 CR series)

**Solutions:** [../solutions/](../solutions/)

## Why this directory has no task files

[SOL-AG-WF-001](../solutions/SOL-AG-WF-001-no-agent-change-required.md) is explicitly a
confirmation, not a design — its own header states "no code change proposed" and it exists
"because the task explicitly asks every layer to be assessed before proposing solutions." It
walks all 7 `docs/crs/v4/workflow/` CRs (CR-WF-001 through CR-WF-007) individually and concludes,
for each one, that `agent/` needs no change:

| CR | Touches `agent/`? |
|----|:--:|
| CR-WF-001 (fix agent step executor relay method) | ❌ — the bug and its fix are entirely `workflow-service`-side; `agent.execPrompt`'s handler is already correct. |
| CR-WF-002 (server/provider resolution) | ❌ — resolved server-side before the relay call reaches `agent/`. |
| CR-WF-003 (variable interpolation, new step types) | ❌ — `agent/` only ever sees an already-interpolated, concrete `agent.execPrompt`/`shell.exec` call. |
| CR-WF-004 (template inheritance/clone) | ❌ — resolved entirely server-side before execution begins. |
| CR-WF-005 (sharing/library/list executions) | ❌ — `workflow-service`/`api-gateway` schema and RPC concerns only. |
| CR-WF-006 (frontend builder/library/pause-resume) | ❌ — frontend-only. |
| CR-WF-007 (execution live streaming) | ❌ — scoped to discrete step-completed/failed events from `workflow-service`'s outbox, not continuous stdout streaming; continuous streaming for one-shot agent execution is the task-graph series' concern ([SOL-AG-TG-002](../../task-graph/solutions/SOL-AG-TG-002-agent-chunk-streaming.md), broken into tasks in [`../../task-graph/tasks/`](../../task-graph/tasks/)), not this series'. |

Creating implementation tasks for a solution whose entire content is "no `agent/` change is
needed for any of these 7 CRs" would be inventing work the solution itself says isn't there —
the same reasoning `specs/backend-go/crs/v3/flow-task/tasks/README.md`'s "Why BE-SOL-004 has no
tasks" section applies to an analogous confirmation-only solution: BE-SOL-004 states plainly "no
backend-go code change is proposed," and that document's own text is treated as the answer, not
grounds to manufacture tasks around it.

SOL-AG-WF-001 also names its own review date explicitly ("true as read against the code and CRs
on 2026-09-09, not a permanent property of the feature") and says to update its table — not to
assume the conclusion still holds — if a future revision of any CR-WF-00N adds a requirement that
does reach `agent/`. If that happens, this directory gains task files at that point; until then,
none are warranted.

## Cross-reference

- [SOL-AG-WF-001](../solutions/SOL-AG-WF-001-no-agent-change-required.md) — the confirmation this
  README summarizes.
- [`docs/crs/v4/workflow/`](../../../../../../docs/crs/v4/workflow/) — the 7 CRs assessed.
- [`../../task-graph/tasks/`](../../task-graph/tasks/) — where the one `agent/`-relevant
  streaming concern touched on in the CR-WF-007 row above is actually broken into tasks.
