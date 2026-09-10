# SOL-AG-WF-001: `agent/` needs no code change for the F36 CR series — confirmation

> **📐 Confirmation-only — no code change proposed.** This document exists
> because the task explicitly asks every layer to be assessed before
> proposing solutions; the assessment for `agent/` against all 7
> [`docs/crs/v4/workflow/`](../../../../../../docs/crs/v4/workflow/) CRs concludes
> **none of them require an `agent/` change**, and this records why so that
> conclusion isn't silently assumed.

**Depends on:** nothing — reads current code only
**Affected files:** none

---

## Per-CR assessment

| CR | Touches `agent/`? | Why |
|----|:--:|-----|
| [CR-WF-001](../../../../../../docs/crs/v4/workflow/CR-WF-001-fix-agent-step-executor-relay-method.md) | ❌ | The bug is `workflow-service` calling the wrong relay method (`agent.exec` instead of `agent.execPrompt`) — `agent.execPrompt`'s real handler is already correct, confirmed directly by reading `agent-print-mode-exec.ts` in full (same file [SOL-AG-TG-001](../../task-graph/solutions/SOL-AG-TG-001-agent-env-contract-assessment.md) reads for the task-graph series). The entire fix is [BE-SOL-001](../../../../../backend-go/crs/v4/workflow/solutions/BE-SOL-001-fix-agent-step-executor-relay-method.md), a `workflow-service`-only change. |
| [CR-WF-002](../../../../../../docs/crs/v4/workflow/CR-WF-002-server-and-provider-resolution.md) | ❌ | Server/provider target resolution happens in `workflow-service` before the relay call is made — `agent/` only ever receives an already-resolved `connectionId`/`model`/`accountId`, exactly as it does today from `task-service.SimpleExecutor`. |
| [CR-WF-003](../../../../../../docs/crs/v4/workflow/CR-WF-003-variable-interpolation-and-step-types.md) | ❌ | Variable interpolation and the new `action`/`parallel` step types are `workflow-service`-internal DAG/dispatch concerns — by the time a step reaches `agent/`, its config is already a fully-interpolated, concrete `agent.execPrompt`/`shell.exec` call, identical in shape to what `agent/` receives today. |
| [CR-WF-004](../../../../../../docs/crs/v4/workflow/CR-WF-004-template-inheritance-merge-and-clone.md) | ❌ | Template inheritance/Clone is resolved entirely server-side before any execution begins. |
| [CR-WF-005](../../../../../../docs/crs/v4/workflow/CR-WF-005-template-sharing-library-and-list-executions.md) | ❌ | Sharing/library schema and RPCs are `workflow-service`/`api-gateway` concerns; share-link resolution never reaches the execution plane. |
| [CR-WF-006](../../../../../../docs/crs/v4/workflow/CR-WF-006-frontend-builder-library-pause-resume.md) | ❌ | Frontend-only. |
| [CR-WF-007](../../../../../../docs/crs/v4/workflow/CR-WF-007-execution-live-streaming.md) | ❌ | Explicitly scoped to discrete step-completed/failed events published from `workflow-service`'s outbox (reusing CR-FLOW-TASK-003's catalog) — **not** continuous stdout streaming. Continuous streaming for one-shot agent execution is a separate, already-scoped concern owned by the task-graph series ([SOL-AG-TG-002](../../task-graph/solutions/SOL-AG-TG-002-agent-chunk-streaming.md)), which F36's `agent`-type workflow steps would incidentally benefit from once built, but that CR belongs to `docs/crs/v4/task-graph/`, not this series. |

## If this changes

If a future revision of any CR-WF-00N above adds a requirement that does
reach `agent/` (e.g., a new step type needing a new agent-side primitive),
update this table rather than assuming the "no agent change" conclusion
still holds — it was true as read against the code and CRs on 2026-09-09,
not a permanent property of the feature.

## References

- [`docs/crs/v4/workflow/`](../../../../../../docs/crs/v4/workflow/)
- [SOL-AG-TG-001](../../task-graph/solutions/SOL-AG-TG-001-agent-env-contract-assessment.md), [SOL-AG-TG-002](../../task-graph/solutions/SOL-AG-TG-002-agent-chunk-streaming.md) — the task-graph series' agent solutions, cited above for the shared `agent.execPrompt` code path
