# SOL-MB-01: Implement mobile device pairing (QR + TweetNaCl E2E key exchange)

**Resolves:** [BUG-MB-01](../BUG-MB-01-pair-device-not-implemented.md)
**Service:** `auth-service` (owns pairing + post-pairing JWT issuance) +
`api-gateway` (REST facade + rate limiting for the unauthenticated
completion step)
**Affected files (proposed):**
- `backend-go/proto/orca/auth/v1/auth.proto` (extend — new pairing RPC group)
- `backend-go/services/auth-service/internal/domain/{paired_device,pairing_session}.go`
- `backend-go/services/auth-service/internal/usecase/{initiate_device_pairing,complete_device_pairing,list_paired_devices,unpair_device,resolve_device_shared_secret}.go`
- `backend-go/services/auth-service/internal/usecase/ports.go` (extend — `DeviceKeyExchanger`, `SharedSecretSealer`)
- `backend-go/services/auth-service/internal/adapter/nacl/` (new — TweetNaCl box wrapper)
- `backend-go/services/auth-service/internal/adapter/postgres/` (new tables: `paired_devices`, `pairing_sessions`)
- `backend-go/services/auth-service/internal/adapter/grpc-client/credentialbroker/` (extend — Transit `encrypt`/`decrypt` for shared secrets)
- `backend-go/services/api-gateway/internal/adapter/http/` (REST routes: authenticated initiate, unauthenticated complete — rate-limited)
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

### Where this belongs: `auth-service`, not a new service or `notification-service`

`07-security-architecture.md`'s AuthN table states the mobile client's
credential is a "Short-lived JWT (RS256) + refresh token, obtained after the
existing QR-pairing + TweetNaCl E2E handshake" — `auth-service` issues it,
`api-gateway` validates it against `auth-service`'s JWKS
(`07-security-architecture.md:8`). This sentence *presupposes* pairing
happens, without saying which service performs it — the genuine gap
BUG-MB-01 found. `auth-service.md` §1 already states this service "owns...
**Sessions** — browser cookie sessions and mobile/CLI JWT issuance, refresh,
and revocation" (`auth-service.md:22`) and its RPC surface already has
`IssueToken(sessionToken | refreshToken)` that "mints a short-lived RS256
JWT for mobile/CLI... use, plus a refresh token" (`auth-service.md:79-80`).
A successfully paired device is exactly the precondition `IssueToken`
already exists to serve — pairing produces the credential auth-service was
always going to turn into a JWT, so extending this service (not
`notification-service`, whose closest concept — `PushSubscription` — is a
device-*token* registration with no keypair/shared-secret concept at all
per BUG-MB-01's own finding) keeps one service owning "who is this device"
end to end, the same "data here, logic there" boundary `auth-service.md` §1
already draws for users. Standing up an 18th service for pairing alone
would duplicate `auth-service`'s existing JWT-minting/Vault-Transit-signing
machinery (§6, §9) for no isolation benefit, the same reasoning SOL-009 used
to fold `files.*` into `git-gateway-service` rather than a new service.

### A genuine architecture adaptation this solution must flag

BL-MB-01's flow describes "Desktop" generating the QR (ephemeral keypair +
one-time token + **local server address, IP:port**) for Mobile to scan
directly. In the Go backend's **server-mode** target (the only mode this
TDD set scopes — `api-gateway.md` §10: "Electron desktop mode is out of
scope... `api-gateway` is exclusively the server-mode multi-user
deployment's edge"), there is no per-user local IP:port process to point
at — every client, including a soon-to-be-paired mobile device, only ever
reaches the system through `api-gateway`'s single external listener
(`api-gateway.md` §1: "the only service with an external-facing
listener"). This solution adapts BL-MB-01's "server address" field to be
**`api-gateway`'s configured public base URL** (a static value from
`ServiceRegistry`/config, `api-gateway.md` §4, never derived from the
request) rather than a literal local socket address, and "Desktop" becomes
"the currently browser-authenticated user's session" rather than a
distinct OS process. This is an explicit extension beyond what any TDD doc
states outright — flagged the same way SOL-009 flagged file I/O as a scope
addition to `git-gateway-service.md`.

### Crypto: TweetNaCl box, Go-side via `golang.org/x/crypto/nacl/box`

