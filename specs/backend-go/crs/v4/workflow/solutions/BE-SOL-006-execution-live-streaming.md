# BE-SOL-006: Workflow execution live streaming — publish into CR-FLOW-TASK-003's catalog

**Resolves:** [CR-WF-007](../../../../../../docs/crs/v4/workflow/CR-WF-007-execution-live-streaming.md) (backend-go portion — frontend consumption is [FE-SOL-002](../../../../../frontend/crs/v4/workflow/solutions/FE-SOL-002-execution-live-streaming.md))
**Service:** `workflow-service` only
**Hard dependency:** [`docs/crs/v3/flow-task/CR-FLOW-TASK-003`](../../../../../../docs/crs/v3/flow-task/CR-FLOW-TASK-003-unified-task-activity-event-catalog.md) must land first — do not build an independent event schema
**Affected files (proposed):**
- `backend-go/services/workflow-service/internal/usecase/wave_dispatcher.go` (publish calls only)
**Status:** 📋 Proposed — not yet implemented, blocked on CR-FLOW-TASK-003

---

## Current state

No RPC or wscompat channel pushes step/execution events today —
`workflow.proto` has no `StreamExecutionEvents` (TDD §3 sketches one,
`workflow-service.md:67`, never added to the real proto), and
`channels_workflow.go`'s 11 channels have nothing named `step.output`/
`step.completed`. `useWorkflowExecution.ts` polls `workflow.getExecution`
every 4 seconds as an explicitly-labeled interim stopgap.

## Design — publish at the point CR-FLOW-TASK-003 already designates

This solution does not design an event schema — `CR-FLOW-TASK-003` already
specifies exactly this extension point (`orca.workflow.step.completed`/
`.failed`, published from `wave_dispatcher.go`, joining the unified
`task.activity:{taskId}` channel). This solution's only job is landing that
one publish call once CR-003's outbox/event-catalog machinery exists:

```go
// wave_dispatcher.go — after a step's StepExecutor.Execute returns
outbox.Publish(ctx, tx, "orca.workflow.step.completed", StepCompletedEvent{
    ExecutionID: exec.ID, StepID: step.ID, Status: result.Status,
    OriginTaskID: exec.OriginTaskID, // empty when the execution didn't originate from a Task (CR-FLOW-TASK-002)
})
```

For an execution with no `OriginTaskID` (a workflow run independently of
any Task — a legitimate use case per CR-FLOW-TASK-002's own risk note),
this solution adds a parallel channel key so standalone runs still get live
updates:

```
workflow.activity:{executionId}   — same event shape, keyed by executionId instead of taskId
```

## Not in scope (per the CR)

- Continuous stdout/output streaming per step — that is task-graph's
  [BE-SOL-006](../../task-graph/solutions/BE-SOL-006-task-execute-streaming-relay.md)/[SOL-AG-TG-002](../../../../../agent/crs/v4/task-graph/solutions/SOL-AG-TG-002-agent-chunk-streaming.md)
  mechanism — this solution only publishes discrete step-completed/failed
  events, matching CR-FLOW-TASK-003's own scope boundary.
- The outbox/eventbus mechanism itself — reused from CR-FLOW-TASK-003/
  SOL-PW-04 verbatim, not redesigned here.
- Resume-after-restart event replay — `resumeRunningExecutions()`'s
  existing behavior is unchanged; a follow-up test (see Test plan) confirms
  it doesn't double-publish, but this solution doesn't redesign the resume
  path itself.

## Test plan

- Event published exactly once per step completion/failure, correct
  channel key depending on `OriginTaskID` presence.
- `resumeRunningExecutions()` after a simulated restart does not re-publish
  already-delivered events for steps that completed before the restart.

## References

- [CR-WF-007](../../../../../../docs/crs/v4/workflow/CR-WF-007-execution-live-streaming.md)
- [CR-FLOW-TASK-003](../../../../../../docs/crs/v3/flow-task/CR-FLOW-TASK-003-unified-task-activity-event-catalog.md)
