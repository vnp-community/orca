# BUG-TM-03: Terminal scrollback buffer is never persisted or restored

**Business Logic:** [BL-TM-03](../../../../docs/logic/terminal-management/BL-TM-03-scrollback-persistence.md) — Lưu và Khôi phục Scrollback Buffer
**Priority (per spec):** P1
**Status:** NOT_IMPLEMENTED
**Severity:** Medium
**Symptom:** When a user closes the app/worktree and reopens it later, every terminal pane comes back completely empty — none of the prior session's command output, scrollback history, cursor position, or text attributes survive, because backend-go never wrote any of it down in the first place.

---

## Spec summary

On close (app close, worktree close, or idle timeout), the terminal's scrollback buffer should be serialized (via `@xterm/addon-serialize`), gzip-compressed, and written to storage keyed by `worktree_id` along with cursor position and a timestamp. On reopening a worktree, the snapshot should be loaded, decompressed, deserialized, and used to restore the terminal's prior output, cursor position, and attributes. Rules: only serialize while idle (BR-TM-09), cap snapshot size at 50MB/worktree (BR-TM-10), restore cursor/attributes exactly (BR-TM-11), and expire snapshots after 30 days of the worktree not being opened (BR-TM-12).

## What backend-go has

- `infra.terminal_sessions` (`backend-go/services/infra-fleet-service/migrations/0005_terminal_sessions.up.sql:14-22`) persists only session *metadata* — `pty_id`, `tenant_id`, `connection_id`, `cwd`, `created_at`, `last_active_at`, `closed_at`. No column for serialized buffer content, cursor position, or size.
- `backend-go/services/infra-fleet-service/internal/domain/terminal_session.go:12-25` — the `TerminalSession` domain struct mirrors that same metadata-only shape.
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal_multiplex.go:14-28` explicitly documents the gap: the real frontend/TS protocol's `SnapshotRequest`/`Start`/`Chunk`/`End` opcodes (on-demand scrollback snapshot delivery) are "accepted (never corrupt the connection) but are documented no-ops — a real client falls back to full redraws instead of flow control/snapshot recovery."
- `terminal.wait`/`WaitTerminalSession` (`channels_terminal.go:392-418`) tracks only whether/how the whole PTY process exited — unrelated to scrollback content.

## What's missing

- No serialize/compress step anywhere: `grep -rni "scrollback\|serialize.*xterm\|gzip"` across `backend-go/services/` finds no scrollback-buffer capture logic (the two `serialize` hits that exist are about JWT/dispatch-context serialization, unrelated).
- No persistence table or column for buffer bytes, cursor position, or per-worktree snapshot size — `infra.terminal_sessions` has none of these fields, and no other migration in the repo defines one.
- No RPC/wscompat channel to push a snapshot on close or pull one on reopen — `terminal.multiplex`'s `SnapshotRequest` opcode is a documented no-op (see above); there is no equivalent unary RPC in `infrafleet.proto` either (`grep -n "Snapshot"` on the proto returns nothing terminal-related).
- No idle-detection gate (BR-TM-09), no 50MB cap enforcement (BR-TM-10), no cursor/attribute-exact restore (BR-TM-11), and no 30-day expiry sweep (BR-TM-12) — none of these can exist without the underlying storage first.
- No trigger wiring on worktree-close/app-close to capture a snapshot — `git-gateway-service`'s `RemoveWorktree` (`server.go:697-702`) and `project-service`'s worktree lifecycle usecases have no call into any terminal-snapshot capture path.

## See also

- No direct prior missing-v1/api-v1 bug covers this — BUG-029 (`missing-v1/BUG-029-terminal-channels-not-implemented.md`) predates the current `terminal.*`/`AttachPty` implementation and is now stale for the create/send/resize/close flow, but it never addressed scrollback persistence either way.

## References

- `docs/logic/terminal-management/BL-TM-03-scrollback-persistence.md`
- `backend-go/services/infra-fleet-service/migrations/0005_terminal_sessions.up.sql:14-22` — terminal session table, metadata only
- `backend-go/services/infra-fleet-service/internal/domain/terminal_session.go:12-25` — domain struct, metadata only
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal_multiplex.go:14-28` — `SnapshotRequest`/`Chunk`/`End` documented as no-ops
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` — no Snapshot/scrollback RPC or message defined
- `backend-go/services/git-gateway-service/internal/adapter/grpc/server.go:697-702` — `RemoveWorktree`, no snapshot-capture hook