BL-MB-01's security model (`docs/logic/mobile-companion/BL-MB-01-pair-device.md:43-56`)
is literally NaCl `box`: X25519 key agreement + XSalsa20-Poly1305 AEAD. Go's
standard `golang.org/x/crypto/nacl/box` is the same primitive family
TweetNaCl implements (both are the NaCl `crypto_box` construction), so the
mobile client's TweetNaCl.js `box.before`/`box(open)` interoperates with
this package directly — no protocol redesign, just a same-primitive
implementation on each side, consistent with `07-security-architecture.md`'s
framing of pairing as "the existing... TweetNaCl E2E handshake" (i.e.
already a decided primitive, not an open question this solution reopens).

### Shared-secret storage follows the "never a plaintext key column" pattern already established twice

`notification-service.md` §9 and `infra-fleet-service.md` §9 both apply the
same rule to different secrets (VAPID private key, SSH credentials): never a
plaintext value in this service's own Postgres row, mediated through
Vault/`credential-broker-service` instead. This solution applies the
identical pattern to each paired device's derived shared secret — extending
that precedent to a new secret *class* (an E2E symmetric secret keyed per
device-pairing, not a system-wide signing key) is this solution's own
proposal, not something either doc already covers; flagged as an extension
in the design below, not an assumption.

---

## Design — proto (`orca.auth.v1`, extending `auth-service.md` §3's RPC list)

```protobuf
service AuthService {
  // ... existing session/JWT/admin RPCs unchanged ...

  // ── Mobile device pairing (BL-MB-01) ──────────────────────────────────
  // InitiateDevicePairing requires an authenticated caller (browser
  // session or an already-issued desktop JWT) — this is "my account wants
  // to pair a new device," never anonymous.
  rpc InitiateDevicePairing(InitiateDevicePairingRequest) returns (InitiateDevicePairingResponse);
  // CompleteDevicePairing is the ONE unauthenticated RPC on this service's
  // entire surface (per api-gateway.md's REST facade — routed through a
  // dedicated, heavily rate-limited path, see wiring section). Guarded
  // solely by pairing_token possession + one-time-use + 5-minute expiry.
  rpc CompleteDevicePairing(CompleteDevicePairingRequest) returns (CompleteDevicePairingResponse);
  rpc ListPairedDevices(ListPairedDevicesRequest) returns (ListPairedDevicesResponse);
  rpc UnpairDevice(UnpairDeviceRequest) returns (google.protobuf.Empty);

  // Internal-only (mesh ingress, never routed by api-gateway's REST facade)
  // — the ONE path any other service uses to obtain a device's raw shared
  // secret for E2E encrypt/decrypt. Mirrors credential-broker-service's
  // "mediator hands back decrypted material on demand, never persists it"
  // pattern (auth-service.md §2's own analogy point for secret material).
  rpc ResolveDeviceSharedSecret(ResolveDeviceSharedSecretRequest) returns (ResolveDeviceSharedSecretResponse);
}

message InitiateDevicePairingRequest {
  // tenant_id/user_id come from gRPC metadata (identity propagation, per
  // 08-inter-service-communication.md's gRPC conventions) — never a field.
}
message InitiateDevicePairingResponse {
  string pairing_token = 1;          // opaque, high-entropy; hashed at rest (mirrors sessions.id, auth-service.md:176)
  bytes  desktop_public_key = 2;     // this pairing session's ephemeral X25519 public key
  string server_address = 3;         // api-gateway's public base URL — see "architecture adaptation" above
  int64  expires_at_unix_ms = 4;     // BR-MB-01: now + 5 minutes
}

message CompleteDevicePairingRequest {
  string pairing_token = 1;
  bytes  mobile_public_key = 2;      // mobile's own ephemeral X25519 public key
  string device_label = 3;           // e.g. "Sam's iPhone" — user-facing, not trusted for anything security-relevant
}
message CompleteDevicePairingResponse {
  string device_id = 1;
  bytes  desktop_public_key_confirmation = 2; // echoes InitiateDevicePairingResponse.desktop_public_key so mobile can verify it paired the device it scanned
  // Per 07-security-architecture.md's AuthN table: JWT is issued
  // immediately after a successful handshake — CompleteDevicePairing
  // internally calls the same issuance path IssueToken uses, so mobile
  // leaves this call already authenticated, not forced into a second
  // round trip.
  string access_token = 3;
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

## Design — domain

```go
// internal/domain/pairing_session.go
// PairingSession is the ephemeral server-side state of one in-progress
// QR pairing attempt — BR-MB-01 (5-minute expiry) and BR-MB-02 (one-time
// use) are both invariants of this type, not ad hoc checks scattered
// across the usecase.
type PairingSession struct {
    ID                     string // == pairing_token, hashed at rest (mirrors auth-service.md:176's sessions.id precedent)
    TenantID, UserID       string
    DesktopPublicKey       []byte
    DesktopPrivateKeyCiphertext []byte // Vault Transit-encrypted — see usecase note below
    VaultKeyRef            string
    CreatedAt, ExpiresAt   time.Time
    ConsumedAt             *time.Time // BR-MB-02: non-nil once CompleteDevicePairing has consumed it
}

