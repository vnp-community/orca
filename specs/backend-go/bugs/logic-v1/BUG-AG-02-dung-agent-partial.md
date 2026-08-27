# BUG-AG-02: Stop-agent has generic PTY stop/kill primitives but no agent-session lifecycle around them

**Business Logic:** [BL-AG-02](../../../../docs/logic/agent-orchestration/BL-AG-02-dung-agent.md) — Dừng Agent
**Priority (per spec):** P0
**Status:** PARTIAL
**Severity:** High
**Symptom:** Clicking "Stop" on an agent card has no backend-go endpoint that (a) knows there is an "agent" (as opposed to a bare PTY) running, (b) records a stopped session for later resume, or (c) refuses to kill while the agent is mid-write. What exists is a transport-layer SIGINT/kill for any PTY, indistinguishable from stopping a plain shell.

---

## Spec summary

Stopping an agent sends `\x03` (Ctrl+C) over the same Dev Server WS connection for a graceful exit; on `agent.exit`, `orca_sessions.status` is set to `stopped` and the session id retained for resume. If the agent doesn't exit within 10s, the UI offers force-kill (`agent.kill` with `SIGKILL`), which also clears the Dev Server's PTY session store. A write-lock check (BR-AG-06) must block killing while the agent is writing a file.

## What backend-go has

- **Graceful interrupt primitive**: `StopTerminalProcess` (`backend-go/services/infra-fleet-service/internal/usecase/stop_terminal_process.go:20-47`) calls `DevServerAgentClient.SendSignal(..., "SIGINT")`, which relays JSON-RPC `pty.sendSignal` (`backend-go/services/infra-fleet-service/internal/adapter/devserveragent/methods.go:112-130`) — a real signal, not the old Ctrl+C-byte-over-write workaround the doc comment explicitly says it replaces. Reachable via wscompat `terminal.stop` (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go:357-374`).
- **Force-kill primitive**: `KillTerminalSession` (`backend-go/services/infra-fleet-service/internal/usecase/kill_terminal_session.go:17-47`) calls `DevServerAgentClient.KillPty(..., graceful=true)` → JSON-RPC `pty.destroy`, and always marks the session row closed even if the agent call fails. `SendSignal` also independently supports `SIGKILL` in its `allowedSignals` set (`methods.go:108-110`). Reachable via wscompat `terminal.close` (`channels_terminal.go:337-356`).
- Both are scoped to `TerminalSessionRepository`'s generic `terminal_sessions` table (`backend-go/services/infra-fleet-service/internal/adapter/postgres/terminal_session_repository.go`), not an agent-specific session concept.

## What's missing

- No `orca_sessions` (or equivalent) status transition to `stopped` tied to an agent-exit event — there is no agent-session table at all (see BUG-AG-01 and BUG-AG-03 for the same absence).
- No linkage from "stopped PTY" to "session id retained for resume" (BR-AG-07) — since there is no session id concept distinct from `pty_id`/`connection_id`, nothing survives a stop in a form BL-AG-03's resume flow could consume.
- No write-lock check before kill (BR-AG-06) — `KillTerminalSession.Execute` (kill_terminal_session.go:27-47) unconditionally proceeds to `agent.KillPty` + `sessions.Close`, no precondition check.
- No 10-second graceful-timeout escalation logic server-side — `StopTerminalProcess` only sends SIGINT once; whether/when to escalate to `terminal.close`/SIGKILL is left entirely to a caller that doesn't exist in backend-go (no orchestrating usecase ties the two calls together with a timer).
- No `agent.exit { ptyId, code }` event distinguishing a graceful vs forced exit outcome for the UI to react to — `PtyExited` (`backend-go/proto/orca/infrafleet/v1/infrafleet.proto:362`) just carries an exit code from the generic `AttachPty` stream, with no "was this a stop or a kill" semantics attached.

## See also

- specs/backend-go/bugs/logic-v1/BUG-AG-01-khoi-dong-agent-partial.md — same missing agent-session abstraction underlies both start and stop.
- specs/backend-go/bugs/logic-v1/BUG-AG-03-resume-session-not-implemented.md — the "session id retained for resume" postcondition this bug can't satisfy.

## References

- `docs/logic/agent-orchestration/BL-AG-02-dung-agent.md`
- `backend-go/services/infra-fleet-service/internal/usecase/stop_terminal_process.go:20-47`
- `backend-go/services/infra-fleet-service/internal/usecase/kill_terminal_session.go:17-47`
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/methods.go:94-130`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go:337-374`
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:50-51,316-317,362`
