# BUG-MB-03: Remote dispatch from mobile does not exist — no mobile-reachable endpoint, and the underlying PTY-input primitive enforces none of the spec's business rules

**Business Logic:** [BL-MB-03](../../../../docs/logic/mobile-companion/BL-MB-03-remote-dispatch.md) — Remote Dispatch — Gửi Prompt từ Mobile về Agent
**Priority (per spec):** P1
**Status:** NOT_IMPLEMENTED
**Severity:** High
**Symptom:** Sam cannot send a follow-up prompt from Orca Mobile to a desktop agent at all. There is no mobile-authenticated channel to reach the desktop (BUG-MB-01), and even the desktop-only PTY-input mechanism this flow would have to reuse has no state-gating, queueing, validation, or confirmation logic — it just writes raw bytes to the PTY unconditionally.

---

## Spec summary

Sam sends a follow-up prompt from the mobile app; it's encrypted and sent to desktop, which decrypts, validates, and injects it into the agent's PTY. Dispatch is only allowed when the agent is idle/waiting; a dispatch to a running agent gets queued instead of dropped or rejected; prompts are capped at 10,000 chars; overwriting an already-queued prompt requires confirmation.

## What backend-go has

The only PTY-input-injection primitive in backend-go is `terminal.send`, a `wscompat` WS channel meant for the desktop's own authenticated session (JWT-based `Identity`, not a paired mobile device):

```go
// backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go:283-304
func registerTerminalSendChannel(r *Registry) {
	r.Register("terminal.send", func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[terminalSendArgs](args, 0)
		...
		entry, ok := streams.get(in.PtyID)
		...
		if err := entry.send(&infrafleetv1.PtyClientFrame{
			Frame: &infrafleetv1.PtyClientFrame_Input{Input: &infrafleetv1.PtyInput{Data: []byte(in.Data)}},
		}); err != nil {
			return nil, err
		}
		return nil, nil
	})
}
```

It writes `in.Data` straight into the PTY's input stream with no other logic. Agent status is separately queryable via `terminal.agentStatus`/`terminal.isRunningAgent` (`channels_terminal.go:446-477`), but `terminal.send` never calls or checks it.

## What's missing

- **No mobile-reachable endpoint at all.** `terminal.send` lives behind the same `wscompat` desktop WS session as every other `terminal.*` channel; there is no separate mobile-paired transport for it to arrive over (root cause: BUG-MB-01 — no pairing means no mobile identity/decrypt step exists to sit in front of this call).
- **No decrypt/validate step (steps 6 of the spec's flow).** `terminal.send`'s handler does no decryption (there's no shared secret to decrypt with) and no payload validation beyond JSON decoding.
- **No idle/waiting gate (BR-MB-09).** `registerTerminalSendChannel` never calls `GetTerminalAgentStatus`/checks `AgentRunning`/`ReadyForInput` before writing — a dispatch is accepted regardless of agent state.
- **No queueing for a running agent (BR-MB-10).** There is no queue data structure or deferred-delivery mechanism anywhere in `channels_terminal.go` or `infra-fleet-service`'s PTY usecases (`internal/usecase/attach_pty.go`) — input is written immediately or not at all, never held for later.
- **No prompt-length validation (BR-MB-11).** No 10,000-char (or any) cap is enforced in `terminalSendArgs`/`registerTerminalSendChannel`.
- **No overwrite-confirmation flow (BR-MB-12).** There is no concept of an already-queued prompt to overwrite, since no queue exists.

## See also

- `specs/backend-go/bugs/logic-v1/BUG-MB-01-pair-device-not-implemented.md` — root cause of the missing mobile transport/encryption layer this flow depends on.
- `specs/backend-go/bugs/missing-v1/BUG-025-status-channels-not-implemented.md` — unrelated `status.get` gap, cited here only because it touches the same `wscompat` package; not a root-cause overlap.

## References

- `docs/logic/mobile-companion/BL-MB-03-remote-dispatch.md` — full spec
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go:283-304` — `registerTerminalSendChannel`/`terminal.send`, unconditional PTY write
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go:446-477` — `terminal.agentStatus`/`terminal.isRunningAgent`, never consulted by `terminal.send`
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/methods.go:191,210` — `ReadyForInput` cannot distinguish "waiting" from "running" even if `terminal.send` did check it
