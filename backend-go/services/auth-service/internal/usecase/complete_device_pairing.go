package usecase

import (
	"context"
	"time"

	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/jwtauth"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// deviceRefreshTokenBytes is the entropy generateRandomToken reads for a
// paired device's refresh token — same size Login/pairing-token generation
// uses (see internal/usecase/token.go).
const deviceRefreshTokenBytes = 32

// CompleteDevicePairing finishes the handshake BL-MB-01 describes:
// consumes the pairing token (BR-MB-02, atomically, via
// PairingSessionRepository.GetAndConsume), enforces the BR-MB-03 device
// cap, computes the NaCl shared secret, persists the new PairedDevice, and
// mints device-scoped access/refresh tokens.
//
// KNOWN GAP (mirrors IssueServiceToken's own "Known gaps" note): this
// service has no standing refresh-token subsystem yet — the returned
// refresh_token is an opaque high-entropy value handed back to the caller
// exactly once, the same "raw value never stored" contract session tokens
// use, but there is no RefreshToken RPC yet that redeems it for a new
// access token. Wiring that in is out of this task's scope.
type CompleteDevicePairing struct {
	sessions     PairingSessionRepository
	devices      PairedDeviceRepository
	keyExchanger DeviceKeyExchanger
	sealer       SharedSecretSealer
	signer       TokenSigner
	clock        Clock
	accessTTL    time.Duration
}

func NewCompleteDevicePairing(sessions PairingSessionRepository, devices PairedDeviceRepository, keyExchanger DeviceKeyExchanger, sealer SharedSecretSealer, signer TokenSigner, clock Clock, accessTTL time.Duration) *CompleteDevicePairing {
	return &CompleteDevicePairing{
		sessions: sessions, devices: devices, keyExchanger: keyExchanger, sealer: sealer,
		signer: signer, clock: clock, accessTTL: accessTTL,
	}
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
	if session.Expired(uc.clock.Now()) {
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
	shared, sharedErr := uc.keyExchanger.SharedSecret(priv, mobilePub)
	for i := range priv {
		priv[i] = 0 // never persisted past this call
	}
	if sharedErr != nil {
		return CompleteResult{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_PAIRING_BAD_MOBILE_KEY", "invalid mobile public key", sharedErr)
	}

	secretCiphertext, keyRef, err := uc.sealer.Encrypt(ctx, shared)
	if err != nil {
		return CompleteResult{}, apperrors.New(apperrors.KindInternal, "AUTH_PAIRING_SEAL_FAILED", "failed to seal shared secret", err)
	}
	now := uc.clock.Now()
	device := domain.PairedDevice{
		ID:                     newUUID(),
		TenantID:               session.TenantID,
		UserID:                 session.UserID,
		DeviceLabel:            label,
		SharedSecretCiphertext: secretCiphertext,
		VaultKeyRef:            keyRef,
		Status:                 domain.DeviceActive,
		PairedAt:               now,
	}
	if err := uc.devices.Save(ctx, device); err != nil {
		return CompleteResult{}, apperrors.New(apperrors.KindInternal, "AUTH_PAIRING_SAVE_DEVICE_FAILED", "failed to persist paired device", err)
	}

	access, refresh, err := uc.issueDeviceTokens(ctx, session.TenantID, session.UserID, device.ID, now)
	if err != nil {
		return CompleteResult{}, apperrors.New(apperrors.KindInternal, "AUTH_PAIRING_ISSUE_TOKEN_FAILED", "failed to issue device token", err)
	}
	return CompleteResult{
		DeviceID:                     device.ID,
		DesktopPublicKeyConfirmation: session.DesktopPublicKey,
		AccessToken:                  access,
		RefreshToken:                 refresh,
	}, nil
}

// issueDeviceTokens mints the access JWT (device_id claim set, so
// wscompat.Identity.DeviceID can be threaded through later) and a raw
// opaque refresh token, mirroring IssueServiceToken's claim-building
// pattern but keyed to a paired device rather than an audience.
func (uc *CompleteDevicePairing) issueDeviceTokens(ctx context.Context, tenantID, userID, deviceID string, now time.Time) (access, refresh string, err error) {
	jti, err := generateRandomToken(serviceTokenJTIBytes)
	if err != nil {
		return "", "", err
	}
	claims := jwtauth.Claims{
		Claims: jwt.Claims{
			Issuer:   jwtauth.Issuer,
			Subject:  userID,
			IssuedAt: jwt.NewNumericDate(now),
			Expiry:   jwt.NewNumericDate(now.Add(uc.accessTTL)),
			ID:       jti,
		},
		TenantID: tenantID,
		DeviceID: deviceID,
	}
	access, err = uc.signer.Sign(ctx, claims)
	if err != nil {
		return "", "", err
	}
	refresh, err = generateRandomToken(deviceRefreshTokenBytes)
	if err != nil {
		return "", "", err
	}
	return access, refresh, nil
}
