package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
)

func newPairingCtx(tenantID, userID string) context.Context {
	ctx := tenant.WithTenantID(context.Background(), tenantID)
	return tenant.WithUserID(ctx, userID)
}

func TestInitiateDevicePairing_Execute_Success(t *testing.T) {
	sessions := newFakePairingSessionRepository()
	ke := &fakeDeviceKeyExchanger{}
	sealer := &fakeSharedSecretSealer{}
	clock := &fakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}

	uc := NewInitiateDevicePairing(sessions, ke, sealer, clock, "https://example.orca.dev")

	out, err := uc.Execute(newPairingCtx("tenant-1", "user-1"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.PairingToken == "" {
		t.Fatal("expected a non-empty pairing token")
	}
	if len(out.DesktopPublicKey) == 0 {
		t.Fatal("expected a non-empty desktop public key")
	}
	if out.ServerAddress != "https://example.orca.dev" {
		t.Fatalf("ServerAddress = %q, want the configured server address", out.ServerAddress)
	}
	if !out.ExpiresAt.Equal(clock.now.Add(pairingSessionTTL)) {
		t.Fatalf("ExpiresAt = %v, want now+5m", out.ExpiresAt)
	}

	stored, ok := sessions.byID[hashToken(out.PairingToken)]
	if !ok {
		t.Fatal("expected a PairingSession saved keyed by hash(pairing_token)")
	}
	if stored.TenantID != "tenant-1" || stored.UserID != "user-1" {
		t.Fatalf("stored session identity = (%q,%q), want (tenant-1,user-1)", stored.TenantID, stored.UserID)
	}
}

func TestInitiateDevicePairing_Execute_NoTenant(t *testing.T) {
	uc := NewInitiateDevicePairing(newFakePairingSessionRepository(), &fakeDeviceKeyExchanger{}, &fakeSharedSecretSealer{}, &fakeClock{}, "")
	if _, err := uc.Execute(context.Background()); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}
