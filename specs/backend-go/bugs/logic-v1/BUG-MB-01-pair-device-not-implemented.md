# BUG-MB-01: Mobile device pairing (QR + E2E key exchange) does not exist in backend-go

**Business Logic:** [BL-MB-01](../../../../docs/logic/mobile-companion/BL-MB-01-pair-device.md) — Pair Mobile Device với Desktop App
**Priority (per spec):** P0
**Status:** NOT_IMPLEMENTED
**Severity:** Critical
**Symptom:** There is no way for Sam to pair Orca Mobile with the desktop app at all — no QR-code generation, no pairing token, no key exchange endpoint anywhere in backend-go. Any mobile client trying to follow BL-MB-01's flow has nothing server-side to call.

---

## Spec summary

Desktop generates an ephemeral TweetNaCl keypair, a one-time pairing token, and encodes both (plus the local server address) into a QR code that expires after 5 minutes. Mobile scans the QR, sends a pairing request with the token, desktop verifies it, both sides exchange public keys and derive a shared secret (NaCl box) for all subsequent encrypted traffic. Business rules cap devices at 3 per desktop, enforce one-time token use, and require unpair to wipe the shared secret.

## What backend-go has

Nothing. A repo-wide search confirms zero hits for any pairing-related concept:

```
$ grep -rliE "pairing|qrcode|qr_code|tweetnacl|nacl\.|device_pair|shared_secret|e2e" backend-go --include="*.go"
(no matches, aside from unrelated `_e2e_test.go` end-to-end test filenames)
```

There is no `PairDevice`/`GeneratePairingToken`/`VerifyPairingToken` RPC in any `.proto` file (checked `notification.proto`, `gitgateway.proto`, `scmintegration.proto` — the only files where "pair" appears at all, and only as a substring of unrelated identifiers). No service owns a `mobile_devices` or `paired_devices` table (`backend-go/services/*/internal/adapter/postgres/*.go` — no such repository exists). `notification-service`'s only device-facing concept is `PushSubscription` (`backend-go/services/notification-service/internal/domain/push_subscription.go:19-20` — `ChannelIOS`/`ChannelAndroid` enum values), which is a Web-Push-style APNs/FCM device-token registration, not a paired E2E-encrypted session — it carries no keypair, no shared secret, no token expiry/one-time-use semantics, and is never actually sent to (see BUG-MB-02).

## What's missing

- QR-code generation endpoint (ephemeral keypair + one-time token + server address encoding)
- Pairing-token issuance, 5-minute expiry, and one-time-use invalidation (BR-MB-01, BR-MB-02)
- A pairing-request endpoint for mobile to submit its public key + token
- Server-side key exchange / shared-secret derivation (NaCl box or equivalent)
- A `paired_devices` (or similar) table and the 3-devices-per-desktop cap (BR-MB-03)
- An unpair operation that deletes the shared secret (BR-MB-04)

Because none of this exists, BL-MB-02/03/04 — which all assume an already-paired, E2E-encrypted mobile session — have no authenticated mobile identity or encryption channel to build on.

## See also

No existing `missing-v1`/`api-v1` bug documents this gap directly — those audits are scoped to the legacy frontend's `callRuntimeRpc` catalog, which has no mobile-pairing call sites since Orca Mobile is a separate client not covered by that inventory.

## References

- `docs/logic/mobile-companion/BL-MB-01-pair-device.md` — full spec
- `backend-go/services/notification-service/internal/domain/push_subscription.go:19-26` — closest existing "device" concept (Web Push subscription, not a paired E2E session)
- `backend-go/proto/orca/notification/v1/notification.proto` — no pairing RPCs
