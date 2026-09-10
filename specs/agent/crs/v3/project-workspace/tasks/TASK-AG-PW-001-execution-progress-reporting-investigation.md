# TASK-AG-PW-001: Investigate + design agent-side execution progress reporting

**Status:** 🔲 Designed — not implemented (2026-09-06)
**Solution Ref:** SOL-AG-PW-001
**Priority:** 🔵 P3 (Phase D of CR-PW-006 — highest risk, lowest priority phase)

---

## What was done this session

Read (not assumed) the following to confirm/refine the architecture-gap summary this task started
from:

1. `agent/src/shared/workflow-types.ts` — confirmed: type definitions only, no dispatcher.
2. `backend-go/services/workflow-service/internal/adapter/infrafleetclient/agent_step_executor.go`
   — confirmed unary `Relay` call, no streaming; confirmed the file's own doc comment already
   documents the `agent.exec`/`agent.execPrompt` mismatch (citing
   `specs/agent/api/gaps-and-findings.md`'s "TS Gap 4").
3. `agent/src/relay/agent-rpc-dispatch-agent-exec.ts` — confirmed BOTH `agent.exec` (generic
   process-exec) and `agent.execPrompt` (prompt-driven) exist as real, distinct handlers — the
   mismatch is a **param-shape** mismatch on an RPC that does exist, not a missing-handler
   situation.
4. `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go` —
   confirmed as the reference shape (`terminal.create`/`drainAttachPtyOutput`,
   `RegisterStreamChannel`) any future `workflow.execution.subscribe` channel should mirror.

## Why not implemented

See SOL-AG-PW-001 §4. Summary: cross-repo (agent + infra-fleet-service + workflow-service +
api-gateway), reuses a connection that live PTY streaming already depends on, and has an
unresolved prerequisite bug (§3 of the solution doc) that should be fixed first, independently,
before adding streaming progress on top of it.

## Next steps for whoever picks this up

1. Fix `agent_step_executor.go`'s method/param mismatch first (separate, smaller, verifiable
   fix — not part of this task).
2. Confirm a real agent step round-trips successfully against a live dev server after that fix.
3. Only then design the wire format for progress events over the existing relay connection (open
   question — not resolved in this pass).
4. Implement CR-PW-006 Phase B/C first (this task's persistence target doesn't exist without
   them).

## Acceptance Criteria (for this investigation-only task)

- [x] Confirmed `agent/src/` has zero workflow-execution code today.
- [x] Confirmed the reuse-existing-connection constraint from the original task brief is
      accurate and binding.
- [x] Corrected an imprecise part of the original brief: the mismatch is param-shape, not
      "no handler found" — documented precisely in SOL-AG-PW-001.
- [ ] No code implemented — this is a documented, deliberate non-goal for this session.