func (s PairingSession) Expired(now time.Time) bool { return now.After(s.ExpiresAt) }
func (s PairingSession) Consumed() bool             { return s.ConsumedAt != nil }

// internal/domain/paired_device.go
// PairedDevice is a durably paired mobile device — BR-MB-03 (max 3 per
// account) is enforced by the usecase counting active rows before insert,
// not a domain invariant here (needs a repository query, not just the
// struct's own fields).
type PairedDevice struct {
    ID, TenantID, UserID    string
    DeviceLabel             string
    SharedSecretCiphertext  []byte // Vault Transit-encrypted 32-byte NaCl box shared secret — never plaintext in this row
    VaultKeyRef             string
    Status                  DeviceStatus // "active" | "revoked"
    PairedAt, LastUsedAt    time.Time
    RevokedAt                *time.Time
}

var (
    ErrPairingTokenNotFound = errors.New("domain: pairing token not found")
    ErrPairingTokenExpired  = errors.New("domain: pairing token expired")   // BR-MB-01
    ErrPairingTokenConsumed = errors.New("domain: pairing token already used") // BR-MB-02
    ErrDeviceLimitReached   = errors.New("domain: device pairing limit reached") // BR-MB-03
    ErrDeviceNotFound       = errors.New("domain: paired device not found")
)

const MaxPairedDevicesPerAccount = 3 // BR-MB-03
```

## Design — data model (Postgres, `auth` schema — extends `auth-service.md` §5)

```sql
CREATE TABLE auth.pairing_sessions (
    id                       TEXT PRIMARY KEY,          -- hash of the pairing token, per sessions.id's precedent
    tenant_id                UUID NOT NULL,
    user_id                  UUID NOT NULL REFERENCES auth.users(id),
    desktop_public_key       BYTEA NOT NULL,
    desktop_private_key_ciphertext BYTEA NOT NULL,       -- Vault Transit-encrypted; decrypted once, in CompleteDevicePairing, then the row is deleted
    vault_key_ref            TEXT NOT NULL,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at                TIMESTAMPTZ NOT NULL,       -- BR-MB-01
    consumed_at               TIMESTAMPTZ                 -- BR-MB-02
);
CREATE INDEX idx_pairing_sessions_expires_at ON auth.pairing_sessions(expires_at); -- reaper job, mirrors sessions/refresh_tokens (auth-service.md §8)

CREATE TABLE auth.paired_devices (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                UUID NOT NULL,
    user_id                  UUID NOT NULL REFERENCES auth.users(id),
    device_label             TEXT,
    shared_secret_ciphertext BYTEA NOT NULL,              -- Vault Transit-encrypted; never plaintext here
    vault_key_ref            TEXT NOT NULL,
    status                   TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','revoked')),
    paired_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at             TIMESTAMPTZ,
    revoked_at               TIMESTAMPTZ
);
CREATE INDEX idx_paired_devices_user_active ON auth.paired_devices(tenant_id, user_id) WHERE status = 'active'; -- backs BR-MB-03's count check
```

## Design — usecase layer (key flows)

```go
// InitiateDevicePairing — requires an authenticated caller (tenant.RequireTenantID +
// an authenticated user_id from context, same as every other auth-service RPC).
func (uc *InitiateDevicePairing) Execute(ctx context.Context) (PairingResult, error) {
    tenantID, userID := ...
    pub, priv, err := uc.keyExchanger.GenerateEphemeralKeypair() // adapter/nacl, box.GenerateKey
    ciphertext, vaultKeyRef, err := uc.sealer.Encrypt(ctx, priv) // credential-broker-service Transit encrypt — priv never written to disk unencrypted
    token := uc.tokens.NewHighEntropyToken()
    session := domain.PairingSession{
        ID: hash(token), TenantID: tenantID, UserID: userID,
        DesktopPublicKey: pub, DesktopPrivateKeyCiphertext: ciphertext, VaultKeyRef: vaultKeyRef,
        CreatedAt: now, ExpiresAt: now.Add(5 * time.Minute), // BR-MB-01
    }
    uc.sessions.Save(ctx, session)
    return PairingResult{PairingToken: token, DesktopPublicKey: pub, ServerAddress: uc.serverAddress, ExpiresAt: session.ExpiresAt}, nil
}

