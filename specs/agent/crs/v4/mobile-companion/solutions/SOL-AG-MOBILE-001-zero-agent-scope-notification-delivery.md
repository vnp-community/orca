# SOL-AG-MOBILE-001: `notification-service` native push delivery — zero `agent/` code change required

> **📐 Assessment-only — confirms no `agent/` code change is needed for
> [CR-MOBILE-001](../../../../../../docs/crs/v4/mobile-companion/CR-MOBILE-001-notification-service-native-push-delivery.md).**
> The entire fix is backend-go (`notification-service` + 1 `api-gateway` file)
> — see
> [BE-MOBILE-SOL-001](../../../../../backend-go/crs/v4/mobile-companion/solutions/BE-MOBILE-SOL-001-notification-service-push-delivery.md).
> This document exists so that "no `agent/` change" isn't taken on faith from
> the CR's own scope table — it re-derives the conclusion from
> `specs/agent/tdd/v5`'s architecture docs plus direct `agent/src/` reads,
> and surfaces one incidental finding the CR's own audit didn't check.

**CR:** [CR-MOBILE-001](../../../../../../docs/crs/v4/mobile-companion/CR-MOBILE-001-notification-service-native-push-delivery.md)
**Depends on:** nothing — reads current code only
**Affected files:** none

---

## 1. What CR-MOBILE-001 needs from `agent/`

CR-MOBILE-001's own "Changes Required" table lists 9 files — all under
`backend-go/proto/`, `backend-go/services/notification-service/`, and one
`backend-go/services/api-gateway/` file. None are under `agent/`. The open
question this document answers independently: **is that omission actually
correct, or did the CR's audit simply not look at `agent/`?**

## 2. Re-deriving the answer from the agent's own architecture docs

`specs/agent/tdd/v5/00-index.md`'s Addendum A.1 ("Vai trò trong hệ thống")
enumerates every role the Dev Server Agent plays: Execution Environment, Code
Host, File System Provider, Git Operations Host, AI Credential Store,
Workflow Step Executor, Health Reporter, AI Agent CLI Host, External API
Caller. Push notification delivery is not one of them, and no TDD chapter
(`01-architecture.md` through `13-external-api-connectors.md`) mentions
`notification-service`, APNs, FCM, or Web Push as an agent responsibility —
the only "push notification" hits in the whole `specs/agent/tdd/` tree
(`07-jsonrpc-dispatch.md` §9, `03-connection-modes.md` §5,
`11-fs-handler-extension.md`) are about the **Agent WebSocket Protocol's own
server-push frames** (`pty.data`, `pty.exit`, `fs.changed` — one-way frames
the agent sends unsolicited over its existing `wss://backend:6768/agent`
connection, per TDD-AG-02/03). This is a same-named but structurally
unrelated concept to CR-MOBILE-001's APNs/FCM mobile push — no code, port, or
protocol is shared between them.

