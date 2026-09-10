# SOL-AG-PW-001: Agent-side execution-progress reporting (CR-PW-006 Phase D) — design only

> **🔲 Designed — not implemented.** No agent/src code changed in this pass. Reason: Phase D is
> the highest-risk phase of CR-PW-006 (cross-repo: agent + infra-fleet-service + workflow-service
> + api-gateway; reuses the live PTY relay connection, which must not regress; and depends on
> resolving a still-open param-shape mismatch first — see §3 below). Implementing it without being
> able to verify all 4 sides end-to-end in one session would be guessing at an unowned contract,
> which this session's instructions explicitly rule out.

**CR:** [CR-PW-006](../../../../../../docs/crs/v3/project-workspace/CR-PW-006-execution-monitoring-architecture.md) — Phase D
**Depends on:** CR-PW-006 Phase B (a `ListStepExecutions`-shaped persistence target must exist
before there's anywhere for these events to land)

---

## 1. What exists today (verified by reading the actual code, not assumed)

- `agent/src/` has **zero** workflow-execution-specific code. The only workflow-related file is
  `agent/src/shared/workflow-types.ts` — pure type definitions mirroring the frontend/backend-go
  shared types, imported by nothing that dispatches or executes a step.
- A dev-server agent already maintains **one persistent relay connection** to
  infra-fleet-service, used today for PTY/terminal streaming (`agent/src/relay/` — the same
  connection `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go`'s
  `terminal.create`/`drainAttachPtyOutput` ultimately reads from via infra-fleet-service's Relay).
  Any new progress-reporting mechanism MUST reuse this connection, not open a second one — this is
  a hard design constraint, not a suggestion, per the original task brief.
- `backend-go/services/workflow-service/internal/adapter/infrafleetclient/agent_step_executor.go`
  calls `infrafleetv1.InfraFleetServiceClient.Relay(...)` **unary** — one request, one response,
  no streaming, no channel for progress while the step runs.
- `agent/src/relay/agent-rpc-dispatch-agent-exec.ts` has two real, distinct RPC handlers:
  `case 'agent.exec'` (generic process-exec: `{binary, args, cwd, stdin, env, timeoutMs}`) and
  `case 'agent.execPrompt'` (prompt-driven agent invocation: `{prompt, model, trustPreset, ...}`).

## 2. Target design (from the CR — not yet built)

1. The agent-side handler for the step's real method (see §3 for which one that should be)
   emits progress events **over the existing relay connection** as the step runs — not a new
   connection. The exact wire shape (a new relay message type alongside PTY frames, vs. a
   secondary logical channel multiplexed over the same connection) is **not decided** — this is
   the first open design question a follow-up session must resolve by reading
   `agent/src/relay/`'s connection/frame protocol in full.
2. infra-fleet-service (which already terminates this connection) or workflow-service (further
   downstream) persists each incoming progress event into `workflow.step_executions` as it
   arrives — using Phase C's `started_at`/`completed_at` columns and the existing `Status` field.
3. A new **server-streaming** gRPC method (name TBD, e.g. `SubscribeExecutionProgress`) exposes
   these events live, mirroring how `terminal.create`'s `drainAttachPtyOutput` already streams PTY
   output through api-gateway today.
4. `api-gateway`'s wscompat exposes that as a new `RegisterStreamChannel`, name
   `workflow.execution.subscribe`, in the exact shape `channels_terminal.go`'s `terminal.create`
   already uses — this part (backend-go only) is lower-risk than the agent-side event-emission
   work and could plausibly be built without agent-side changes existing yet (streaming zero
   events until an emitter exists), but this session chose not to build partial plumbing with no
   producer, per the "don't build things you can't verify end-to-end" instruction.

## 3. Open question that MUST be resolved before implementing this — found in this pass

`agent_step_executor.go`'s `agentExecMethod` constant sends `"agent.exec"` with
`{prompt, worktreePath, trustPreset}` params. But `agent.exec` (verified: `agent-rpc-dispatch-
agent-exec.ts` `case 'agent.exec'`) is a **generic process-exec RPC** expecting
`{binary, args, cwd, stdin, env, timeoutMs}` — a completely different shape. This is not a new
finding — the constant's own doc comment in `agent_step_executor.go` already documents it,
citing `specs/agent/api/gaps-and-findings.md`'s "TS Gap 4" and legacy TS's own past fix (switching
to `"agent.execPrompt"`). **backend-go's `AgentExecutor` was never updated to match.**

Consequence for this design: Phase D's progress-reporting handler needs to be added to whichever
RPC actually runs agent steps for real — and today that's ambiguous, because the RPC backend-go
calls (`agent.exec`) is very likely the wrong one for a prompt-driven agent step. Fixing the
method-name/param-shape mismatch is a **prerequisite**, not part of Phase D itself (it's a
correctness bug in the existing unary path, independent of adding streaming progress on top of
whichever RPC turns out to be correct). Recommendation for whoever picks this up: fix that
mismatch first, confirm a real agent step round-trips successfully against a real dev server, THEN
design the progress-event wire format on top of the now-correct RPC.

## 4. Why this stays design-only this session

- Cross-repo: touching `agent/src/relay/`'s connection protocol without also touching
  infra-fleet-service and workflow-service in the same pass would leave a partially-wired,
  unverifiable change.
- The connection being modified is the same one production PTY/terminal streaming already
  depends on — a mistake here has a blast radius well beyond workflow monitoring.
- The prerequisite fix in §3 was not in this CR's scope to fix, and implementing progress
  reporting on top of a known-mismatched RPC would mean building on a foundation already known to
  be wrong.

## Checklist

- [x] Confirmed agent/src has no workflow-execution code (only shared type definitions).
- [x] Confirmed the existing relay connection is the one PTY streaming uses today, and that reuse
      of it (not a second connection) is the binding design constraint.
- [x] Confirmed and further precision-checked the `agent.exec`/`agent.execPrompt` mismatch (found
      it's a param-shape mismatch on a real, existing agent RPC — not a missing-handler situation
      as an earlier pass had guessed).
- [ ] Wire protocol for progress events over the relay connection — NOT designed in detail
      (open question, see §2 point 1).
- [ ] Any code — NOT written.
