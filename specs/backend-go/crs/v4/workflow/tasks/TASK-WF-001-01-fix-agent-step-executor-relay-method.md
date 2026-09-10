# TASK-WF-001-01: Fix `AgentExecutor`'s relay method (`agent.exec` → `agent.execPrompt`)

**From Solution:** BE-SOL-001
**Priority:** P0
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/internal/adapter/infrafleetclient/agent_step_executor.go`
**Depends on:** None
**Status:** `[x]` DONE

## Execution notes (2026-09-09)

Re-verified against live file before editing — matched the task's
description byte for byte. Applied exactly the two changes:

1. `agentExecMethod` constant flipped from `"agent.exec"` to
   `"agent.execPrompt"`; doc comment rewritten to state the fix is applied
   (kept the historical "why agent.exec looked plausible" context, dropped
   the "reconcile before depending on this in production" hedge since it's
   now reconciled).
2. `agentExecParams` widened with `Model`, `AccountID`, `StepID`,
   `Env map[string]string` (all `omitempty`), left unpopulated at the
   `Execute` call site as instructed — that's TASK-WF-002-03's job.

Added `TestAgentExecutor_RelayUsesExecPromptMethodAndParamsShape`
asserting the relay call's method name (`"agent.execPrompt"`) and params
JSON shape explicitly, including that the four new fields marshal as
absent (not empty-string/null) when unset.

**Verify output:**
```
go build ./services/workflow-service/...   # clean, no output
go vet ./services/workflow-service/...     # clean, no output
go test ./services/workflow-service/internal/adapter/infrafleetclient/... -v
# ok  	github.com/stablyai/orca-go/services/workflow-service/internal/adapter/infrafleetclient	0.006s
# 12/12 tests PASS (6 pre-existing agent tests + 1 new regression test + 5 shell/notification tests)
```

---

## Context

Re-verified directly against the live file — it matches BE-SOL-001's
description exactly, byte for byte:

- `agent_step_executor.go:12-25` carries a doc comment that already
  documents this exact bug: `agent.exec` (used by TS's
  `StepExecutors.executeAgent()`, `backend/src/main/workflow/StepExecutors.ts`)
  turned out to be a real, but wrong, RPC — a generic
  `{binary, args, cwd, stdin, env, timeoutMs}` process-exec call with no
  prompt/model/trustPreset concept — and the correct method for a
  prompt-driven agent step is `agent.execPrompt`.
- Line 26 still has the bug live: `const agentExecMethod = "agent.exec"`.
- The current `agentExecParams` struct (lines 30-34) is exactly
  `{Prompt, WorktreePath, TrustPreset}` — no `Model`/`AccountID`/`StepID`/
  `Env` fields.
- `Execute` (lines 51-67) builds `agentExecParams` inline from `cfg
  domain.AgentStepConfig` and calls `relay(ctx, e.client, cfg.ConnectionID,
  agentExecMethod, agentExecParams{...}, &result)`.

`shell_step_executor.go` and `notification_step_executor.go` were also
read directly and confirmed correct as BE-SOL-001 states: `shellExecMethod
= "shell.exec"` with `{Script, Env}` params, and
`notificationSendMethod = "notification.send"` with `{Channel, Message}`
params — both match their real handler shapes. This task touches neither
file.

## Changes to make

In `backend-go/services/workflow-service/internal/adapter/infrafleetclient/agent_step_executor.go`:

1. Change the constant on line 26:

```go
// agentExecMethod is the Relay method name AgentExecutor uses — see the
// doc comment above for the "agent.exec" investigation this corrects.
const agentExecMethod = "agent.execPrompt" // was: "agent.exec"
```

Update the doc comment above it (lines 12-25) so it no longer reads as an
open, unresolved investigation — state plainly that the fix below has been
applied, keeping the historical context (why `agent.exec` looked plausible
but is a different RPC) for future readers.

2. Widen `agentExecParams` (lines 28-34):

```go
// agentExecParams is the params_json payload AgentExecutor sends to
// infra-fleet-service's Relay RPC for the "agent.execPrompt" method.
type agentExecParams struct {
	Prompt       string            `json:"prompt"`
	WorktreePath string            `json:"worktreePath,omitempty"`
	TrustPreset  string            `json:"trustPreset,omitempty"`
	Model        string            `json:"model,omitempty"`
	AccountID    string            `json:"accountId,omitempty"`
	StepID       string            `json:"stepId,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
}
```

`Model`, `AccountID`, `StepID`, and `Env` are added to the struct now so
BE-SOL-002 (server/provider resolution) doesn't need a second struct-shape
change immediately after this one lands, but this task leaves them empty
— `Execute`'s call site (lines 51-67) keeps populating only `Prompt`,
`WorktreePath`, `TrustPreset` from `cfg`, same as today. Sending the four
new fields empty is not a regression: `agent.execPrompt`'s real handler
already treats an absent `model` as "default to claude" and an absent
`accountId` as "rely on the CLI's own already-authenticated state" (the
same fallback `task-service`'s `SimpleExecutor` already relies on for its
own, already-correct `agent.execPrompt` call).

3. `Execute`'s body (lines 51-67) is otherwise unchanged by this task —
   do not add `Model`/`AccountID`/`StepID`/`Env` population here; that is
   BE-SOL-002's job (TASK-WF-002-03), not this one's.

## Not in scope

- Resolving real values for `Model`/`AccountID`/`Env` — TASK-WF-002-03.
- Any `agent/`-side change — `agent.execPrompt`'s real handler is already
  correct and unchanged by this task.
- `shell_step_executor.go`/`notification_step_executor.go` — confirmed
  correct, not touched.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go vet ./services/workflow-service/...
go test ./services/workflow-service/internal/adapter/infrafleetclient/... -v
```

Expected: clean build; the existing `agent_step_executor_test.go` suite
(if it asserts on the old `"agent.exec"` method name or the old 3-field
params literal) needs updating to assert `agentExecMethod ==
"agent.execPrompt"` and the widened `agentExecParams` JSON shape —
`Model`/`AccountID`/`StepID` marshal as absent (`omitempty`) when empty,
matching `agent.execPrompt`'s documented default-handling. Add a
regression test asserting the relay call's method name and params shape
explicitly, since this is exactly the class of bug that shipped
undetected before (a compiling struct literal that silently didn't match
the real handler's contract).
