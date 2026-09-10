# task-graph (F37) solutions — index

Implementation solutions for the `task-graph` CR series
([`docs/crs/v4/task-graph/`](../../../../../../docs/crs/v4/task-graph/)).
None of these are implemented yet. Every solution below reads the real,
current `backend-go` code (file:line citations throughout) and the
`task-service`/`orchestration-service` TDDs before proposing anything —
several corrected the originating CR after that reading turned up an
already-shipped field, an already-correct design, or a smaller gap than
the CR assumed (flagged inline in each solution as "Correction relative to
CR-TG-00N" or "Current state" sections). This is deliberate: the brief for
this series is to change as little code as the real gap requires, not to
implement the CR's narrative literally where the code has moved on.

| CR | Solution | Status | Note |
|----|----------|--------|------|
| [CR-TG-001](../../../../../../docs/crs/v4/task-graph/CR-TG-001-orcatask-data-model-widening.md) | [BE-SOL-001](./BE-SOL-001-orcatask-data-model-widening.md) | 📋 Proposed | Adopts [SOL-TG-01](../../../../bugs/logic-v1/solutions/SOL-TG-01-task-graph-structural-management.md) near-verbatim |
| [CR-TG-002](../../../../../../docs/crs/v4/task-graph/CR-TG-002-ai-decompose-context-and-dependency-edges.md) | [BE-SOL-002](./BE-SOL-002-ai-decompose-context-and-dependency-edges.md) | 📋 Proposed | Adopts [SOL-TG-02](../../../../bugs/logic-v1/solutions/SOL-TG-02-ai-task-planning.md) |
| [CR-TG-003](../../../../../../docs/crs/v4/task-graph/CR-TG-003-task-access-control-team-scope-and-sharing.md) | [BE-SOL-003](./BE-SOL-003-task-access-control-team-scope-and-sharing.md) | 📋 Proposed | **Corrects** CR-TG-003's `GranteeKind`/`PermissionLevel` split proposal — real code + OPA bundle already implement the domain-computes/OPA-decides split correctly; realigns with [SOL-TG-03](../../../../bugs/logic-v1/solutions/SOL-TG-03-task-access-control.md) instead |
| [CR-TG-004](../../../../../../docs/crs/v4/task-graph/CR-TG-004-orchestration-service-coordinator-run-lifecycle.md) | [BE-SOL-004](./BE-SOL-004-orchestration-service-coordinator-run-lifecycle.md) | 📋 Proposed | Narrower than the CR assumed — `domain.CoordinatorRun` + its table already exist; only the usecase/proto/loop layer is missing |
| [CR-TG-005](../../../../../../docs/crs/v4/task-graph/CR-TG-005-task-agent-execution-permission-and-complex-executor.md) | [BE-SOL-005](./BE-SOL-005-task-agent-execution-permission-and-complex-executor.md) | 📋 Proposed | **Corrects** — the "Run-Agent prompt override" gap is already shipped end-to-end (`docs/backlog/BACKLOG-016`); real remaining gap is permission precheck + status revert + `ComplexExecutor` + env injection only |
| [CR-TG-006](../../../../../../docs/crs/v4/task-graph/CR-TG-006-task-execute-streaming-relay.md) | [BE-SOL-006](./BE-SOL-006-task-execute-streaming-relay.md) | 📋 Proposed | backend-go portion only — agent-side portion is [SOL-AG-TG-002](../../../../../agent/crs/v4/task-graph/solutions/SOL-AG-TG-002-agent-chunk-streaming.md) |
| CR-TG-007 | — | N/A | Frontend-only, see [`specs/frontend/crs/v4/task-graph/solutions/`](../../../../../frontend/crs/v4/task-graph/solutions/) |

## Dependency order

```
BE-SOL-001 (data model)
  ├── BE-SOL-002 (AI decompose)
  └── BE-SOL-003 (access control)
BE-SOL-004 (orchestration coordinator) — independent, no data-model dependency
  └── BE-SOL-005 (permission precheck + real ComplexExecutor) — needs BE-SOL-001 + BE-SOL-003 + BE-SOL-004
        └── BE-SOL-006 (streaming) — needs a real dispatch path from BE-SOL-004/005
```

## Cross-references — do not re-derive elsewhere

- [`docs/crs/v3/flow-task/`](../../../../../../docs/crs/v3/flow-task/) — cross-engine `ExecutionEngine`
  architecture, Task↔Workflow linkage, unified activity event catalog. This series' solutions
  cross-reference it (e.g. BE-SOL-005's `ReportTaskExecutionResult`) rather than duplicating.
- [`specs/backend-go/bugs/task-v1/`](../../../../bugs/task-v1/README.md) and
  [`specs/backend-go/bugs/logic-v1/`](../../../../bugs/logic-v1/) — the original bug audits and
  SOL-TG-01..04 designs this series adopts.
