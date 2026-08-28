# TASK-PRF-04-04: Add pure `agent_environment.go` env builder to `task-service` (own copy)

**From Solution:** SOL-PRF-04
**Priority:** P0 — TASK-PRF-04-08's executor edit calls these functions
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/domain/agent_environment.go` (new)
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

Same pure builder as TASK-PRF-04-03, `task-service`'s own copy —
deliberate per-service duplication (see TASK-PRF-04-03's Context for the
full rationale: `workflow-service`/`task-service` already each maintain a
parallel relay-dispatch adapter for the identical conceptual operation
— `agent_step_executor.go` vs. `simple_executor.go` — so a second small,
~80-line, zero-I/O duplication follows the same precedent). Independent of
TASK-PRF-04-03; either can land first.

## Changes to make

Create `backend-go/services/task-service/internal/domain/agent_environment.go`
with **identical content** to TASK-PRF-04-03's
`backend-go/services/workflow-service/internal/domain/agent_environment.go`
(same `package domain`, same `AgentBinaryMap`/`ResolveAgentBinary`/
`TrustPresetArgs`/`BuildAgentArgs`/`AgentEnv`/`BuildAgentEnv`/`PreambleInput`/
`BuildProjectContext` — copy verbatim, only the package's owning service
differs). See TASK-PRF-04-03's Changes-to-make section for the full source.

One difference to note: `task-service`'s `SimpleExecutor`
(TASK-PRF-04-08) resolves the task's *assignee* profile, not a per-step
`UserID` — `BuildAgentEnv`'s `userID` parameter here is whichever user id
`SimpleExecutor` resolves (assignee if `domain.Task` carries one, else the
request-context caller — see TASK-PRF-04-08's Context for that open
question). No change to this file's own code from that — it's the caller's
job to pass the right id.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/domain/... -run AgentEnvironment -v
```

Copy the same `agent_environment_test.go` coverage TASK-PRF-04-03 adds to
`workflow-service`, adjusted only for `task-service`'s import path — same
spec-fidelity table tests, same `BuildAgentEnv`/`BuildProjectContext` cases.
