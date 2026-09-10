# BACKLOG-021: Workflow execution live streaming — blocked 2 layers deep on an unbuilt event catalog

**Origin:** `specs/frontend/crs/v4/workflow/tasks/FE-TASK-005-execution-live-streaming-subscription.md` (FE-SOL-002, CR-WF-007)
**Priority:** Medium — polling already works as a functioning fallback; this is a UX upgrade, not a broken feature
**Blocked on:** `CR-FLOW-TASK-003` (unified task-activity event catalog, backend-go, **📋 Proposed**) → `BE-SOL-006` (execution-live-streaming, backend-go, **📋 Proposed**) — a 2-layer backend dependency chain, neither layer started
**Owner:** whoever owns the backend-go workflow-service roadmap

---

## What this is

`ExecutionMonitor`/`WorkflowMonitor` poll for execution status today (works,
already shipped via other v4 tasks this session — pause/resume, status
types). FE-TASK-005 wants to replace polling with a live event subscription
(`task.activity:{id}`/`workflow.activity:{id}` channel via
`subscribeRuntimeEvent`) for lower latency.

## Why it's blocked

Neither the event channel nor its shape exists at any layer:

- `CR-FLOW-TASK-003` (a unified event catalog spanning task/workflow
  activity, layer 1) is still `📋 Proposed` in backend-go's own CRs.
- `BE-SOL-006` (execution-live-streaming, layer 2, depends on layer 1's
  catalog shape) is also `📋 Proposed` — its own status line says so
  explicitly: "not yet implemented, blocked on CR-FLOW-TASK-003."
- No `subscribeRuntimeEvent`-compatible channel for either
  `task.activity:{id}` or `workflow.activity:{id}` exists in
  `wscompat` today.

Building the frontend subscription now would mean guessing at an API shape
that CR-FLOW-TASK-003 hasn't decided yet — the frontend task's own doc
explicitly warns against this ("không tự thiết kế lại backend event
schema").

## What unblocks this

1. `CR-FLOW-TASK-003` lands (defines the unified event catalog shape).
2. `BE-SOL-006` implements the execution-specific event publishing against
   that shape.
3. Only then does FE-TASK-005 become a real, unblocked frontend task —
   revisit its existing doc (design already sketched, just waiting on a
   real API to bind to).
