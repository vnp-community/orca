# BUG-AG-05: Raw PTY output reaches the renderer, but nothing classifies it into agent status — no idle/running/waiting/completed/error, no persistence, no mobile push

**Business Logic:** [BL-AG-05](../../../../docs/logic/agent-orchestration/BL-AG-05-monitor-status.md) — Monitor Trạng thái Agent Real-time
**Priority (per spec):** P0
**Status:** PARTIAL
**Severity:** Critical
**Symptom:** The agent card in the sidebar has no backend-go source of truth for `idle`/`running`/`waiting`/`completed`/`error`/`stopped` — backend-go streams raw terminal bytes to the renderer but never parses OSC 133 sequences or output text into a status, never persists status for restart-survival, and never pushes to a mobile app. Anyone building the mobile/desktop status indicator against backend-go today gets bytes, not state.

---

## Spec summary

PTY output streamed continuously from the Dev Server over WS is parsed by Orca (`AgentHookParser`) for OSC 133 A/B/D markers (command start/output/finish → `running`/`idle`/`error`) and text patterns (`waiting for input` → `waiting`, rate-limit patterns → `agent:rateLimited`, `task completed` → `completed`). Status changes emit `agent:statusChanged`, pushed to the Renderer via IPC and to a paired Mobile App via a TweetNaCl-encrypted WebSocket channel, and must be persisted so status survives a server restart (BR-AG-15). Target latency: <500ms detect-to-update, <5s to mobile.

## What backend-go has

- **Raw output transport is real**: `terminal.create`'s stream handler (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go:189-282`) relays `AttachPty`'s server-stream frames straight through as `PushEvent{Channel: "terminal.output", ...}` (channels_terminal.go:259) and `PushEvent{Channel: "terminal.exited", ...}` (channels_terminal.go:266) — unparsed bytes and a raw exit code, nothing else.
- **A best-effort "is an agent running" signal exists, but it's not the OSC/status pipeline the spec describes**: `GetTerminalAgentStatus`/`AgentStatus` (`backend-go/services/infra-fleet-service/internal/usecase/get_terminal_agent_status.go:22-48`, `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/methods.go:183-211`) answers "is a CLI named claude/codex the foreground process in this pane" by calling `pty.listProcesses` and matching the process title substring (methods.go:172-221) — a poll-based heuristic over process metadata, not output-stream parsing, and its own doc comment (methods.go:183-199) says `ReadyForInput` is hard-set equal to `AgentRunning` because "no busy/idle signal exists anywhere in the agent's pty.* RPC surface to distinguish actively-streaming from idle-at-a-prompt" — i.e. the exact `running` vs `idle` distinction BL-AG-05 requires is explicitly flagged as unbuildable with today's agent-side RPC surface.
- Reachable via wscompat `terminal.agentStatus`/`terminal.isRunningAgent` (`channels_terminal.go:446-488`).

## What's missing

- No OSC 133 A/B/D sequence parser anywhere in backend-go (grep for `OSC`/`133` across all of `backend-go/` returns only unrelated SCM rate-limit-header code — see BUG-AG-04 — and zero PTY-output-parsing hits).
- No status enum matching the spec's six states (`idle`/`running`/`waiting`/`completed`/`error`/`stopped`) tied to agent sessions — the only "idle"/"completed" enum values found in backend-go belong to `orchestration-service`'s unrelated coordinator-run/task-status domain (`backend-go/services/orchestration-service/internal/domain/orchestration.go:28,134,269,271`) and `workflow-service`'s execution status, not PTY/agent lifecycle.
- No text-pattern matching for `"waiting for input"` / `"task completed"` on streamed PTY output.
- No `agent:statusChanged { sessionId, status, detail }` event or equivalent push channel.
- No persistence of agent status for restart-survival (BR-AG-15) — no such table exists (see BUG-AG-01/03's confirmed absence of any `orca_session`/`agent_session` table).
- No mobile push path at all: grepping backend-go for `tweetnacl`/`nacl.` returns zero hits, and `notification-service`'s usecases (`get_vapid_public_key.go`, `handle_incoming_event.go`, `subscribe.go`, `unregister_push_subscription.go`) are web-push (VAPID) plumbing for a different feature, with no caller wiring agent-status changes into them.
- No measurable <500ms detect-to-update or <5s mobile-delivery SLO, since no status-detection pipeline exists to measure.

## See also

- specs/backend-go/bugs/logic-v1/BUG-AG-04-switch-account-partial.md — rate-limit detection is the same missing PTY-output-parsing gap, called out separately there.
- specs/backend-go/bugs/logic-v1/BUG-AG-01-khoi-dong-agent-partial.md / BUG-AG-03 — the missing session-persistence layer this status pipeline would need to write into.

## References

- `docs/logic/agent-orchestration/BL-AG-05-monitor-status.md`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go:188-282,446-488`
- `backend-go/services/infra-fleet-service/internal/usecase/get_terminal_agent_status.go:22-48`
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/methods.go:172-221`
- `backend-go/services/orchestration-service/internal/domain/orchestration.go:28,134,269,271` — unrelated status enum, cited to rule it out
- `backend-go/services/notification-service/internal/usecase/` — unrelated web-push plumbing, cited to rule it out
