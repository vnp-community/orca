# backend-go Solutions — Project Workspace

**CRs:** [docs/crs/v3/project-workspace/CR-PW-005/006](../../../../../docs/crs/v3/project-workspace/README.md)
**Frontend counterpart:** [specs/frontend/crs/v3/project-workspace/](../../../../frontend/crs/v3/project-workspace/solutions/README.md)
**Agent counterpart (design-only):** [specs/agent/crs/v3/project-workspace/](../../../../agent/crs/v3/project-workspace/solutions/README.md)

## Solutions

| Solution | CR | Phase | Status |
|---|---|---|---|
| [BE-SOL-001](./BE-SOL-001-workflow-wscompat-wiring.md) | CR-PW-005 | Phase A — wire existing RPCs | ✅ COMPLETED (2026-09-06) |

## What's designed but not implemented in this pass

CR-PW-006's Phase B (`ListExecutions`/`ListStepExecutions` new proto RPCs),
Phase C (`step_executions.started_at`/`completed_at` migration), and Phase D
(agent→infra-fleet-service→workflow-service live step-progress streaming +
a new `workflow.execution.subscribe` `RegisterStreamChannel`) are designed in
full in [CR-PW-006](../../../../../docs/crs/v3/project-workspace/CR-PW-006-execution-monitoring-architecture.md)
but **not implemented** — see that CR's "Trạng thái triển khai" section for
exactly what was attempted vs. deferred and why. No `BE-SOL-002` exists yet
for those phases; a future session should create one when picking this up.

## Why only BE-SOL-001 shipped this pass

CR-PW-005 (Phase A) is pure wiring of RPCs that already existed, compiled,
and had a working server implementation — safe to implement and verify
end-to-end in one session. CR-PW-006's remaining phases each require either
a proto change + `buf generate` across a shared module (Phase B), a
migration on a service other agents/sessions may also be touching (Phase C),
or cross-repo (agent + backend-go) event-emission plumbing that cannot be
fully verified without touching `agent/src/` and re-deploying the relay
connection (Phase D) — all three are documented in detail instead of
attempted speculatively, per this session's explicit instructions to only
implement what's safe, bounded, and doesn't require guessing at unowned
cross-service contracts.
