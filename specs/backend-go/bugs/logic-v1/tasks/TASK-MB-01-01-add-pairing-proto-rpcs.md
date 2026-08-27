# TASK-MB-01-01: Add device-pairing RPCs to `auth.proto`

**From Solution:** SOL-MB-01
**Priority:** P0 — everything else in this solution depends on generated stubs from this
**Service:** `auth-service`
**File:** `backend-go/proto/orca/auth/v1/auth.proto`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

SOL-MB-01 adds a mobile QR-pairing + TweetNaCl E2E handshake flow to
`auth-service`. None of the pairing RPCs/messages exist in `auth.proto`
yet — this task adds them, additive-only so `buf breaking` stays clean.

## Changes to make

Add to the `AuthService` service block in `auth.proto` (after the existing
admin RPCs):

```protobuf
// ── Mobile device pairing (BL-MB-01) ──────────────────────────────────
// InitiateDevicePairing requires an authenticated caller (browser session
// or an already-issued desktop JWT) — "my account wants to pair a new
// device," never anonymous.
rpc InitiateDevicePairing(InitiateDevicePairingRequest) returns (InitiateDevicePairingResponse);
// CompleteDevicePairing is the ONE unauthenticated RPC on this service's
// entire surface — guarded solely by pairing_token possession +
// one-time-use + 5-minute expiry. api-gateway routes it through a
// dedicated, rate-limited, unauthenticated REST path (TASK-MB-01-08).
rpc CompleteDevicePairing(CompleteDevicePairingRequest) returns (CompleteDevicePairingResponse);
rpc ListPairedDevices(ListPairedDevicesRequest) returns (ListPairedDevicesResponse);
rpc UnpairDevice(UnpairDeviceRequest) returns (google.protobuf.Empty);

// Internal-only (mesh ingress, never routed by api-gateway's REST facade)
// — the ONE path any other service uses to obtain a device's raw shared
// secret for E2E encrypt/decrypt. Mirrors credential-broker-service's
// "mediator hands back decrypted material on demand, never persists it"
// pattern.
rpc ResolveDeviceSharedSecret(ResolveDeviceSharedSecretRequest) returns (ResolveDeviceSharedSecretResponse);
```

Add messages (append to the bottom of the file):

```protobuf
message InitiateDevicePairingRequest {
  // tenant_id/user_id come from gRPC metadata (identity propagation, per
  // 08-inter-service-communication.md's gRPC conventions) — never a field.
}
message InitiateDevicePairingResponse {
  string pairing_token = 1;          // opaque, high-entropy; hashed at rest (mirrors sessions.id)
  bytes  desktop_public_key = 2;     // this pairing session's ephemeral X25519 public key
  string server_address = 3;         // api-gateway's public base URL (server-mode adaptation, see SOL-MB-01 rationale)
  int64  expires_at_unix_ms = 4;     // BR-MB-01: now + 5 minutes
}

message CompleteDevicePairingRequest {
  string pairing_token = 1;
  bytes  mobile_public_key = 2;      // mobile's own ephemeral X25519 public key
  string device_label = 3;           // e.g. "Sam's iPhone" — user-facing, not trusted for anything security-relevant
}
message CompleteDevicePairingResponse {
  string device_id = 1;
  bytes  desktop_public_key_confirmation = 2; // echoes InitiateDevicePairingResponse.desktop_public_key
  string access_token = 3;  // BR-MB-01: JWT issued immediately after a successful handshake
  string refresh_token = 4;
}

message ListPairedDevicesRequest {}
message ListPairedDevicesResponse { repeated PairedDevice devices = 1; }
message PairedDevice {
  string id = 1;
  string device_label = 2;
  int64  paired_at_unix_ms = 3;
  int64  last_used_at_unix_ms = 4;
  string status = 5; // "active" | "revoked"
}

message UnpairDeviceRequest { string device_id = 1; }

// Internal-only — never exposed over the REST facade.
message ResolveDeviceSharedSecretRequest { string device_id = 1; }
message ResolveDeviceSharedSecretResponse {
  bytes shared_secret = 1; // raw 32-byte NaCl box shared secret; caller must not persist it
}
```

## Regenerate stubs

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./proto/...
```

Expected: clean build, `buf breaking` reports no breaking changes (only additions).
