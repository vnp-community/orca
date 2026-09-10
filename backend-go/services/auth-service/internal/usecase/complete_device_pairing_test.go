package usecase

import (
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

func appErrorCode(t *testing.T, err error) string {
	t.Helper()
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected an *apperrors.AppError, got %T: %v", err, err)
	}
	return ae.Code
}

func seedPairingSession(sessions *fakePairingSessionRepository, token string, expiresAt time.Time) domain.PairingSession {
	session := domain.PairingSession{
		ID:                          hashToken(token),
		TenantID:                    "tenant-1",
		UserID:                      "user-1",
		DesktopPublicKey:            []byte("desktop-pub"),
		DesktopPrivateKeyCiphertext: []byte("sealed:desktop-priv"),
		VaultKeyRef:                 "fake-key-ref",
		CreatedAt:                   expiresAt.Add(-pairingSessionTTL),
		ExpiresAt:                   expiresAt,
	}
	sessions.byID[session.ID] = session
	return session
}

func newCompleteDevicePairingUC(sessions *fakePairingSessionRepository, devices *fakePairedDeviceRepository, signer *fakeTokenSigner, now time.Time) *CompleteDevicePairing {
	return NewCompleteDevicePairing(sessions, devices, &fakeDeviceKeyExchanger{}, &fakeSharedSecretSealer{}, signer, &fakeClock{now: now}, time.Hour)
}

func TestCompleteDevicePairing_Execute_ExpiredToken(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	sessions := newFakePairingSessionRepository()
	seedPairingSession(sessions, "expired-token", now.Add(-time.Minute)) // already past expiry
	devices := newFakePairedDeviceRepository()

	uc := newCompleteDevicePairingUC(sessions, devices, &fakeTokenSigner{}, now)
	_, err := uc.Execute(newPairingCtx("tenant-1", "user-1"), "expired-token", []byte("mobile-pub"), "Sam's iPhone")
	if err == nil {
		t.Fatal("expected an error for an expired pairing token")
	}
	if code := appErrorCode(t, err); code != "AUTH_PAIRING_TOKEN_INVALID" {
		t.Fatalf("error code = %q, want AUTH_PAIRING_TOKEN_INVALID", code)
	}
	if len(devices.byID) != 0 {
		t.Fatalf("expected no device row inserted, got %d", len(devices.byID))
	}
}

func TestCompleteDevicePairing_Execute_AlreadyConsumedToken(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	sessions := newFakePairingSessionRepository()
	seedPairingSession(sessions, "reused-token", now.Add(time.Minute))
	devices := newFakePairedDeviceRepository()

	uc := newCompleteDevicePairingUC(sessions, devices, &fakeTokenSigner{}, now)
	ctx := newPairingCtx("tenant-1", "user-1")

	// First call consumes the token successfully.
	if _, err := uc.Execute(ctx, "reused-token", []byte("mobile-pub"), "Sam's iPhone"); err != nil {
		t.Fatalf("first Execute: %v", err)
	}

	// Second call on the SAME token must fail with the identical error code
	// an expired token would produce — no oracle distinguishing the two.
	_, err := uc.Execute(ctx, "reused-token", []byte("mobile-pub"), "Sam's iPhone")
	if err == nil {
		t.Fatal("expected an error for an already-consumed pairing token")
	}
	if code := appErrorCode(t, err); code != "AUTH_PAIRING_TOKEN_INVALID" {
		t.Fatalf("error code = %q, want AUTH_PAIRING_TOKEN_INVALID (same as expired)", code)
	}
}

func TestCompleteDevicePairing_Execute_DeviceLimitReached(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	sessions := newFakePairingSessionRepository()
	seedPairingSession(sessions, "fourth-device-token", now.Add(time.Minute))
	devices := newFakePairedDeviceRepository()
	for i := 0; i < domain.MaxPairedDevicesPerAccount; i++ {
		devices.byID[newUUID()] = domain.PairedDevice{
			ID: newUUID(), TenantID: "tenant-1", UserID: "user-1", Status: domain.DeviceActive,
		}
	}

	uc := newCompleteDevicePairingUC(sessions, devices, &fakeTokenSigner{}, now)
	before := len(devices.byID)

	_, err := uc.Execute(newPairingCtx("tenant-1", "user-1"), "fourth-device-token", []byte("mobile-pub"), "4th device")
	if err == nil {
		t.Fatal("expected an error when the account already has 3 active devices")
	}
	if code := appErrorCode(t, err); code != "AUTH_DEVICE_LIMIT_REACHED" {
		t.Fatalf("error code = %q, want AUTH_DEVICE_LIMIT_REACHED", code)
	}
	if len(devices.byID) != before {
		t.Fatalf("expected no additional device row inserted, count changed from %d to %d", before, len(devices.byID))
	}
}

func TestCompleteDevicePairing_Execute_Success(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	sessions := newFakePairingSessionRepository()
	seedPairingSession(sessions, "good-token", now.Add(time.Minute))
	devices := newFakePairedDeviceRepository()
	signer := &fakeTokenSigner{token: "signed-jwt"}

	uc := newCompleteDevicePairingUC(sessions, devices, signer, now)
	out, err := uc.Execute(newPairingCtx("tenant-1", "user-1"), "good-token", []byte("mobile-pub"), "Sam's iPhone")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.AccessToken == "" {
		t.Fatal("expected a non-empty access_token")
	}
	if out.RefreshToken == "" {
		t.Fatal("expected a non-empty refresh_token")
	}
	if out.DeviceID == "" {
		t.Fatal("expected a non-empty device_id")
	}
	if signer.lastCall.DeviceID != out.DeviceID {
		t.Fatalf("signed claims device_id = %q, want %q", signer.lastCall.DeviceID, out.DeviceID)
	}
	if _, ok := devices.byID[out.DeviceID]; !ok {
		t.Fatal("expected the new device to be persisted")
	}
}
