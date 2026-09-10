# TASK-WF-006-01: [BLOCKED] Publish `orca.workflow.step.completed`/`.failed` into CR-FLOW-TASK-003's catalog

**From Solution:** BE-SOL-006
**Priority:** P2 — BLOCKED, not orderable against the other tasks in this directory until its hard dependency lands; do not schedule this ahead of its blocker regardless of nominal priority
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/internal/usecase/wave_dispatcher.go` (publish calls only)
**Depends on:** [`docs/crs/v3/flow-task/CR-FLOW-TASK-003`](../../../../../../docs/crs/v3/flow-task/CR-FLOW-TASK-003-unified-task-activity-event-catalog.md) — **hard blocker, not a soft ordering preference.** Also depends on that CR's own task breakdown ([`specs/backend-go/crs/v3/flow-task/tasks/TASK-FT-003-01` through `-05`](../../../../crs/v3/flow-task/tasks/)) actually landing the outbox/event-catalog machinery this task publishes into.
**Status:** `[ ]` BLOCKED — do not start until CR-FLOW-TASK-003's outbox/event-catalog machinery exists in `orchestration-service`/`workflow-service`; see Context for why this is a hard block, not a scope note

---

## Why this exists as a task file, not just a README note

`specs/backend-go/crs/v3/flow-task/tasks/README.md`'s precedent (BE-SOL-004
in that series) omits task files entirely when a solution's own text is
"don't build anything yet" with no concrete design to break down — that
solution literally proposes zero code change. BE-SOL-006 is different: it
**does** specify a concrete, small, well-defined code change (one publish
call site, one new channel-key convention for `OriginTaskID`-less
executions) — it is fully specified, just gated on a dependency that
hasn't landed. `specs/backend-go/bugs/logic-v1/tasks/TASK-AG-04-05-blocked-switch-inherits-credential-injection-gap.md`
is the closer precedent: a real task file exists, with `Status: [ ] BLOCKED`
and an explicit "needs X first" note, rather than being omitted. This task
follows that convention: concrete enough to be worth writing down now (so
whoever unblocks it doesn't have to re-derive the design from BE-SOL-006
cold), but explicitly marked as not pickable today.

**Do not flip this to `[ ] TODO`** until CR-FLOW-TASK-003's outbox
machinery is confirmed live in both `orchestration-service` and
`workflow-service` — flipping the checkbox without that confirmation
would let an implementing agent start work that cannot compile against a
schema that doesn't exist yet.

## Context

Re-verified live: `workflow.proto` has no `StreamExecutionEvents` RPC —
confirmed via the same `grep -n "rpc "` pass used for every other task in
this directory (11 RPCs total before TASK-WF-005-01/-02/-03 add more,
none named `Stream*`). `channels_workflow.go`'s (currently) 11 registered
channels have nothing named `step.output`/`step.completed`. No
`outbox.Publish`-style call exists anywhere in `workflow-service` today —
confirmed via direct search for `outbox.` across
`internal/usecase/wave_dispatcher.go` and the rest of that package.

`docs/crs/v3/flow-task/CR-FLOW-TASK-003-unified-task-activity-event-catalog.md`
exists and, per BE-SOL-006's framing, already specifies the exact
extension point this task needs (`orca.workflow.step.completed`/`.failed`,
published from `wave_dispatcher.go`, joining the unified
`task.activity:{taskId}` channel) — this task's only job, once that
machinery exists, is landing the one publish call, not designing a new
event schema.

`domain.WorkflowExecution` has no `OriginTaskID` field today (confirmed —
that field is `CR-FLOW-TASK-002`'s addition, tracked in
`specs/backend-go/crs/v3/flow-task/tasks/TASK-FT-002-02-workflow-service-origin-task-id-migration.md`,
also not yet landed as of this writing). This task's channel-key logic
(`workflow.activity:{executionId}` for executions with no
`OriginTaskID`) is therefore ALSO blocked on that migration existing, in
addition to CR-FLOW-TASK-003's outbox machinery — two independent
prerequisites, both unmet today.

## Changes to make (once unblocked)

```go
// wave_dispatcher.go — after a step's StepExecutor.Execute returns,
// inside runStep (internal/usecase/wave_dispatcher.go:199-214) or its
// caller dispatchStep (:172-191) — confirm the exact insertion point
// against CR-FLOW-TASK-003's real, landed outbox call signature at
// implementation time, since that signature does not exist to check
// against yet.
outbox.Publish(ctx, tx, "orca.workflow.step.completed", StepCompletedEvent{
	ExecutionID:  exec.ID,
	StepID:       step.ID,
	Status:       result.Status,
	OriginTaskID: exec.OriginTaskID, // empty when the execution didn't originate from a Task (CR-FLOW-TASK-002's own risk note: this is a legitimate, expected case, not an error)
})
```

For an execution with no `OriginTaskID`, also publish (or key) onto a
parallel channel so standalone runs still get live updates:

```
workflow.activity:{executionId}   — same event shape, keyed by executionId instead of taskId
```

**Do not design the outbox/eventbus mechanism itself here** — per
BE-SOL-006's explicit "Not in scope": it is reused verbatim from
CR-FLOW-TASK-003/SOL-PW-04, not redesigned by this task.

## Not in scope

- Continuous stdout/output streaming per step — a different mechanism
  (`specs/backend-go/crs/v4/task-graph/solutions/BE-SOL-006-task-execute-streaming-relay.md`
  and `specs/agent/crs/v4/task-graph/solutions/SOL-AG-TG-002-agent-chunk-streaming.md`)
  — this task only publishes discrete step-completed/failed events.
- The outbox/eventbus mechanism itself.
- Resume-after-restart event replay design — `resumeRunningExecutions()`'s
  existing behavior is unchanged; a test (see Test plan) confirms no
  double-publish, but the resume path itself isn't redesigned here.

## Test plan (once unblocked)

- Event published exactly once per step completion/failure, correct
  channel key depending on `OriginTaskID` presence.
- `resumeRunningExecutions()` after a simulated restart does not
  re-publish already-delivered events for steps that completed before the
  restart.

## Verify (once unblocked)

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go test ./services/workflow-service/internal/usecase/... -run "TestWaveDispatcher_PublishesStepCompleted|TestResumeExecutions_NoDoublePublish" -v
```

## Unblock checklist — re-check both before flipping Status to TODO

1. `CR-FLOW-TASK-003`'s outbox/event-catalog machinery is live in
   `orchestration-service` and consumable from `workflow-service` —
   confirm by checking `specs/backend-go/crs/v3/flow-task/tasks/README.md`'s
   task statuses (`TASK-FT-003-01` through `-05`), not just this
   directory.
2. `CR-FLOW-TASK-002`'s `origin_task_id` migration/field has landed on
   `domain.WorkflowExecution` — confirm via
   `specs/backend-go/crs/v3/flow-task/tasks/TASK-FT-002-02-workflow-service-origin-task-id-migration.md`'s
   status.

Only once both are confirmed `DONE` (not just "in review") should this
task's `Status` move to `[ ] TODO`.
