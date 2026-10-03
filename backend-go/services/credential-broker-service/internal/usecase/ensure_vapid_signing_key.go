package usecase

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

// Error codes the notification-service client keys off.
const (
	CodeVaultForbidden       = "CREDBROKER_VAULT_FORBIDDEN"
	CodeVaultUnavailable     = "CREDBROKER_VAULT_UNAVAILABLE"
	CodeVapidKeyTypeMismatch = "CREDBROKER_VAPID_KEY_TYPE_MISMATCH"
	vapidKeyType             = "ecdsa-p256"
)

// EnsureVapidSigningKeyInput carries the caller identity; the tenant comes
// from ctx (request metadata), never from the message.
type EnsureVapidSigningKeyInput struct {
	RequestingService string
}

type EnsureVapidSigningKeyResult struct {
	PublicKey string
	Created   bool
}

// EnsureVapidSigningKey makes "vapid-signing-<tenant>" an ecdsa-p256 Transit
// key and returns its Web Push public key. Idempotent and race-safe: Vault
// create is a no-op on an existing key and the key is re-read afterwards. A
// key of another type is refused and never deleted — rotating it would
// silently invalidate every browser subscription.
type EnsureVapidSigningKey struct {
	store SecretStore
}

func NewEnsureVapidSigningKey(store SecretStore) *EnsureVapidSigningKey {
	return &EnsureVapidSigningKey{store: store}
}

func (uc *EnsureVapidSigningKey) Execute(ctx context.Context, in EnsureVapidSigningKeyInput) (EnsureVapidSigningKeyResult, error) {
	if in.RequestingService != domain.NotificationServiceCaller {
		return EnsureVapidSigningKeyResult{}, apperrors.New(apperrors.KindPermissionDenied, "CREDENTIAL_CALLER_NOT_ALLOWED", "caller is not allowed to provision vapid keys", domain.ErrCallerNotAllowed)
	}
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return EnsureVapidSigningKeyResult{}, apperrors.New(apperrors.KindInvalidArgument, "CREDENTIAL_MISSING_SCOPE", "tenant_id is required", err)
	}
	if !validTransitNameSegment(tenantID) {
		return EnsureVapidSigningKeyResult{}, apperrors.New(apperrors.KindInvalidArgument, "CREDENTIAL_MISSING_SCOPE", "tenant_id is malformed", nil)
	}
	name := vapidKeyName(tenantID)

	info, err := uc.store.TransitReadKey(ctx, name)
	if err != nil {
		return EnsureVapidSigningKeyResult{}, mapVaultError(err)
	}
	created := false
	if info == nil {
		if err := uc.store.TransitCreateKey(ctx, name, vapidKeyType); err != nil {
			return EnsureVapidSigningKeyResult{}, mapVaultError(err)
		}
		created = true
		if info, err = uc.store.TransitReadKey(ctx, name); err != nil {
			return EnsureVapidSigningKeyResult{}, mapVaultError(err)
		}
		if info == nil {
			return EnsureVapidSigningKeyResult{}, apperrors.New(apperrors.KindInternal, "CREDBROKER_VAPID_KEY_MISSING", "vapid key not readable after creation", nil)
		}
	}
	if info.Type != vapidKeyType {
		return EnsureVapidSigningKeyResult{}, apperrors.New(apperrors.KindFailedPrecondition, CodeVapidKeyTypeMismatch,
			fmt.Sprintf("transit key %s exists with type %q, expected %s; refusing to modify it", name, info.Type, vapidKeyType), nil)
	}
	pub, err := VapidPublicKeyFromPEM(info.PublicKeyPEM)
	if err != nil {
		return EnsureVapidSigningKeyResult{}, apperrors.New(apperrors.KindInternal, "CREDBROKER_VAPID_PUBLIC_KEY_INVALID", "failed to derive vapid public key", err)
	}
	return EnsureVapidSigningKeyResult{PublicKey: pub, Created: created}, nil
}

func mapVaultError(err error) error {
	switch {
	case errors.Is(err, ErrSecretStoreForbidden):
		return apperrors.New(apperrors.KindPermissionDenied, CodeVaultForbidden,
			"vault denied access to transit/keys/vapid-signing-*; apply deploy/dev/orca-policy.hcl to the credential-broker Vault role", err)
	case errors.Is(err, ErrSecretStoreUnavailable):
		return apperrors.New(apperrors.KindInternal, CodeVaultUnavailable, "vault is unavailable", err)
	default:
		return apperrors.New(apperrors.KindInternal, "CREDENTIAL_VAULT_ENSURE_FAILED", "failed to ensure vapid signing key", err)
	}
}

// validTransitNameSegment keeps a tenant id from altering the Vault path.
func validTransitNameSegment(s string) bool {
	if s == "" || len(s) > 128 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

// VapidPublicKeyFromPEM converts Vault's PKIX PEM public key into the Web
// Push form: base64url (no padding) of the 65-byte uncompressed point.
func VapidPublicKeyFromPEM(pemKey string) (string, error) {
	block, _ := pem.Decode([]byte(pemKey))
	if block == nil {
		return "", errors.New("public key is not PEM")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parsing public key: %w", err)
	}
	pub, ok := parsed.(*ecdsa.PublicKey)
	if !ok || pub.Curve != elliptic.P256() {
		return "", errors.New("public key is not a P-256 ECDSA key")
	}
	ecdhKey, err := pub.ECDH()
	if err != nil {
		return "", fmt.Errorf("converting public key: %w", err)
	}
	point := ecdhKey.Bytes() // uncompressed: 0x04 || X || Y
	if len(point) != 65 || point[0] != 0x04 {
		return "", errors.New("unexpected public key encoding")
	}
	return base64.RawURLEncoding.EncodeToString(point), nil
}
