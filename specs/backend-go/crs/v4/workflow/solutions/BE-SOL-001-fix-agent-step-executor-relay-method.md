# BE-SOL-001: Fix `AgentExecutor`'s relay method (`agent.exec` → `agent.execPrompt`)

**Resolves:** [CR-WF-001](../../../../../../docs/crs/v4/workflow/CR-WF-001-fix-agent-step-executor-relay-method.md)
**Service:** `workflow-service` only
**Affected files (proposed):**
- `backend-go/services/workflow-service/internal/adapter/infrafleetclient/agent_step_executor.go`
**Status:** 📋 Proposed — not yet implemented

---

## Current state — the file already documents its own bug, unfixed

This is the smallest-footprint solution in either v4 series: the bug and
its fix are already fully written up in the file's own doc comment
(`agent_step_executor.go:12-34`), which recounts the exact investigation —
`agent.exec` (`agent-rpc-dispatch-agent-exec.ts:72`) is a real RPC, just the
wrong one (generic `{binary, args, cwd, stdin, env, timeoutMs}` exec, no
prompt/model/trustPreset concept); the correct method for a prompt-driven
agent step is `agent.execPrompt` (`:176`), already used correctly by
`task-service.SimpleExecutor` (confirmed directly — see
[BE-SOL-005](../../task-graph/solutions/BE-SOL-005-task-agent-execution-permission-and-complex-executor.md)
in the task-graph series, which documents that call site in full). The
comment stops short of actually applying the fix — `agentExecMethod` is
still `"agent.exec"` at line 26.

`shell_step_executor.go`/`notification_step_executor.go` are confirmed
correct (call `shell.exec`/`notification.send` with matching shapes) —
this solution touches nothing in either file.

## Design — the fix

```go
// agent_step_executor.go
const agentExecMethod = "agent.execPrompt" // was: "agent.exec"

type agentExecParams struct {
    Prompt       string            `json:"prompt"`
    WorktreePath string            `json:"worktreePath"`
    TrustPreset  string            `json:"trustPreset,omitempty"`
    Model        string            `json:"model,omitempty"`
    AccountID    string            `json:"accountId,omitempty"`
    StepID       string            `json:"stepId,omitempty"`
    Env          map[string]string `json:"env,omitempty"`
}
```

`Model`/`AccountID` are added to the struct now (so [BE-SOL-002](./BE-SOL-002-server-and-provider-resolution.md)
doesn't need a second struct-shape change immediately after this one lands)
but are left empty by this solution — populating them is BE-SOL-002's job,
not this one's. Sending them empty is not a regression: `agent.execPrompt`'s
real handler already treats an absent `model` as "default to claude" and an
absent `accountId` as "rely on the CLI's own already-authenticated state"
(confirmed directly in `agent-print-mode-exec.ts`, see BE-SOL-005's citation)
— the exact same fallback `SimpleExecutor` already relies on today.

## Test plan

- Integration test: a workflow with one `agent`-type step runs end-to-end
  against a fake `infra-fleet-service`/agent double, asserting the relay
  call's method name is `agent.execPrompt` and its params shape matches
  `agentExecParams` (not the old `{prompt, worktreePath, trustPreset}`
  literal, which happened to be a subset that partially-serialized without
  a compile error but was rejected by the real handler at runtime).
- Regression: `shell_step_executor.go`/`notification_step_executor.go`
  behavior unchanged (no code touched, but re-run their existing test suite
  to confirm no accidental shared-state coupling).

## Not in scope (per the CR)

- Resolving real values for `Model`/`AccountID`/`Env` — [BE-SOL-002](./BE-SOL-002-server-and-provider-resolution.md).
- Any `agent/`-side change — `agent.execPrompt`'s real handler is already correct and unchanged by this solution.

## References

- [CR-WF-001](../../../../../../docs/crs/v4/workflow/CR-WF-001-fix-agent-step-executor-relay-method.md)
- `specs/backend-go/bugs/logic-v1/solutions/SOL-PRF-04-profile-aware-agent-execution.md`
- `backend-go/services/workflow-service/internal/adapter/infrafleetclient/agent_step_executor.go:12-34`
