# SOL-AG-02: Add agent-aware stop/kill usecases to `infra-fleet-service` — reusing `agent.sendInput`/`agent.kill`, not `pty.sendSignal`/`pty.destroy`

**Resolves:** [BUG-AG-02](../BUG-AG-02-dung-agent-partial.md)
**Service:** `infra-fleet-service` (extended)
**Affected files (proposed):**
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (new `StopAgentSession`/`KillAgentSession` RPCs)
- `backend-go/services/infra-fleet-service/internal/usecase/stop_agent_session.go` (new)
- `backend-go/services/infra-fleet-service/internal/usecase/kill_agent_session.go` (new)
- `backend-go/services/infra-fleet-service/internal/usecase/watch_agent_stop_escalation.go` (new — 10s graceful→force timer)
- `backend-go/services/infra-fleet-service/internal/usecase/ports.go` (already extended by SOL-AG-01 with `SendAgentInput`/`KillAgent`; this solution adds `WriteActivityChecker`, see BR-AG-06)
- `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go` (wire the two new RPCs)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_agent.go` (extended by this solution with `agent.stop`/`agent.kill`)
- `backend-go/services/git-gateway-service/internal/usecase/ports.go` + a small in-memory tracker adapter (proposed, see BR-AG-06 — flagged as an open design question, not a settled sketch)
**Status:** 📋 Proposed — not yet implemented (BR-AG-06 write-lock check is a flagged open design question, see below)

---

## Design rationale (grounded in TDD)

### Agent-spawned PTYs live in a different registry than shell PTYs — existing stop/kill primitives don't reach them

BUG-AG-02 frames `StopTerminalProcess`/`KillTerminalSession`
(`stop_terminal_process.go:32-47`, `kill_terminal_session.go:27-47`) as
"generic... indistinguishable from stopping a plain shell." Reading the real
agent implementation confirms this is not just a semantic gap — it is a
**wire-level** one: `pty.sendSignal`/`pty.destroy` (what `SendSignal`/`KillPty`
call) operate against the pty-daemon's PTY store
(`agent/src/relay/pty-daemon-client.ts`), while `agent.spawn`'s PTYs are
tracked in a **separate, module-local `PTY_REGISTRY` map inside
`agent-spawner.ts`** (`agent-spawner.ts:88-96`). `handleAgentKill`
(`agent-spawner.ts:543-577`) and `handleAgentSendInput`
(`agent-spawner.ts:583-620`) both look up `PTY_REGISTRY`, not the
pty-daemon's store — calling `pty.sendSignal`/`pty.destroy` against an
agent-spawned `ptyId` would hit "PTY not found" on the pty-daemon side even
though the process is alive. **`StopTerminalProcess`/`KillTerminalSession`
cannot be reused for agent sessions even as a thin wrapper; this solution
needs sibling usecases calling `agent.kill`/`agent.sendInput`** (the
`DevServerAgentClient.SendAgentInput`/`KillAgent` methods SOL-AG-01 already
adds).

This mirrors `infra-fleet-service.md`'s own warning almost exactly: §10
"Known TS drift to account for during porting" flags that the agent runs
"two independently-implemented RPC surfaces... that frequently diverge in
method names and param shapes for the same nominal operation" and that
"porting a call site without checking which Part actually implements the
target method... is a known source of TS-side bugs" — this solution is a
direct instance of that exact warning, just discovered between two RPC
groups on the *same* Part (Part A / direct-websocket) rather than across
Parts A/B.

### Where this belongs

Same reasoning as SOL-AG-01: `StopAgentSession`/`KillAgentSession` follow
`StopTerminalProcess`/`KillTerminalSession`'s exact
resolve-session→resolve-devServer→call-agent shape
(`stop_terminal_process.go:32-47`), reusing the `resolveTerminalSession`
helper's pattern (this solution adds an equivalent
`resolveAgentSession(ctx, tenantID, sessionID, sessions, resolver)` for
`AgentSessionRepository`) and the same `ConnectionResolver`/
`DevServerAgentClient` ports — no new service.

## Design — proto

```protobuf
service InfraFleetService {
  // ... existing + SOL-AG-01's StartAgentSession ...

  // StopAgentSession sends agent.sendInput('\x03') — graceful interrupt,
  // BR-AG-05. Does not tear the session down; UPDATE ... SET status='stopped'
  // happens once agent.exited arrives (SOL-AG-05's classifier), matching
  // BL-AG-02's own step "d. Dev Server gửi event: agent.exit ... e. UPDATE
  // orca_sessions SET status='stopped'" — status transition is exit-driven,
  // not request-driven.
  rpc StopAgentSession(StopAgentSessionRequest) returns (google.protobuf.Empty);

  // KillAgentSession sends agent.kill with the given signal (default
  // SIGKILL) — full teardown, mirrors KillTerminalSession's "mark closed
  // even if the agent call fails" discipline.
  rpc KillAgentSession(KillAgentSessionRequest) returns (google.protobuf.Empty);
}

