# TASK-MB-01-05: Implement `InitiateDevicePairing`/`CompleteDevicePairing` usecases

**From Solution:** SOL-MB-01
**Priority:** P0
**Service:** `auth-service`
**File:** `backend-go/services/auth-service/internal/usecase/initiate_device_pairing.go`, `backend-go/services/auth-service/internal/usecase/complete_device_pairing.go`
**Depends on:** TASK-MB-01-02, TASK-MB-01-03, TASK-MB-01-04
**Status:** `[ ]` TODO

---

## Context

These two usecases implement BR-MB-01 (5-minute expiry), BR-MB-02
(one-time-use, closed by the repository's atomic `GetAndConsume`), and
BR-MB-03 (max 3 active paired devices). `CompleteDevicePairing` is
`auth-service`'s only unauthenticated usecase — it must not leak whether a
token is "expired" vs. "wrong" vs. "already consumed" in a way a caller can
distinguish (see TASK-MB-01-08's REST wiring note).

## Changes to make

`backend-go/services/auth-service/internal/usecase/initiate_device_pairing.go`:

```go
package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

const pairingSessionTTL = 5 * time.Minute // BR-MB-01

type InitiateDevicePairing struct {
	sessions      PairingSessionRepository
	keyExchanger  DeviceKeyExchanger
	sealer        SharedSecretSealer
	tokens        TokenGenerator // reuse existing high-entropy token generator port, if present; else add one
	serverAddress string          // api-gateway's public base URL, from config
}

func NewInitiateDevicePairing(sessions PairingSessionRepository, keyExchanger DeviceKeyExchanger, sealer SharedSecretSealer, tokens TokenGenerator, serverAddress string) *InitiateDevicePairing {
	return &InitiateDevicePairing{sessions: sessions, keyExchanger: keyExchanger, sealer: sealer, tokens: tokens, serverAddress: serverAddress}
}

type PairingResult struct {
	PairingToken     string
	DesktopPublicKey []byte
	ServerAddress    string
	ExpiresAt        time.Time
}

func (uc *InitiateDevicePairing) Execute(ctx context.Context) (PairingResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return PairingResult{}, apperrors.New(apperrors.KindUnauthenticated, "AUTH_NO_TENANT", "no tenant in request context", err)
	}
	userID := tenant.UserIDFromContext(ctx) // adjust to this repo's actual identity-from-context accessor

	pub, priv, err := uc.keyExchanger.GenerateEphemeralKeypair()
	if err != nil {
		return PairingResult{}, apperrors.New(apperrors.KindInternal, "AUTH_PAIRING_KEYGEN_FAILED", "failed to generate pairing keypair", err)
	}
	ciphertext, keyRef, err := uc.sealer.Encrypt(ctx, priv)
	if err != nil {
		return PairingResult{}, apperrors.New(apperrors.KindInternal, "AUTH_PAIRING_SEAL_FAILED", "failed to seal ephemeral private key", err)
	}
	token := uc.tokens.NewHighEntropyToken()
	now := time.Now()
	session := domain.PairingSession{
		ID: hashToken(token), TenantID: tenantID, UserID: userID,
		DesktopPublicKey: pub, DesktopPrivateKeyCiphertext: ciphertext, VaultKeyRef: keyRef,
		CreatedAt: now, ExpiresAt: now.Add(pairingSessionTTL),
	}
	if err := uc.sessions.Save(ctx, session); err != nil {
		return PairingResult{}, apperrors.New(apperrors.KindInternal, "AUTH_PAIRING_SAVE_FAILED", "failed to persist pairing session", err)
	}
	return PairingResult{PairingToken: token, DesktopPublicKey: pub, ServerAddress: uc.serverAddress, ExpiresAt: session.ExpiresAt}, nil
}
```

`backend-go/services/auth-service/internal/usecase/complete_device_pairing.go`:

```go
package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

type CompleteDevicePairing struct {
	sessions    PairingSessionRepository
	devices     PairedDeviceRepository
	keyExchanger DeviceKeyExchanger
	sealer      SharedSecretSealer
	issueToken  *IssueToken // reuses the existing mobile/CLI JWT-issuance usecase
}

func NewCompleteDevicePairing(sessions PairingSessionRepository, devices PairedDeviceRepository, keyExchanger DeviceKeyExchanger, sealer SharedSecretSealer, issueToken *IssueToken) *CompleteDevicePairing {
	return &CompleteDevicePairing{sessions: sessions, devices: devices, keyExchanger: keyExchanger, sealer: sealer, issueToken: issueToken}
}

type CompleteResult struct {
	DeviceID                     string
	DesktopPublicKeyConfirmation []byte
	AccessToken, RefreshToken    string
}

func (uc *CompleteDevicePairing) Execute(ctx context.Context, token string, mobilePub []byte, label string) (CompleteResult, error) {
	session, err := uc.sessions.GetAndConsume(ctx, hashToken(token))
	if err != nil {
		// Deliberately the SAME generic error for not-found/already-consumed
		// — see REST wiring's "no oracle" requirement (TASK-MB-01-08).
		return CompleteResult{}, apperrors.New(apperrors.KindNotFound, "AUTH_PAIRING_TOKEN_INVALID", "pairing token is invalid or already used", err)
	}
	if session.Expired(time.Now()) {
		return CompleteResult{}, apperrors.New(apperrors.KindNotFound, "AUTH_PAIRING_TOKEN_INVALID", "pairing token is invalid or already used", domain.ErrPairingTokenExpired)
	}

	active, err := uc.devices.CountActive(ctx, session.TenantID, session.UserID)
	if err != nil {
		return CompleteResult{}, apperrors.New(apperrors.KindInternal, "AUTH_PAIRING_COUNT_FAILED", "failed to count active devices", err)
	}
	if active >= domain.MaxPairedDevicesPerAccount {
		return CompleteResult{}, apperrors.New(apperrors.KindFailedPrecondition, "AUTH_DEVICE_LIMIT_REACHED", "device pairing limit reached", domain.ErrDeviceLimitReached)
	}

	priv, err := uc.sealer.Decrypt(ctx, session.DesktopPrivateKeyCiphertext, session.VaultKeyRef)
	if err != nil {
		return CompleteResult{}, apperrors.New(apperrors.KindInternal, "AUTH_PAIRING_UNSEAL_FAILED", "failed to unseal ephemeral private key", err)
	}
	shared, err := uc.keyExchanger.SharedSecret(priv, mobilePub)
	for i := range priv {
		priv[i] = 0 // never persisted past this call
	}
	if err != nil {
		return CompleteResult{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_PAIRING_BAD_MOBILE_KEY", "invalid mobile public key", err)
	}

	secretCiphertext, keyRef, err := uc.sealer.Encrypt(ctx, shared)
	if err != nil {
		return CompleteResult{}, apperrors.New(apperrors.KindInternal, "AUTH_PAIRING_SEAL_FAILED", "failed to seal shared secret", err)
	}
	device := domain.PairedDevice{
		ID: newUUID(), TenantID: session.TenantID, UserID: session.UserID,
		DeviceLabel: label, SharedSecretCiphertext: secretCiphertext, VaultKeyRef: keyRef,
		Status: domain.DeviceActive, PairedAt: time.Now(),
	}
	if err := uc.devices.Save(ctx, device); err != nil {
		return CompleteResult{}, apperrors.New(apperrors.KindInternal, "AUTH_PAIRING_SAVE_DEVICE_FAILED", "failed to persist paired device", err)
	}

	access, refresh, err := uc.issueToken.ExecuteForDevice(ctx, session.UserID, device.ID)
	if err != nil {
		return CompleteResult{}, apperrors.New(apperrors.KindInternal, "AUTH_PAIRING_ISSUE_TOKEN_FAILED", "failed to issue device token", err)
	}
	return CompleteResult{DeviceID: device.ID, DesktopPublicKeyConfirmation: session.DesktopPublicKey, AccessToken: access, RefreshToken: refresh}, nil
}
```

Notes:
- `hashToken`/`newUUID` should reuse whatever helpers `auth-service`'s
  existing session-token hashing already uses (grep `sessions.id` hashing
  in `internal/adapter/postgres`/`internal/usecase` before adding a new one).
- `IssueToken.ExecuteForDevice` is a new, small extension of the existing
  `IssueToken` usecase — add a `deviceID` parameter/variant that embeds
  `device_id` into the minted JWT's claims (needed by TASK-MB-03/04's
  `wscompat.Identity.DeviceID`).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/auth-service/... && go vet ./services/auth-service/...
go test ./services/auth-service/internal/usecase/... -run 'InitiateDevicePairing|CompleteDevicePairing'
```

Test cases to add (fakes for `PairingSessionRepository`/`PairedDeviceRepository`/`DeviceKeyExchanger`/`SharedSecretSealer`):
- Expired token → `AUTH_PAIRING_TOKEN_INVALID`, no device row inserted.
- Already-consumed token (simulate via fake `GetAndConsume` returning not-found on 2nd call) → same error code as expired (no oracle).
- 4th device for an account with 3 active → `AUTH_DEVICE_LIMIT_REACHED`, no row inserted.
- Successful pairing → fake `IssueToken` invoked exactly once, response carries non-empty `access_token`/`refresh_token`.