The event chain CR-MOBILE-001 wires up is: an agent action completes (e.g. a
task-graph step) → the agent reports it up through the Agent WebSocket
Protocol (`agent.statusChanged`, `StreamExecOutput`, etc., per TDD-AG-07) to
whichever backend-go service owns that domain (`task-service`,
`workflow-service`, `orchestration-service`, `automation-service`) → **that
service**, not the agent, publishes the NATS domain event
(`orca.task.task.completed`, etc.) that `notification-service`'s
`HandleIncomingEvent` consumes (CR-MOBILE-001 §2's `subjectRules`). The agent
has no NATS client, no knowledge of `notification-service`'s existence, and
no reason to acquire either — it reports outcomes one hop up its existing
wire protocol and stops there.

## 3. Direct verification — zero references in `agent/src/`

Confirmed by grep across `agent/src/**/*.ts` (excluding tests), with no
hits for any of:

- `notification-service` (as a string/import) — 0
- `apns`, `fcm` (as identifiers, not substrings of unrelated words) — 0
- `expo.push`, `push_subscri`, `push-subscri`, `PushSubscription` — 0
- `DeviceRegistry`, `dispatchMobileNotification`, `createPairingOffer` (the
  desktop-side pairing/notify symbols CR-MOBILE-001 §0 cites) — 0

**Conclusion: `agent/` requires zero code changes for CR-MOBILE-001.** The
CR's own scope table is correct, and this holds independent of whichever
architectural decision (Vault Transit vs. `credential-broker-service`) the
backend-go side lands on for APNs/FCM credential storage — that choice has
no `agent/`-visible surface either way.

## 4. Incidental finding, out of scope for CR-MOBILE-001

While verifying §3, `agent/src/main/persistence.ts` (6,659 lines) turned up
with a `getWebPushSubscriptions()`/`setWebPushSubscriptions()`/
`getVapidKeys()` API and a `// ─── Web Push Persistence (Phase 3 — TASK-033)
─────` section (lines ~5280-5300), using a `WebPushSubscription` type from
`agent/src/shared/types.ts`. This looks, at first read, like exactly the kind
of dead mobile-push code CR-MOBILE-002 found in `desktop/src/main/mobile/MobileCompanionService.ts`
— but it is **not reachable from `agent/`'s real build**:

- `agent/build.mjs`'s `entryPoints` is `[AGENT_ENTRY]` only
  (`agent/src/relay/agent-entry.ts`).
- `agent-entry.ts`'s import graph (`agent-config`, `agent-logger`,
  `agent-tool-registry`, `agent-connection-direct/relay/stdio`) never reaches
  `agent-hooks/server.ts` or any other file that could transitively import
  `main/persistence.ts`.
- No file under `agent/src/relay/` imports `main/persistence.ts` at all —
  confirmed by grep for the import path directly (0 hits); the handful of
  `agent/src/relay/*` files that matched a loose grep for the word
  "persistence" only use it in prose comments ("no persistence needed here"),
  not as an import.
- The identical file also exists at `backend/src/main/persistence.ts` and
  `desktop/src/main/persistence.ts` — `desktop/`'s copy is the real, used
  Electron-main settings store (it imports `electron`'s `app`/`safeStorage`,
  which `agent/`'s relay binary has no reason to ever call). `agent/`'s copy
  reads as a leftover whole-file duplicate from before the `backend/` →
  `desktop/` + `agent/` + `frontend/` workspace split, not intentionally
  ported code.

This is the same *class* of finding as `MobileCompanionService.ts`
(pre-existing, orphaned, superseded-by-`notification-service` Web Push code)
but living in `agent/` instead of `desktop/`, and it was not part of
CR-MOBILE-002's audit (which only looked at `desktop/`). It requires no
action for CR-MOBILE-001 or CR-MOBILE-002 — nothing calls it, so nothing
breaks by leaving it — but is flagged here for whoever eventually writes a
cleanup CR for this class of dead code, so the search isn't repeated from
scratch. **Not converted into a task in this solution set**, per the
"assessment, not a design" scope of this document.

## Not in scope

- Any design work — this is a confirmation, not a solution to implement.
- Deleting `agent/src/main/persistence.ts`'s dead Web Push section (§4) —
  needs its own cleanup CR if the product wants `agent/`'s orphaned files
  retired (mirrors `desktop/`'s `MobileCompanionService.ts` retirement in
  CR-MOBILE-002, but that CR's "Changes Required" list is `desktop/`-scoped
  and doesn't cover this file).

## References

- [CR-MOBILE-001](../../../../../../docs/crs/v4/mobile-companion/CR-MOBILE-001-notification-service-native-push-delivery.md)
- [docs/crs/v4/mobile-companion/README.md](../../../../../../docs/crs/v4/mobile-companion/README.md) — original conclusion table ("agent/ có logic mobile thật không? Không, và không cần có")
- `specs/agent/tdd/v5/00-index.md` Addendum A.1, A.12 (role table; feature→component mapping has no F03 row)
- `specs/agent/tdd/v5/02-wire-protocol.md`, `03-connection-modes.md` §5, `07-jsonrpc-dispatch.md` §9 (the agent's own "push notification" concept — server-push wire frames, unrelated to APNs/FCM)
- `agent/src/main/persistence.ts:5280-5300`, `agent/src/shared/types.ts:3896-3903` (incidental finding, §4)
- `agent/build.mjs`, `agent/src/relay/agent-entry.ts` (confirms `persistence.ts` is unreachable from the real build)
