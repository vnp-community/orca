package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// Generator point G of P-256 (private scalar 1): a fixed, publicly known key.
const (
	goldenPEM = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEaxfR8uEsQkf4vOblY6RA8ncDfYEt
6zOg9KE5RdiYwpZP40Li/hp/m47n60p8D54WK84zV2sxXs7LtkBoN79R9Q==
-----END PUBLIC KEY-----`
	goldenPublicKey = "BGsX0fLhLEJH-Lzm5WOkQPJ3A32BLeszoPShOUXYmMKWT-NC4v4af5uO5-tKfA-eFivOM1drMV7Oy7ZAaDe_UfU"
)

func ensureCtx() context.Context { return tenant.WithTenantID(context.Background(), "tenant-1") }

func appCode(t *testing.T, err error) string {
	t.Helper()
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("not an AppError: %v", err)
	}
	return ae.Code
}

func TestVapidPublicKeyFromPEM_Golden(t *testing.T) {
	got, err := VapidPublicKeyFromPEM(goldenPEM)
	if err != nil {
		t.Fatal(err)
	}
	if got != goldenPublicKey {
		t.Fatalf("got %q want %q", got, goldenPublicKey)
	}
	if len(got) != 87 {
		t.Fatalf("len=%d want 87", len(got))
	}
}

func TestVapidPublicKeyFromPEM_RejectsGarbage(t *testing.T) {
	if _, err := VapidPublicKeyFromPEM("nope"); err == nil {
		t.Fatal("expected error")
	}
}

func TestEnsureVapidSigningKey_CreatesWhenMissing(t *testing.T) {
	store := newFakeSecretStore(&callRecorder{})
	store.createKeyPEM = goldenPEM
	res, err := NewEnsureVapidSigningKey(store).Execute(ensureCtx(), EnsureVapidSigningKeyInput{RequestingService: "notification-service"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Created || res.PublicKey != goldenPublicKey {
		t.Fatalf("unexpected result %+v", res)
	}
	if len(store.createKeyCalls) != 1 || store.createKeyCalls[0] != "vapid-signing-tenant-1:ecdsa-p256" {
		t.Fatalf("create calls: %v", store.createKeyCalls)
	}
}

func TestEnsureVapidSigningKey_ReusesExistingKey(t *testing.T) {
	store := newFakeSecretStore(&callRecorder{})
	store.transitKeys = map[string]*TransitKeyInfo{"vapid-signing-tenant-1": {Type: "ecdsa-p256", LatestVersion: 1, PublicKeyPEM: goldenPEM}}
	res, err := NewEnsureVapidSigningKey(store).Execute(ensureCtx(), EnsureVapidSigningKeyInput{RequestingService: "notification-service"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Created || res.PublicKey != goldenPublicKey {
		t.Fatalf("unexpected result %+v", res)
	}
	if len(store.createKeyCalls) != 0 {
		t.Fatalf("must not create: %v", store.createKeyCalls)
	}
}

func TestEnsureVapidSigningKey_WrongTypeRefusedWithoutMutation(t *testing.T) {
	store := newFakeSecretStore(&callRecorder{})
	store.transitKeys = map[string]*TransitKeyInfo{"vapid-signing-tenant-1": {Type: "aes256-gcm96", LatestVersion: 1}}
	_, err := NewEnsureVapidSigningKey(store).Execute(ensureCtx(), EnsureVapidSigningKeyInput{RequestingService: "notification-service"})
	if err == nil || appCode(t, err) != CodeVapidKeyTypeMismatch {
		t.Fatalf("want type mismatch, got %v", err)
	}
	if len(store.createKeyCalls) != 0 {
		t.Fatalf("must not create: %v", store.createKeyCalls)
	}
	// The port has no delete method; the key must still be there untouched.
	if store.transitKeys["vapid-signing-tenant-1"].Type != "aes256-gcm96" {
		t.Fatal("key was modified")
	}
}

func TestEnsureVapidSigningKey_VaultForbiddenMapped(t *testing.T) {
	for _, tc := range []struct {
		name     string
		read, cr error
	}{
		{"read", ErrSecretStoreForbidden, nil},
		{"create", nil, ErrSecretStoreForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeSecretStore(&callRecorder{})
			store.readKeyErr, store.createKeyErr = tc.read, tc.cr
			_, err := NewEnsureVapidSigningKey(store).Execute(ensureCtx(), EnsureVapidSigningKeyInput{RequestingService: "notification-service"})
			if err == nil || appCode(t, err) != CodeVaultForbidden {
				t.Fatalf("want forbidden, got %v", err)
			}
			var ae *apperrors.AppError
			errors.As(err, &ae)
			if ae.Kind != apperrors.KindPermissionDenied || !strings.Contains(ae.Message, "orca-policy.hcl") {
				t.Fatalf("bad mapping: %+v", ae)
			}
		})
	}
}

func TestEnsureVapidSigningKey_VaultUnavailableMapped(t *testing.T) {
	store := newFakeSecretStore(&callRecorder{})
	store.readKeyErr = ErrSecretStoreUnavailable
	_, err := NewEnsureVapidSigningKey(store).Execute(ensureCtx(), EnsureVapidSigningKeyInput{RequestingService: "notification-service"})
	if err == nil || appCode(t, err) != CodeVaultUnavailable {
		t.Fatalf("want unavailable, got %v", err)
	}
}

func TestEnsureVapidSigningKey_CallerNotAllowed(t *testing.T) {
	for _, caller := range []string{"", "mcp-service", "api-gateway"} {
		store := newFakeSecretStore(&callRecorder{})
		_, err := NewEnsureVapidSigningKey(store).Execute(ensureCtx(), EnsureVapidSigningKeyInput{RequestingService: caller})
		if err == nil || appCode(t, err) != "CREDENTIAL_CALLER_NOT_ALLOWED" {
			t.Fatalf("caller %q: got %v", caller, err)
		}
		if len(store.recorder.snapshot()) != 0 {
			t.Fatalf("caller %q reached the store", caller)
		}
	}
}

func TestEnsureVapidSigningKey_TenantRequiredAndSafe(t *testing.T) {
	uc := NewEnsureVapidSigningKey(newFakeSecretStore(&callRecorder{}))
	in := EnsureVapidSigningKeyInput{RequestingService: "notification-service"}
	if _, err := uc.Execute(context.Background(), in); err == nil {
		t.Fatal("expected error without tenant")
	}
	if _, err := uc.Execute(tenant.WithTenantID(context.Background(), "../sys/seal"), in); err == nil {
		t.Fatal("expected error for path-like tenant")
	}
}
