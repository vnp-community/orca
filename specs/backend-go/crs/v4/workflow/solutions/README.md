# workflow (F36) solutions — index

Implementation solutions for the `workflow` CR series
([`docs/crs/v4/workflow/`](../../../../../../docs/crs/v4/workflow/)).
None of these are implemented yet — every solution reads the real, current
`workflow-service` code (file:line citations throughout) against
`workflow-service.md`'s TDD before proposing anything, and flags plainly
wherever a proposal goes beyond what the TDD's literal text sketches (most
of the sharing/library scope, and the `action`/`parallel` step types, do).

| CR | Solution | Status | Note |
|----|----------|--------|------|
| [CR-WF-001](../../../../../../docs/crs/v4/workflow/CR-WF-001-fix-agent-step-executor-relay-method.md) | [BE-SOL-001](./BE-SOL-001-fix-agent-step-executor-relay-method.md) | 📋 Proposed | Smallest solution in either series — the bug and its fix are already documented in the file's own comment, just not applied |
| [CR-WF-002](../../../../../../docs/crs/v4/workflow/CR-WF-002-server-and-provider-resolution.md) | [BE-SOL-002](./BE-SOL-002-server-and-provider-resolution.md) | 📋 Proposed | Needs a `PickByTag`-equivalent RPC verified/added on `infra-fleet-service`'s side |
| [CR-WF-003](../../../../../../docs/crs/v4/workflow/CR-WF-003-variable-interpolation-and-step-types.md) | [BE-SOL-003](./BE-SOL-003-variable-interpolation-and-step-types.md) | 📋 Proposed | Coordinate `ExecuteRequest` proto timing with CR-FLOW-TASK-002 |
| [CR-WF-004](../../../../../../docs/crs/v4/workflow/CR-WF-004-template-inheritance-merge-and-clone.md) | [BE-SOL-004](./BE-SOL-004-template-inheritance-merge-and-clone.md) | 📋 Proposed | Preserves the existing "closest-with-steps-wins" base-selection policy; extends it, doesn't replace it |
| [CR-WF-005](../../../../../../docs/crs/v4/workflow/CR-WF-005-template-sharing-library-and-list-executions.md) | [BE-SOL-005](./BE-SOL-005-template-sharing-library-and-list-executions.md) | 📋 Proposed | `ListExecutions` fix is a good fast-follow split — `WorkflowMonitor.tsx` is broken in production today without it |
| [CR-WF-006](../../../../../../docs/crs/v4/workflow/CR-WF-006-frontend-builder-library-pause-resume.md) | — | N/A | Frontend-only, see [`specs/frontend/crs/v4/workflow/solutions/`](../../../../../frontend/crs/v4/workflow/solutions/) |
| [CR-WF-007](../../../../../../docs/crs/v4/workflow/CR-WF-007-execution-live-streaming.md) | [BE-SOL-006](./BE-SOL-006-execution-live-streaming.md) | 📋 Proposed | Hard-blocked on CR-FLOW-TASK-003 landing first |

## Dependency order

```
BE-SOL-001 (hotfix) → BE-SOL-002 (resolvers)
BE-SOL-004 (inheritance merge + Clone) — independent, can run in parallel with 001/002
  → BE-SOL-003 (inputs/interpolation/step types) — coordinate ExecuteRequest proto timing with CR-FLOW-TASK-002
    → BE-SOL-005 (sharing/library — Clone from 004 is its "Import" mechanism)
BE-SOL-006 (streaming) — blocked on CR-FLOW-TASK-003, independent of the rest
```

## Cross-references — do not re-derive elsewhere

- [`docs/crs/v3/flow-task/`](../../../../../../docs/crs/v3/flow-task/) — Task↔Workflow linkage (CR-002),
  unified event catalog (CR-003, hard dependency of BE-SOL-006).
- [`specs/backend-go/bugs/logic-v1/BUG-WF-01..03`](../../../../bugs/logic-v1/), [`specs/backend-go/bugs/task-v1/BUG-TASKV1-006/007`](../../../../bugs/task-v1/) — original audits this series turns into concrete solutions.
- [`specs/backend-go/bugs/logic-v1/solutions/SOL-PRF-04`](../../../../bugs/logic-v1/solutions/SOL-PRF-04-profile-aware-agent-execution.md) — BE-SOL-001's fix is adopted from here.
