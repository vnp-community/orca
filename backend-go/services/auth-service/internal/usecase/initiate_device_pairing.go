package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// pairingSessionTTL is BR-MB-01's 5-minute pairing-token expiry.
const pairingSessionTTL = 5 * time.Minute

// pairingTokenBytes is the entropy generateRandomToken reads for the raw
// pairing token — same size Login uses for session tokens (see
// internal/usecase/token.go).
const pairingTokenBytes = 32

// InitiateDevicePairing starts a new QR-pairing attempt (BL-MB-01): mints a
// fresh ephemeral X25519 keypair, seals the private half through Vault
// Transit, and persists a PairingSession the caller's mobile app has 5
// minutes (BR-MB-01) to complete via CompleteDevicePairing.
type InitiateDevicePairing struct {
	sessions      PairingSessionRepository
	keyExchanger  DeviceKeyExchanger
	sealer        SharedSecretSealer
	clock         Clock
	serverAddress string // api-gateway's public base URL, from config
}

func NewInitiateDevicePairing(sessions PairingSessionRepository, keyExchanger DeviceKeyExchanger, sealer SharedSecretSealer, clock Clock, serverAddress string) *InitiateDevicePairing {
	return &InitiateDevicePairing{sessions: sessions, keyExchanger: keyExchanger, sealer: sealer, clock: clock, serverAddress: serverAddress}
}

// PairingResult carries the raw pairing token — like LoginOutput's session
// token, the ONLY point in the system's lifetime this raw value exists
// outside the caller's hands; from here on only its hash
// (domain.PairingSession.ID) is ever seen again.
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
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return PairingResult{}, apperrors.New(apperrors.KindUnauthenticated, "AUTH_NO_ACTOR", "no authenticated user in request context", nil)
	}

	pub, priv, err := uc.keyExchanger.GenerateEphemeralKeypair()
	if err != nil {
		return PairingResult{}, apperrors.New(apperrors.KindInternal, "AUTH_PAIRING_KEYGEN_FAILED", "failed to generate pairing keypair", err)
	}
	ciphertext, keyRef, err := uc.sealer.Encrypt(ctx, priv)
	if err != nil {
		return PairingResult{}, apperrors.New(apperrors.KindInternal, "AUTH_PAIRING_SEAL_FAILED", "failed to seal ephemeral private key", err)
	}
	token, err := generateRandomToken(pairingTokenBytes)
	if err != nil {
		return PairingResult{}, apperrors.New(apperrors.KindInternal, "AUTH_PAIRING_TOKEN_GEN_FAILED", "failed to generate pairing token", err)
	}

	now := uc.clock.Now()
	session := domain.PairingSession{
		ID:                          hashToken(token),
		TenantID:                    tenantID,
		UserID:                      userID,
		DesktopPublicKey:            pub,
		DesktopPrivateKeyCiphertext: ciphertext,
		VaultKeyRef:                 keyRef,
		CreatedAt:                   now,
		ExpiresAt:                   now.Add(pairingSessionTTL),
	}
	if err := uc.sessions.Save(ctx, session); err != nil {
		return PairingResult{}, apperrors.New(apperrors.KindInternal, "AUTH_PAIRING_SAVE_FAILED", "failed to persist pairing session", err)
	}
	return PairingResult{
		PairingToken:     token,
		DesktopPublicKey: pub,
		ServerAddress:    uc.serverAddress,
		ExpiresAt:        session.ExpiresAt,
	}, nil
}