message StopAgentSessionRequest { string session_id = 1; }
message KillAgentSessionRequest {
  string session_id = 1;
  string signal      = 2; // "SIGTERM" | "SIGKILL", default SIGKILL — mirrors agent.kill's own default
}
```

## Design — `usecase/stop_agent_session.go` / `kill_agent_session.go`

```go
// StopAgentSession — BR-AG-05: graceful stop is Ctrl+C via agent.sendInput,
// not agent.kill. Mirrors StopTerminalProcess's shape exactly, swapping
// SendSignal(SIGINT) for SendAgentInput('\x03') because that's the RPC that
// actually reaches an agent-spawned PTY (see rationale above).
type StopAgentSession struct {
	sessions AgentSessionRepository
	resolver ConnectionResolver
	agent    DevServerAgentClient
}

func (uc *StopAgentSession) Execute(ctx context.Context, sessionID string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	_, devServer, session, err := resolveAgentSession(ctx, tenantID, sessionID, uc.sessions, uc.resolver)
	if err != nil {
		return err
	}
	if err := uc.agent.SendAgentInput(ctx, devServer, session.PtyID, []byte{0x03}); err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_AGENT_STOP_FAILED", "failed to send graceful interrupt to agent pty", err)
	}
	// No status write here — see BL-AG-02 step (d)-(e): the transition to
	// 'stopped' is driven by the agent's own agent.exited notification,
	// consumed by SOL-AG-05's classifier. Writing 'stopped' here would race
	// a graceful exit that takes a few hundred ms and briefly lie about
	// state (the same "persisted record must not lie" discipline
	// KillTerminalSession already applies, just in the opposite direction:
	// don't write a status the process hasn't actually reached yet).
	return nil
}

// KillAgentSession — force teardown. Marks the session row 'stopped' even
// if the agent call fails, same discipline as KillTerminalSession.Execute
// (kill_terminal_session.go:38-46).
type KillAgentSession struct {
	sessions      AgentSessionRepository
	resolver      ConnectionResolver
	agent         DevServerAgentClient
	writeActivity WriteActivityChecker // BR-AG-06, see below — nil-safe: a nil checker always allows the kill
	clock         func() time.Time
}

