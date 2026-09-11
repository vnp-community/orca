# SOL-AG-MOBILE-002: Mobile app device registration + dead-code retirement — zero `agent/` code change required

> **📐 Assessment-only — confirms no `agent/` code change is needed for
> [CR-MOBILE-002](../../../../../../docs/crs/v4/mobile-companion/CR-MOBILE-002-mobile-app-device-registration-and-dead-code-retirement.md).**
> The real work is `mobile/` (React Native), `desktop/` (Electron main
> dead-code retirement), and one `api-gateway` file — see
> [BE-MOBILE-SOL-002](../../../../../backend-go/crs/v4/mobile-companion/solutions/BE-MOBILE-SOL-002-mobile-subscribe-auth-endpoint.md).
> This document re-derives that `agent/` has no role in either half of
> CR-MOBILE-002 (push-token registration, or `MobileCompanionService.ts`
> retirement) from direct code reads, rather than assuming it from the CR's
> own scope table.

**CR:** [CR-MOBILE-002](../../../../../../docs/crs/v4/mobile-companion/CR-MOBILE-002-mobile-app-device-registration-and-dead-code-retirement.md)
**Depends on:** [SOL-AG-MOBILE-001](./SOL-AG-MOBILE-001-zero-agent-scope-notification-delivery.md) (same conclusion pattern, read first for the shared background)
**Affected files:** none

---

## 1. What CR-MOBILE-002 needs from `agent/`

CR-MOBILE-002 has two independent halves:

1. **Push-token registration** — `mobile/app.json`, `mobile/src/notifications/mobile-notifications.ts`, `mobile/src/transport/pairing.ts`, plus one `api-gateway` route for device-token auth (`notification_routes.go`).
2. **Dead-code retirement** — delete `desktop/src/main/mobile/MobileCompanionService.ts`, audit/retire its SQLite migration, drop the `web-push` npm dependency from `desktop/package.json`.

Neither half's "Changes Required" table lists any `agent/` file. The two
questions this document checks independently:

- Does the **auth mechanism** for CR-MOBILE-002 §B (a device needs to
  register a push token without a browser session cookie) route through
  `agent/` at any point?
- Is there an `agent/`-side equivalent of `MobileCompanionService.ts` that
  CR-MOBILE-002's `desktop/`-only dead-code audit would have missed?

## 2. Auth path — confirmed `desktop/`-only, no `agent/` hop

CR-MOBILE-002 §B's own text is explicit that whichever auth design is
chosen, `DeviceRegistry` — the thing that knows which `deviceToken` maps to
which `userId` — "sống trong tiến trình desktop, không phải
api-gateway/notification-service" (lives in the desktop Electron-main
process, not api-gateway/notification-service). Tracing this independently:

- The real `DeviceRegistry` is `desktop/src/main/runtime/device-registry.ts`
  (cited directly in CR-MOBILE-001 §0). It is a `desktop/`-process,
  in-memory + local-file (`orca-devices.json`) construct.
- `agent/src/main/runtime/mobile-pairing-files.ts` — the only
  mobile-pairing-named file that exists anywhere under `agent/` — is 7 lines
  declaring two filename constants (`DEVICE_REGISTRY_FILENAME`,
  `E2EE_KEYPAIR_FILENAME`) for a **userdata migration helper**, not a second
  `DeviceRegistry` implementation. It holds no pairing logic, no device list,
  no auth check.
- Whichever of CR-MOBILE-002 §B's two auth options is chosen (desktop relays
  the subscribe call on the mobile's behalf via the already-open paired RPC
  channel; or `mobile/` gets its own independent SSO/JWT), the request path
  is **mobile ↔ desktop ↔ api-gateway/notification-service**. The Dev Server
  Agent is a distinct process pool entirely (one instance per dev server the
  *desktop* connects out to for code execution) with no network path to or
  from a paired mobile device — confirmed by `specs/agent/tdd/v5/03-connection-modes.md`'s
  3 connection modes (`relay-ssh`, `relay-websocket`, `direct-websocket`),
  none of which originate from or terminate at a mobile client.

## 3. Dead-code retirement — no `agent/`-side `MobileCompanionService` equivalent

Grepping `agent/src/` for the desktop-side dead-code's own signatures
(`orca_push_subscriptions`, `INSERT OR REPLACE`, `web-push` npm import, a
`MobileCompanionService`-named class) returns zero hits. `agent/`'s
`package.json` has no `web-push` dependency to remove.

The one adjacent finding — `agent/src/main/persistence.ts` also carries an
orphaned `WebPushSubscription`/`getVapidKeys()` API from the same
"Phase 3 / TASK-033" effort `MobileCompanionService.ts` came from — is
already documented in
[SOL-AG-MOBILE-001 §4](./SOL-AG-MOBILE-001-zero-agent-scope-notification-delivery.md#4-incidental-finding-out-of-scope-for-cr-mobile-001)
and not repeated here to avoid two solutions independently proposing the
same future cleanup. It is unreachable from `agent/`'s real build
(`agent/build.mjs`'s sole entry point never imports it), so — like
`MobileCompanionService.ts` before this CR retires it — it is inert, not a
functional gap.

## 4. Conclusion

**`agent/` requires zero code changes for CR-MOBILE-002.** Both halves of
the CR (mobile push-token registration and `desktop/`'s dead-code
retirement) are fully contained in `mobile/`, `desktop/`, `frontend/`, and
`backend-go/api-gateway` — confirmed independently rather than assumed from
the CR's own scope table.

## Not in scope

- Any design work — this is a confirmation, not a solution to implement.
- Retiring `agent/src/main/persistence.ts`'s dead Web Push section — tracked
  once, in SOL-AG-MOBILE-001 §4, not duplicated here.
- Auditing whether `mobile/` needs an independent SSO/auth flow (CR-MOBILE-002's
  own "Quyết định kiến trúc cần chốt trước khi code" — a product decision,
  not an `agent/` code question).

## References

- [CR-MOBILE-002](../../../../../../docs/crs/v4/mobile-companion/CR-MOBILE-002-mobile-app-device-registration-and-dead-code-retirement.md)
- [SOL-AG-MOBILE-001](./SOL-AG-MOBILE-001-zero-agent-scope-notification-delivery.md) (shared background + the persistence.ts incidental finding)
- `desktop/src/main/runtime/device-registry.ts` (the real `DeviceRegistry` — confirmed `desktop/`-only)
- `agent/src/main/runtime/mobile-pairing-files.ts` (the only mobile-pairing-named file in `agent/` — filename constants only)
- `specs/agent/tdd/v5/03-connection-modes.md` (the agent's 3 connection modes — none touch a mobile client)
