# BUG-SSH-03: Real exponential-backoff auto-reconnect exists, but only for relay-websocket mode — the SSH (relay-ssh) mode this BL is about has none, and its remote process dies with the connection by design

**Business Logic:** [BL-SSH-03](../../../../docs/logic/remote-development/BL-SSH-03-auto-reconnect.md) — SSH Auto-Reconnect với Agent Continuity
**Priority (per spec):** P1 (Must Have)
**Status:** PARTIAL
**Severity:** High
**Symptom:** When a Carlos-style remote-dev user's SSH connection to a relay-ssh-mode dev server drops, the relay/agent process on the remote host is killed along with it — there is no background reconnect loop, no output buffering, and no "the agent kept running while you were offline" continuity for this connection mode. The only real auto-reconnect-with-backoff implementation in backend-go applies to a *different* connection mode (relay-websocket, where Orca dials the agent's own long-lived WS server), not to the SSH-based flow BL-SSH-01/02/03 describe.

---

## Spec summary

BL-SSH-03 requires detecting a dropped SSH connection, showing a "Reconnecting..." overlay, buffering user input, retrying with exponential backoff (1s, 2s, 4s, ... capped at 30s, unlimited attempts per BR-SSH-12), and on reconnect: checking whether the remote relay is still alive (reattach) or restarting it (BL-SSH-02) if it died, flushing up to 10MB of buffered agent output (BR-SSH-11) accrued while offline, and resuming input — all transparent to the user (BR-SSH-14), with the remote agent process required to keep running across the local disconnect (BR-SSH-10) and a way for the user to cancel reconnect (BR-SSH-13).

## What backend-go has

- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/session.go:414-473` — `backgroundReconnect` is a real exponential-backoff retry loop: `backoffDelay` (`session.go:560-566`) computes `ReconnectBaseDelay * 2^attempt` capped at `ReconnectMaxDelay`, with jitter, matching the spec's backoff shape conceptually (defaults `2s`/`60s` per `devserveragent/config.go:56-71`, vs. the spec's `1s`/`30s` — same pattern, different constants).
- This loop is exercised by a real test: `session_test.go:181-233`, `TestSession_BackgroundReconnect_RecoversAfterDropWithoutCallerRetry`.
- `devserveragent/client.go:81-88` — `getOrDialSession` (relay-websocket) reuses this backgrounded reconnect automatically.

## What's missing

- **The one mode this BL is actually about (relay-ssh) has no background reconnect at all.** `session.go:69-74`'s own doc comment: `managedExternally` "marks a session this package doesn't own re-establishing on its own — direct-websocket's inbound accept ... and relay-ssh's active provision (a fresh deploy+launch, not a reconnect). `backgroundReconnect` no-ops for both." `client.go:179-200`'s `getOrProvisionSession` only lazily re-provisions **on the next call that needs the session** — there is no proactive drop-detection, no background retry loop, and no exponential backoff for relay-ssh.
- **BR-SSH-10 ("agent process trên remote PHẢI tiếp tục chạy khi local disconnect") is directly contradicted by the current design**, not merely unimplemented: `sshrelay/provisioner.go:14-21`'s package doc comment states the scope is "one exec channel per session, foreground, no detach/nohup/Unix-socket-reattach ... a dropped SSH connection just ends the session; the next call re-provisions from scratch." Because `launch()` (`launch.go:18-45`) starts `node agent.js --stdio` as the foreground child of the SSH exec session (`session.Start(cmd)`, `launch.go:39`), killing the SSH connection kills the remote agent process — the opposite of "agent continuity."
- **No output buffering (BR-SSH-11, 10MB cap)** — no buffer/ring-buffer of agent output exists anywhere in `infra-fleet-service` for the offline window; `grep` for buffer/ring-buffer/replay in this service only turns up the unrelated `pty.replay` PTY-scrollback notification (`session.go:294`, `client.go:293`), which is not an SSH-drop continuity buffer.
- **No unlimited-attempts guarantee for relay-ssh (BR-SSH-12)** — moot since there is no retry loop for this mode at all.
- **No user-facing "cancel reconnect" surface (BR-SSH-13)** — no cancellation API exists for either mode's reconnect path.
- **No "Reconnecting..." state signal for the frontend to render an overlay** — `ssh.getState` (`get_ssh_state.go:35-50`) reports only binary `Connected`/not; there is no `"reconnecting"` intermediate state or event pushed anywhere.

## See also

None found — no missing-v1/api-v1 bug covers reconnect/continuity specifically.

## References

- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/session.go:60-88,414-473,560-566`
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/session_test.go:181-233`
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go:81-88,179-200`
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/config.go:50-71`
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/provisioner.go:1-21,82-115`
- `backend-go/services/infra-fleet-service/internal/adapter/sshrelay/launch.go:18-45`
- `backend-go/services/infra-fleet-service/internal/usecase/get_ssh_state.go:35-50`
- `docs/logic/remote-development/BL-SSH-03-auto-reconnect.md`
