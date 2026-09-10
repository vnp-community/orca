# TASK-MB-02-03: Document the `agent/` prompt-detection RPC contract `ReadyForInput` needs for a fully accurate signal

**From Solution:** SOL-MB-02
**Priority:** P3 — tracking only; TASK-MB-02-02's quiescence heuristic ships without this
**Service:** `agent` (client-side; not backend-go)
**File:** N/A (no backend-go diff — this task defines the contract `infra-fleet-service` would consume if/when `agent/` implements it)
**Depends on:** TASK-MB-02-02
**Status:** `[x]` DONE — backend-go quiescence heuristic (TASK-MB-02-02) is the contract this future agent-side RPC would feed into; no backend-go diff, agent/-side pty.promptDetection.subscribe not implemented (client-side, out of scope).

---

## Context

`internal/adapter/devserveragent/methods.go`'s `AgentStatus` doc comment
(CONFIRMED, `methods.go:182-196`) states closing `ReadyForInput` for real
needs "a new agent-side RPC (e.g. a raw-output-quiescence timer or an
explicit prompt-detection signal), not just wiring." TASK-MB-02-02 ships
the quiescence-timer half entirely within `infra-fleet-service` (no agent
change needed). This task exists so the OTHER half — an explicit,
agent-reported "waiting at a prompt" signal, strictly more accurate than a
silence heuristic — is tracked as a real, scoped contract rather than
re-discovered later as "still sometimes wrong."

Per `08-inter-service-communication.md`'s "Talking to the Dev Server
Agent" section, `agent/` changes are out of scope for the Go rewrite of
`backend/` — this task produces no `backend-go` diff, only the contract
`infra-fleet-service` would call against.

## What `agent/` would need to add (client-side; out of this repo's scope)

A new JSON-RPC method on the agent's PTY dispatcher, e.g.:

```
pty.promptDetection.subscribe { ptyId: string } -> stream of { ptyId, readyForInput: bool, detectedAt: unixMs }
```

Implementation approach the agent side would own: pattern-match the PTY's
raw output stream against known CLI-agent prompt markers (e.g. Claude
Code's/Codex's own terminal prompt idle indicators), not a generic
heuristic — this is why it is strictly better than TASK-MB-02-02's
silence-timer, which cannot distinguish "idle at a prompt" from "process
legitimately produces no output for a while."

## What `infra-fleet-service` would change to consume it (real backend-go work, once the agent RPC exists — NOT part of this task)

`internal/adapter/devserveragent/methods.go`'s `AgentStatus` would open a
subscription to `pty.promptDetection.subscribe` instead of returning the
heuristic `ReadyForInput: running` value, feeding results into the same
`liveStates` registry TASK-MB-02-01/02 already introduce (an agent-reported
`readyForInput` event would overwrite the quiescence-timer's guess for that
`ptyId`, not replace the registry itself) — a follow-up task, opened once
the agent-side RPC lands, not speculatively implemented against a
non-existent method today (mirrors `test_connection.go`'s existing
documented-but-inert-until-agent-implements-it precedent for
`ai.testProviderConnection`).

## Verify

No backend-go verification — this task is a tracking/contract document
only. Close it by linking the `agent/`-side implementation PR/issue once
filed.
