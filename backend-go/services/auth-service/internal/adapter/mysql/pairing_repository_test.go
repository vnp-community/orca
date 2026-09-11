//go:build integration

// Integration tests for PairingSessionStore/PairedDeviceStore — see
// repository_test.go's doc comment: auth-service's Postgres adapter has no
// integration test for either of these ports, so this is fresh coverage,
// not a 1:1 port.
package mysql

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

func TestPairingSessionStore_SaveAndGetAndConsume(t *testing.T) {
	db := setupMySQLDB(t)
	repo := New(db)
	pairingSessions := NewPairingSessionStore(db)
	ctx := context.Background()

	user := createTestUser(t, repo, uuid.NewString(), "pairing-user@example.com")
	now := time.Now().UTC().Truncate(time.Second)

	session := domain.PairingSession{
		ID:                          domain.HashSessionToken("raw-pairing-token"),
		TenantID:                    user.TenantID,
		UserID:                      user.ID,
		DesktopPublicKey:            []byte("pubkey-bytes"),
		DesktopPrivateKeyCiphertext: []byte("ciphertext-bytes"),
		VaultKeyRef:                 "transit/keys/device-pairing-1",
		CreatedAt:                   now,
		ExpiresAt:                   now.Add(10 * time.Minute),
	}
	if err := pairingSessions.Save(ctx, session); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := pairingSessions.GetAndConsume(ctx, session.ID)
	if err != nil {
		t.Fatalf("get and consume: %v", err)
	}
	if got.UserID != user.ID || got.ConsumedAt == nil {
		t.Errorf("unexpected consumed session: %+v", got)
	}

	// BR-MB-02: a second GetAndConsume on the same id must fail — already
	// consumed. This is also the RowsAffected()-safe path documented in
	// pairing_session_repository.go: consumed_at only ever transitions
	// NULL->non-NULL once, so there's no MySQL-vs-Postgres RowsAffected
	// ambiguity to worry about here.
	if _, err := pairingSessions.GetAndConsume(ctx, session.ID); err == nil {
		t.Fatal("expected a second GetAndConsume to fail (already consumed)")
	}

	// A nonexistent id must also fail.
	if _, err := pairingSessions.GetAndConsume(ctx, "does-not-exist"); err == nil {
		t.Fatal("expected GetAndConsume of an unknown id to fail")
	}
}

func TestPairedDeviceStore_Lifecycle(t *testing.T) {
	db := setupMySQLDB(t)
	repo := New(db)
	pairedDevices := NewPairedDeviceStore(db)
	ctx := context.Background()

	user := createTestUser(t, repo, uuid.NewString(), "paired-device-user@example.com")
	now := time.Now().UTC().Truncate(time.Second)

	device := domain.PairedDevice{
		ID:                     uuid.NewString(),
		TenantID:               user.TenantID,
		UserID:                 user.ID,
		DeviceLabel:            "My Phone",
		SharedSecretCiphertext: []byte("sealed-secret-bytes"),
		VaultKeyRef:            "transit/keys/device-shared-secret-1",
		Status:                 domain.DeviceActive,
		PairedAt:               now,
	}
	if err := pairedDevices.Save(ctx, device); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := pairedDevices.Get(ctx, device.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.DeviceLabel != "My Phone" || string(got.SharedSecretCiphertext) != "sealed-secret-bytes" {
		t.Errorf("unexpected device: %+v", got)
	}

	count, err := pairedDevices.CountActive(ctx, user.TenantID, user.ID)
	if err != nil || count != 1 {
		t.Fatalf("expected 1 active device, got %d, err %v", count, err)
	}

	list, err := pairedDevices.List(ctx, user.TenantID, user.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("expected 1 listed device, got %d, err %v", len(list), err)
	}

	if err := pairedDevices.Touch(ctx, device.ID, now.Add(time.Hour)); err != nil {
		t.Fatalf("touch: %v", err)
	}

	if err := pairedDevices.RevokeAndWipeSecret(ctx, device.ID); err != nil {
		t.Fatalf("revoke and wipe secret: %v", err)
	}
	got, err = pairedDevices.Get(ctx, device.ID)
	if err != nil {
		t.Fatalf("get after revoke: %v", err)
	}
	if got.Status != domain.DeviceRevoked || got.SharedSecretCiphertext != nil || got.VaultKeyRef != "" {
		t.Errorf("expected wiped/revoked device, got %+v", got)
	}
	count, err = pairedDevices.CountActive(ctx, user.TenantID, user.ID)
	if err != nil || count != 0 {
		t.Fatalf("expected 0 active devices after revoke, got %d, err %v", count, err)
	}

	if _, err := pairedDevices.Get(ctx, "does-not-exist"); err == nil {
		t.Fatal("expected Get of an unknown device id to fail")
	}
}

// TestPairedDeviceStore_RevokeAndWipeSecret_NoopRetryStillSucceeds proves
// the fix documented in paired_device_repository.go's RevokeAndWipeSecret
// doc comment: a second revoke-and-wipe of an already-revoked,
// already-wiped device (every SET value identical to the prior call) must
// NOT report ErrDeviceNotFound.
func TestPairedDeviceStore_RevokeAndWipeSecret_NoopRetryStillSucceeds(t *testing.T) {
	db := setupMySQLDB(t)
	repo := New(db)
	pairedDevices := NewPairedDeviceStore(db)
	ctx := context.Background()

	user := createTestUser(t, repo, uuid.NewString(), "wipe-retry-user@example.com")
	now := time.Now().UTC().Truncate(time.Second)
	device := domain.PairedDevice{
		ID:                     uuid.NewString(),
		TenantID:               user.TenantID,
		UserID:                 user.ID,
		SharedSecretCiphertext: []byte("sealed-secret-bytes"),
		VaultKeyRef:            "transit/keys/device-shared-secret-1",
		Status:                 domain.DeviceActive,
		PairedAt:               now,
	}
	if err := pairedDevices.Save(ctx, device); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := pairedDevices.RevokeAndWipeSecret(ctx, device.ID); err != nil {
		t.Fatalf("first revoke: %v", err)
	}
	if err := pairedDevices.RevokeAndWipeSecret(ctx, device.ID); err != nil {
		t.Fatalf("expected idempotent revoke-and-wipe retry to succeed, got: %v", err)
	}
}
