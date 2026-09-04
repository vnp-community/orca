# specs/emulator — Remaining Work

TASK-EMU-001 through TASK-EMU-013 are all `✅ DONE` (see
[README.md](./README.md) and
[bugs/missing-v1/00-overview.md](./bugs/missing-v1/00-overview.md)). What's
left are open items from
[CR-DS-009](../../docs/crs/v2/dev-server/CR-DS-009-mobile-emulator-agent-separation.md)'s
Acceptance Criteria (§6) and a couple of gaps noted along the way — tracked
here since they don't map to a completed TASK-EMU yet.

## CR-DS-009 remaining

| # | Item | Why not done yet | Priority |
|---|---|---|---|
| 1 | **End-to-end verify** with `infra-fleet-service` running via docker-compose (register agent → pair → call `emulator.*` → get a real response) | No live DB/backend-go stack in this sandbox to test against | High — the only way to confirm Phase 1-5 actually fit together |
| 2 | Backend-go's `devServer.list`/`listForUser`/`add` (wscompat `channels.go`) don't read/return the `kind` field | `devServerView` struct has no `Kind` yet; the `kind` filter in web/server mode is currently a safe no-op (doesn't filter, doesn't error) | Medium — needs its own backend-go task to thread the field through the view struct |
| 3 | Real **iOS** device control (`device.tap/gesture/button/rotate` for simulator) — still honest-stub `-32601` | Needs a real macOS + Xcode/`simctl` machine to port and verify | Medium |
| 4 | Confirm `emulator/` builds an **independent binary**, with no dependency on `agent/`'s git/fs/pty code | Code is architecturally split per TDD-EMU-03, but no dedicated build-isolation verification step has run | Low — likely already true, just unverified |
| 5 | Fix any reconnect/handshake bug in `packages/dev-agent-transport/` in one place, confirm it applies to both `agent/` and `emulator/` without a second fix | This is a "when it happens" acceptance criterion — no concrete bug to fix yet, just confirms the shared-package architecture holds up | Low |
| 6 | Confirm one project with `devServerId` (Linux remote) and `mobileEmulatorAgentId` (personal Mac) pointing to different hosts works independently, and disconnecting one doesn't affect the other | Needs a real multi-host environment — blocked on #1 | Medium |
| 7 | Confirm Mobile Emulator Agent registration uses the **same token/approval mechanism** as Dev Server Agent (CR-DS-006), no separate approval flow | Likely already true since `AddDevServerDialog` is shared, but not explicitly tested through the approval flow | Low |
| 8 | `MobileEmulatorAgentSetupGuide`/`use-mobile-emulator-agent-setup-state.ts` (setup guide for an **AI agent** driving the emulator via Orca CLI — a different feature from the Mobile Emulator Agent process) not updated to key off `mobileEmulatorAgentId` | Deliberately out of scope for Phase 5 — different feature, not agent registration | Low |

## Noticed but out of scope (not mine to touch)

- `frontend/src/renderer/src/components/project/LinkedProjectsManager.tsx` /
  `__tests__/LinkedProjectsManager.test.tsx` have uncommitted changes of
  unclear origin in the working tree — unrelated to Mobile Emulator Agent,
  left untouched.

## Suggested next step

Item #1 (end-to-end verify) is the only thing that actually blocks CR-DS-009
from being "confirmed working end to end" — everything else is polish or an
edge case gated on a real multi-host/macOS environment.
