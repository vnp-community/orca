# task-graph (F37) solutions — index (agent)

Implementation solutions/assessments for the `task-graph` CR series
([`docs/crs/v4/task-graph/`](../../../../../../docs/crs/v4/task-graph/))
scoped to `agent/`. Only 2 of the 7 CRs touch `agent/` at all — the rest
(CR-TG-001..004, CR-TG-007) are backend-go/frontend only, see those
services' own `solutions/` directories.

| CR | Solution | Status | Note |
|----|----------|--------|------|
| [CR-TG-005](../../../../../../docs/crs/v4/task-graph/CR-TG-005-task-agent-execution-permission-and-complex-executor.md) | [SOL-AG-TG-001](./SOL-AG-TG-001-agent-env-contract-assessment.md) | 📐 Assessment | **Zero code change** — confirms `agent.execPrompt`'s existing `env` param already correctly overrides the buggy auto-derived task id; the actual fix is entirely in `task-service` |
| [CR-TG-006](../../../../../../docs/crs/v4/task-graph/CR-TG-006-task-execute-streaming-relay.md) | [SOL-AG-TG-002](./SOL-AG-TG-002-agent-chunk-streaming.md) | 📋 Proposed | Reuses the existing `stream.chunk`/`stream.end` convention already used by `git.execStream` and the ephemeral-VM handler — new sibling RPCs, no change to existing `agent.execPrompt`/`shell.exec` |

## Why the other 5 CRs have no agent solution here

- CR-TG-001/002/003/004 are entirely backend-go (`task-service`/
  `orchestration-service` data model, AI decompose, grants, coordinator
  lifecycle) — no `agent/` concern.
- CR-TG-007 is frontend-only (Task Board/Grant UI).
