package usecase

import (
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

func TestListPairedDevices_Execute_ScopedToCallerIdentity(t *testing.T) {
	devices := newFakePairedDeviceRepository()
	devices.byID["d1"] = domain.PairedDevice{ID: "d1", TenantID: "tenant-1", UserID: "user-1", Status: domain.DeviceActive}
	devices.byID["d2"] = domain.PairedDevice{ID: "d2", TenantID: "tenant-1", UserID: "someone-else", Status: domain.DeviceActive}

	uc := NewListPairedDevices(devices)
	out, err := uc.Execute(newPairingCtx("tenant-1", "user-1"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(out) != 1 || out[0].ID != "d1" {
		t.Fatalf("expected only the caller's own device, got %+v", out)
	}
}

func TestUnpairDevice_ThenResolveDeviceSharedSecret_NoOracle(t *testing.T) {
	devices := newFakePairedDeviceRepository()
	devices.byID["d1"] = domain.PairedDevice{
		ID: "d1", TenantID: "tenant-1", UserID: "user-1",
		SharedSecretCiphertext: []byte("sealed:shared-secret"), VaultKeyRef: "fake-key-ref",
		Status: domain.DeviceActive,
	}
	sealer := &fakeSharedSecretSealer{}
	clock := &fakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}

	unpair := NewUnpairDevice(devices)
	if err := unpair.Execute(newPairingCtx("tenant-1", "user-1"), "d1"); err != nil {
		t.Fatalf("UnpairDevice.Execute: %v", err)
	}

	stored := devices.byID["d1"]
	if stored.Status != domain.DeviceRevoked {
		t.Fatalf("expected device status revoked, got %q", stored.Status)
	}
	if stored.SharedSecretCiphertext != nil {
		t.Fatal("expected shared_secret_ciphertext to be wiped, not just flagged")
	}

	resolve := NewResolveDeviceSharedSecret(devices, sealer, clock)
	_, err := resolve.Execute(newPairingCtx("tenant-1", "user-1"), "d1")
	if err == nil {
		t.Fatal("expected an error resolving a shared secret for an unpaired device")
	}
	if code := appErrorCode(t, err); code != "AUTH_DEVICE_NOT_FOUND" {
		t.Fatalf("error code = %q, want AUTH_DEVICE_NOT_FOUND", code)
	}
	// The nulled ciphertext must be checked BEFORE any decrypt attempt —
	// asserting the sealer was never invoked closes off "decrypt something
	// stale" as a possible bug.
	if sealer.decryptCalled {
		t.Fatal("expected Decrypt to NOT be called for a device whose ciphertext was wiped")
	}
}

func TestResolveDeviceSharedSecret_Execute_Success(t *testing.T) {
	devices := newFakePairedDeviceRepository()
	devices.byID["d1"] = domain.PairedDevice{
		ID: "d1", TenantID: "tenant-1", UserID: "user-1",
		SharedSecretCiphertext: []byte("sealed:the-secret"), VaultKeyRef: "fake-key-ref",
		Status: domain.DeviceActive,
	}
	sealer := &fakeSharedSecretSealer{}
	clock := &fakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}

	uc := NewResolveDeviceSharedSecret(devices, sealer, clock)
	secret, err := uc.Execute(newPairingCtx("tenant-1", "user-1"), "d1")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if string(secret) != "the-secret" {
		t.Fatalf("secret = %q, want %q", secret, "the-secret")
	}
	if !devices.touchCalled {
		t.Fatal("expected Touch to be called on successful resolve")
	}
}
