# BUG-TM-04: No OSC 133 shell-integration support — no bootstrap injection, no per-command tracking

**Business Logic:** [BL-TM-04](../../../../docs/logic/terminal-management/BL-TM-04-shell-integration.md) — Shell Integration — OSC 133 Command Tracking
**Priority (per spec):** P1
**Status:** NOT_IMPLEMENTED
**Severity:** Low
**Symptom:** A user running PowerShell against a backend-go-hosted terminal never sees command markers, exit codes, or execution timing, and cannot jump between prior commands — PowerShell never emits OSC 133 on its own and backend-go has no mechanism to inject the bootstrap script the spec calls for (BR-TM-14). More generally, backend-go has zero awareness of OSC 133 at all: it only knows when the whole shell *process* exits (`terminal.exited`/`PtyExited`), never when an individual *command* inside that shell starts or finishes.

---

## Spec summary

Shells emit OSC 133 A/B/C/D escape sequences around each command (prompt start, input-start, output-start, command-finished-with-exit-code). The UI uses these to show ✅/❌ exit codes, execution time, and to let the user jump between commands. It's opt-in (BR-TM-13); PowerShell needs a bootstrap script injected at shell-launch time to emit OSC 133 at all, since it doesn't do so natively (BR-TM-14); and OSC sequences must be stripped before copy-to-clipboard (BR-TM-15).

## What backend-go has

- `SpawnTerminalSessionRequest` (`backend-go/proto/orca/infrafleet/v1/infrafleet.proto`, `SpawnTerminalSessionRequest` message) carries a plain `shell` string field, forwarded as-is to the Dev Server Agent (`backend-go/services/infra-fleet-service/internal/usecase/spawn_terminal_session.go:69`, `uc.agent.SpawnPty(ctx, devServer, SpawnPtyInput{Cwd: in.Cwd, Shell: in.Shell, ...})`) — no bootstrap-script parameter or injection logic of any kind.
- PTY output is relayed as opaque bytes end-to-end: `terminal.output` (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go:258-259`) and the `terminal.multiplex` binary `Output` opcode (`channels_terminal_multiplex.go:222-230`) both forward `PtyOutput.Data` verbatim with no scanning for escape sequences.
- The only "command finished" signal backend-go tracks is the whole-process exit: `PtyExited`/`WaitTerminalSessionResponse.exit_code` (`infra-fleet-service/internal/usecase/wait_terminal_session.go:26,80`; `channels_terminal.go:399-417`) — this fires once, when the shell itself terminates, not once per command run inside it.

## What's missing

- No OSC 133 detection/parsing anywhere in backend-go: `grep -rni "osc133\|osc 133"` across `backend-go/` returns zero matches.
- No bootstrap-script injection for PowerShell (or any shell) at spawn time — `SpawnTerminalSessionRequest`/`SpawnPtyInput` have no field for it, and the usecase does no shell-specific setup (BR-TM-14 entirely unimplemented).
- No opt-in flag or per-session shell-integration toggle (BR-TM-13) — there is nothing to opt into.
- No per-command exit-code/timing state anywhere server-side — only the aggregate PTY-process exit code is tracked (`WaitTerminalSession`), so no RPC or wscompat channel can answer "did the last command in this still-open shell succeed" or "how long did it take."
- No command-boundary index for "jump to previous/next command" — nothing records where OSC 133 A/C/D markers occurred in the output stream.
- OSC-strip-before-copy (BR-TM-15) is inherently a client-side clipboard concern and not something backend-go would implement regardless — noted for completeness, not counted as a backend gap.

## See also

None — this is a distinct gap from BUG-029 (`missing-v1/BUG-029-terminal-channels-not-implemented.md`), which is now stale for the base `terminal.*` create/send/resize/close/wait flow (all now implemented, see `channels_terminal.go`) but never covered OSC 133 either way.

## References

- `docs/logic/terminal-management/BL-TM-04-shell-integration.md`
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` — `SpawnTerminalSessionRequest` (no bootstrap-script field), `PtyOutput`/`PtyExited` messages (no per-command shape)
- `backend-go/services/infra-fleet-service/internal/usecase/spawn_terminal_session.go:69` — shell forwarded verbatim, no bootstrap injection
- `backend-go/services/infra-fleet-service/internal/usecase/wait_terminal_session.go:26,80` — only whole-process exit tracked
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go:258-274` — raw byte passthrough, no escape-sequence awareness
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal_multiplex.go:222-240` — same, binary-opcode path