// CompleteDevicePairing — the one unauthenticated path. Atomic
// select-and-consume closes the one-time-use race (two concurrent
// CompleteDevicePairing calls racing the same token).
func (uc *CompleteDevicePairing) Execute(ctx context.Context, token string, mobilePub []byte, label string) (CompleteResult, error) {
    session, err := uc.sessions.GetAndConsume(ctx, hash(token)) // single SQL stmt: UPDATE ... SET consumed_at=now() WHERE id=$1 AND consumed_at IS NULL RETURNING *
    if errors.Is(err, domain.ErrPairingTokenNotFound) { return CompleteResult{}, err }
    if session.Expired(now) { return CompleteResult{}, domain.ErrPairingTokenExpired } // BR-MB-01

    active, err := uc.devices.CountActive(ctx, session.TenantID, session.UserID)
    if active >= domain.MaxPairedDevicesPerAccount { return CompleteResult{}, domain.ErrDeviceLimitReached } // BR-MB-03

    priv, err := uc.sealer.Decrypt(ctx, session.DesktopPrivateKeyCiphertext, session.VaultKeyRef)
    shared, err := uc.keyExchanger.SharedSecret(priv, mobilePub) // box.Precompute — the desktop-side half of BL-MB-01's diagram
    zero(priv) // never persisted past this call

    secretCiphertext, vaultKeyRef, err := uc.sealer.Encrypt(ctx, shared)
    device := domain.PairedDevice{ID: uuid.New(), TenantID: session.TenantID, UserID: session.UserID,
        DeviceLabel: label, SharedSecretCiphertext: secretCiphertext, VaultKeyRef: vaultKeyRef, Status: domain.DeviceActive, PairedAt: now}
    uc.devices.Save(ctx, device)

    access, refresh, err := uc.issueToken.ExecuteForDevice(ctx, session.UserID, device.ID) // reuses IssueToken's existing signing path (auth-service.md §6's TokenSigner/Vault Transit)
    return CompleteResult{DeviceID: device.ID, DesktopPublicKeyConfirmation: session.DesktopPublicKey, AccessToken: access, RefreshToken: refresh}, nil
}

// UnpairDevice — BR-MB-04: wipes the shared secret, not just a status flag,
// so ResolveDeviceSharedSecret can never again return it even from a stale
// cache. Delete (not soft-revoke-with-secret-intact) is the enforcement
// mechanism, not a housekeeping choice.
func (uc *UnpairDevice) Execute(ctx context.Context, deviceID string) error {
    return uc.devices.RevokeAndWipeSecret(ctx, deviceID) // UPDATE ... SET status='revoked', shared_secret_ciphertext=NULL, vault_key_ref=NULL, revoked_at=now()
}