func (uc *KillAgentSession) Execute(ctx context.Context, sessionID, signal string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	_, devServer, session, err := resolveAgentSession(ctx, tenantID, sessionID, uc.sessions, uc.resolver)
	if err != nil {
		return err
	}
	if uc.writeActivity != nil {
		busy, err := uc.writeActivity.HasInFlightWrite(ctx, session.WorktreeID)
		if err == nil && busy {
			// BR-AG-06 — best-effort only, see "Open design question" below;
			// a checker error is NOT treated as "busy" (fail open, not
			// closed — an unreachable checker must never block a user's
			// explicit force-kill request).
			return apperrors.New(apperrors.KindFailedPrecondition, "INFRA_AGENT_KILL_BLOCKED_FILE_WRITE_IN_PROGRESS", "cannot kill agent while it is writing a file", nil)
		}
	}
	if signal == "" {
		signal = "SIGKILL"
	}
	agentErr := uc.agent.KillAgent(ctx, devServer, session.PtyID, signal)
	if err := uc.sessions.MarkStopped(ctx, tenantID, sessionID, uc.clock()); err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_MARK_AGENT_SESSION_STOPPED_FAILED", "failed to mark agent session stopped", err)
	}
	if agentErr != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_AGENT_KILL_FAILED", "agent session marked stopped, but the dev server agent failed to tear down the pty", agentErr)
	}
	return nil
}
```

### BR-AG-05/[A1] — 10s graceful→force escalation

BL-AG-02's `[A1]` puts the "did 10s pass without exit?" decision in the
**UI** ("Hiển thị dialog 'Force Kill?'"), not the server — the renderer
already owns a stop-then-offer-kill dialog pattern for the generic terminal.
This solution keeps that split: `StopAgentSession` fires the graceful
signal and returns immediately; a **client-driven** timer (renderer, same
as today) decides whether to call `KillAgentSession` at 10s. A server-side
`watch_agent_stop_escalation.go` usecase is proposed only as an optional
hardening (auto-escalate even if the renderer disconnects mid-wait), backed
by SOL-AG-05's `StreamPty` subscription watching for `agent.exited` with a
10s deadline — flagged as a nice-to-have, not required to close this bug,
since BL-AG-02's own flow puts the decision at the UI layer.

## Design — BR-AG-06 (write-lock check) — open design question, not a settled sketch

No write-lock concept exists anywhere in backend-go today (confirmed by
BUG-AG-02's own grep). This is a genuine gap the TDD docs don't specify a
mechanism for — `git-gateway-service.md`'s file-I/O design (SOL-009) is
stateless dispatch with no in-flight-operation tracking, and no other
service tracks per-worktree write activity. Two concrete options, presented
for a decision rather than picked unilaterally:

1. **Best-effort in-memory tracker in `git-gateway-service`.** A
   `WriteActivityTracker` increments/decrements an in-process counter keyed
   by `worktree_id` around every `WriteFile`/`WriteFileChunk` dispatch
   (SOL-009's proposed usecases), exposed via a new lightweight RPC
   `HasInFlightWrite(worktree_id) returns (bool)`. `KillAgentSession` calls
   it before killing. **Adds a new `infra --> git` edge** to
   `02-microservices-decomposition.md`'s dependency graph, alongside the
   existing `git --> infra` edge — not a request-cycle (neither call chain
   re-enters itself), but a two-way service relationship the current graph
   doesn't have, worth a deliberate look before adding. Best-effort by
   construction: a pod restart of `git-gateway-service` loses in-flight
   counts (fails open — never blocks a kill it can't prove is unsafe).
2. **Don't build it; rely on the 10s grace period + explicit user
   confirmation.** A force-kill already requires the user to explicitly
   override a failed graceful stop; most file writes are near-atomic OS
   `write()` calls, not multi-step transactions, so the corruption window
   SIGKILL introduces is small compared to, say, a database transaction.
   This avoids new cross-service coupling entirely at the cost of not
   literally satisfying BR-AG-06.

This solution's code sketch above accepts a `WriteActivityChecker` port so
option 1 slots in cleanly if chosen, but implements `KillAgentSession` to
degrade safely (nil-safe, fail-open) if the decision lands on option 2 or
is deferred — **recommend resolving this as a product decision before
implementation**, not inferring it from this solution alone.

## Test plan

- `usecase/stop_agent_session_test.go` — fake `DevServerAgentClient`:
  asserts `SendAgentInput` is called with exactly `{0x03}`, and that
  `SendSignal`/`KillPty` (the shell-PTY methods) are **never** called —
  regression guard against reusing the wrong RPC for an agent PTY.
- `usecase/kill_agent_session_test.go`:
  - agent call fails → session still marked stopped (mirrors
    `kill_terminal_session_test.go`'s "closes even on agent failure"
    coverage, ported to the agent-session equivalent).
  - `WriteActivityChecker` reports `busy=true` → `KillAgent` never called,
    `INFRA_AGENT_KILL_BLOCKED_FILE_WRITE_IN_PROGRESS` returned.
  - `WriteActivityChecker` returns an error → kill proceeds anyway (fail-open assertion).
  - `nil` checker → kill proceeds (BR-AG-06 not wired yet doesn't block basic kill).
- `adapter/devserveragent/agent_methods_test.go` (extends SOL-AG-01's file)
  — `KillAgent`/`SendAgentInput` send `agent.kill`/`agent.sendInput` with
  the exact param names `handleAgentKill`/`handleAgentSendInput` read.
- Regression test asserting `StopAgentSession`/`KillAgentSession` resolve a
  **different** `ptyId` lookup path than `StopTerminalProcess`/
  `KillTerminalSession` do — i.e. they go through `AgentSessionRepository`,
  never `TerminalSessionRepository` directly, since an agent session's
  `pty_id` is not guaranteed to have a live `terminal_sessions` row once
  routing metadata diverges (documented risk, worth a dedicated test).

## References

- `specs/backend-go/bugs/logic-v1/BUG-AG-02-dung-agent-partial.md`
- `docs/logic/agent-orchestration/BL-AG-02-dung-agent.md` — BR-AG-05/06/07/19, `[A1]` escalation flow
- `specs/backend-go/tdd/services/infra-fleet-service.md:525-573` (§10 "Known TS drift" — the exact class of bug this solution avoids)
- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md:110-166` (dependency graph — BR-AG-06's proposed `infra --> git` edge)
- `backend-go/services/infra-fleet-service/internal/usecase/stop_terminal_process.go:1-47`, `kill_terminal_session.go:1-47` — the shape this solution's usecases mirror, and the reason they can't be reused directly
- `agent/src/relay/agent-spawner.ts:88-96` (`PTY_REGISTRY`), `:540-621` (`handleAgentKill`, `handleAgentSendInput`)
- `agent/src/relay/pty-daemon-client.ts` — the separate PTY store `pty.destroy`/`pty.sendSignal` operate on (cited to show the two registries don't overlap)
- `specs/backend-go/bugs/logic-v1/solutions/SOL-AG-01-khoi-dong-agent.md` — `DevServerAgentClient.SendAgentInput`/`KillAgent`, `AgentSessionRepository` this solution builds on
