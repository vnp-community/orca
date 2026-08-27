# BUG-MB-04: No mobile-facing agent/worktree status endpoint exists — no aggregation, no truncation, no mobile transport

**Business Logic:** [BL-MB-04](../../../../docs/logic/mobile-companion/BL-MB-04-mobile-status.md) — Xem Trạng thái Agent từ Mobile
**Priority (per spec):** P1
**Status:** NOT_IMPLEMENTED
**Severity:** Medium
**Symptom:** Sam cannot open Orca Mobile and see a live list of all running agents/worktrees with status, duration, and last output. There is no mobile-reachable endpoint that returns this shape, no per-item output truncation, and no encrypted-transit path.

---

## Spec summary

Mobile requests current status and gets back a list of worktrees, each with `{id, name, agent, status, duration, lastOutput}`; pull-to-refresh and live WebSocket updates while foregrounded are both supported. Data must be encrypted in transit, `lastOutput` truncated to 500 chars, and a cached "last updated X ago" view shown when offline.

## What backend-go has

The closest existing capability is desktop-session-scoped, per-PTY, not an aggregated worktree-status view for mobile:

- `terminal.list` (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go:375-390`) calls `InfraFleetServiceClient.ListTerminalSessions` and returns `[]terminalSessionView` — a list of terminal *sessions*, not worktrees, with no `agent`/`status`/`duration`/`lastOutput` fields (checked `toTerminalSessionView`, `channels_terminal.go:153-172` — it maps id/pty fields only, no output snapshot).
- `terminal.agentStatus`/`terminal.isRunningAgent` (`channels_terminal.go:446-477`) return per-PTY `{AgentRunning, AgentKind, ReadyForInput}` — requires already knowing a specific `ptyId`, not a single call that lists everything.
- Both are `wscompat` WS channels gated by the same desktop JWT `Identity` as every other channel — reachable only by a client already authenticated exactly like the desktop app, not a distinct mobile-paired session (root cause: BUG-MB-01).

No code anywhere returns a `{worktrees: [...]}` shape:

```
$ grep -rln "worktrees:\|WorktreeStatusView\|ListWorktreeStatus" backend-go --include="*.go"
(only project.pb.go and worktree_repository.go — unrelated project/worktree-record CRUD, not agent runtime status)
```

## What's missing

- An aggregate "all worktrees + their agent status" query/RPC — nothing composes `terminal.list` + `terminal.agentStatus` + a "duration" computation + a "last output" snapshot into one response.
- `lastOutput` capture/truncation to 500 chars (BR-MB-15) — no code path snapshots or truncates PTY output for a status summary at all.
- A mobile-reachable transport for this data (root cause: BUG-MB-01 — no paired mobile session exists to serve this to).
- Encryption in transit (BR-MB-13) — same root cause, no shared secret exists.
- Any push/live-update path scoped to "app foreground" (BR-MB-14) is moot without a mobile transport in the first place; the general WS-push mechanism (`channels_push.go`) has no worktree-status-specific stream registered.
- Offline cached-status display (BR-MB-16) is a client-side concern, but even the server-side data it would cache doesn't exist yet.

## See also

- `specs/backend-go/bugs/logic-v1/BUG-MB-01-pair-device-not-implemented.md` — root cause of the missing mobile transport/encryption layer.
- `specs/backend-go/bugs/missing-v1/BUG-025-status-channels-not-implemented.md` — a different, unrelated `status.get` gap (browser-pane/Windows-terminal capability probe) in the same `wscompat` package; not a root-cause overlap with this worktree/agent status feature.

## References

- `docs/logic/mobile-companion/BL-MB-04-mobile-status.md` — full spec
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go:153-172,375-390,446-477` — `toTerminalSessionView`, `terminal.list`, `terminal.agentStatus`/`terminal.isRunningAgent`
- `backend-go/services/project-service/internal/adapter/postgres/worktree_repository.go` — worktree persistence (record CRUD only, no runtime agent status)