// ResolveDeviceSharedSecret — internal-only; the row's ciphertext being NULL
// (post-unpair) is itself the enforcement point BR-MB-04 requires: an error
// here, not a stale secret.
func (uc *ResolveDeviceSharedSecret) Execute(ctx context.Context, deviceID string) ([]byte, error) {
    device, err := uc.devices.Get(ctx, deviceID)
    if device.Status != domain.DeviceActive || device.SharedSecretCiphertext == nil {
        return nil, domain.ErrDeviceNotFound
    }
    device.LastUsedAt = now; uc.devices.Touch(ctx, device.ID, now)
    return uc.sealer.Decrypt(ctx, device.SharedSecretCiphertext, device.VaultKeyRef)
}
```

`DeviceKeyExchanger`/`SharedSecretSealer` join `ports.go`'s existing
`PasswordHasher`/`TokenSigner`/`PolicyDataPublisher` list
(`auth-service.md:190-195`) — `SharedSecretSealer` is implemented against
`credential-broker-service`'s Transit `encrypt`/`decrypt` data operations
(a new call this solution adds to that client package, alongside the
existing `sign` calls other services already make), never against Vault
directly, per `auth-service.md` §6's "no `adapter/vault/`... only path is
`credential-broker-service`" rule *as already applied to JWT signing* —
extended here to a second Vault Transit operation (encrypt/decrypt) on the
same mediated path.

## Design — wiring (REST via `api-gateway`)

```
POST /v1/users/me/paired-devices/pairing-sessions   -> InitiateDevicePairing (authenticated: session cookie or desktop JWT)
POST /v1/paired-devices/pairing-sessions/{token}/complete -> CompleteDevicePairing (UNAUTHENTICATED)
GET  /v1/users/me/paired-devices                    -> ListPairedDevices (authenticated)
DELETE /v1/users/me/paired-devices/{device_id}       -> UnpairDevice (authenticated)
```

`CompleteDevicePairing`'s route is the one REST endpoint on the entire
gateway surface that bypasses `api-gateway.md` §9's JWT/session validation
step by design (there is no session yet — that is the point of pairing) —
flagged explicitly since it is the sole intentional exception to "every
request validated before routing," not an oversight. It must still pass
through every *other* edge control §9 lists: rate limiting (tightened
specifically for this route — a brute-force pairing-token guesser is the
realistic threat model BR-MB-01/02's expiry+one-time-use already partially
defend against, defense in depth), request size limits, and WAF-style
sanitization. `ServiceRegistry` (`api-gateway.md` §4) marks this one route
`AuthRequired: false` explicitly rather than it falling out of a default.

## Test plan

- `pairing_session_test.go` / `paired_device_test.go` (domain) — `Expired`,
  `Consumed` invariants; `NewWorktree`-style constructor validation.
- `initiate_device_pairing_test.go` — fake `DeviceKeyExchanger`/`SharedSecretSealer`:
  asserts a fresh keypair per call, `expires_at` = now+5m exactly.
- `complete_device_pairing_test.go`:
  - Happy path: shared secret derivation matches (assert `box.Precompute`
    symmetry against a mobile-side fixture keypair — the desktop-derived
    and mobile-derived shared secrets must be byte-identical).
  - Expired token → `ErrPairingTokenExpired`, no device row inserted.
  - Already-consumed token → `ErrPairingTokenConsumed`; **concurrency test**:
    two goroutines racing `GetAndConsume` on the same token — exactly one
    succeeds (regression guard for BR-MB-02's one-time-use race).
  - 4th device for an account with 3 active → `ErrDeviceLimitReached`
    (BR-MB-03), and no row inserted (assert fake repository unchanged).
  - Successful pairing returns a real `access_token`/`refresh_token` —
    assert `IssueToken`'s underlying signer was actually invoked (BR-MB-01's
    "obtained after" ordering guarantee).
- `unpair_device_test.go` — asserts `shared_secret_ciphertext`/`vault_key_ref`
  are nulled (BR-MB-04), and a subsequent `ResolveDeviceSharedSecret` call
  against the same device returns `ErrDeviceNotFound`, not a stale secret.
- `resolve_device_shared_secret_test.go` — revoked device → error, never a
  decrypt attempt (assert fake `SharedSecretSealer.Decrypt` not called).
- `adapter/nacl` — round-trip test against a real `golang.org/x/crypto/nacl/box`
  fixture pair generated the way a TweetNaCl.js client would, confirming
  wire compatibility (base64/raw-byte encoding matches what BL-MB-01's QR
  payload format expects).
- `api-gateway` REST contract test — `CompleteDevicePairing`'s route
  reachable with **no** `Authorization` header/session cookie and still
  succeeds with a valid token; a request with a garbage token gets a
  generic `NOT_FOUND`-shaped error (no oracle distinguishing "expired" vs.
  "wrong token" vs. "already consumed" in the HTTP response, to avoid
  leaking pairing-session existence to an unauthenticated prober — a
  security refinement beyond the bare business rule).

## References

- `specs/backend-go/bugs/logic-v1/BUG-MB-01-pair-device-not-implemented.md` — problem statement
- `docs/logic/mobile-companion/BL-MB-01-pair-device.md:22-56` — flow, security model, BR-MB-01..04
- `specs/backend-go/tdd/services/auth-service.md:19-24` (Sessions ownership), `:64-88` (§3 session/JWT RPCs, `IssueToken`), `:176-182` (§5 hashed-token-at-rest precedent), `:190-206` (§6 `TokenSigner`/Vault-Transit-mediation pattern this solution extends to `SharedSecretSealer`)
- `specs/backend-go/tdd/architecture/07-security-architecture.md:8` (mobile JWT "obtained after... QR-pairing + TweetNaCl E2E handshake", the sentence this solution implements)
- `specs/backend-go/tdd/services/api-gateway.md:1-25` (single external listener), `:284-315` (§9 edge controls, session/JWT validation), `:317-357` (§10, Electron-desktop-mode out of scope — grounds the "server address" adaptation)
- `specs/backend-go/tdd/services/notification-service.md:128-134,278-306` (§9's Vault-mediated-secret pattern this solution's shared-secret storage mirrors)
- `specs/backend-go/bugs/missing-v1/solutions/SOL-009-files-channels.md` — precedent for flagging a TDD scope addition explicitly rather than silently assuming it
