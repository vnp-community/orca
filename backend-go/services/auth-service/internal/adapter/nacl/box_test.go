package nacl

import (
	"bytes"
	"testing"
)

// TestSharedSecret_RoundTrip asserts two independently-generated keypairs
// (desktop, mobile) produce a byte-identical SharedSecret from each side —
// the NaCl box.Precompute property BL-MB-01's handshake depends on.
func TestSharedSecret_RoundTrip(t *testing.T) {
	desktop := New()
	mobile := New()

	desktopPub, desktopPriv, err := desktop.GenerateEphemeralKeypair()
	if err != nil {
		t.Fatalf("desktop.GenerateEphemeralKeypair: %v", err)
	}
	mobilePub, mobilePriv, err := mobile.GenerateEphemeralKeypair()
	if err != nil {
		t.Fatalf("mobile.GenerateEphemeralKeypair: %v", err)
	}

	desktopShared, err := desktop.SharedSecret(desktopPriv, mobilePub)
	if err != nil {
		t.Fatalf("desktop.SharedSecret: %v", err)
	}
	mobileShared, err := mobile.SharedSecret(mobilePriv, desktopPub)
	if err != nil {
		t.Fatalf("mobile.SharedSecret: %v", err)
	}

	if !bytes.Equal(desktopShared, mobileShared) {
		t.Fatalf("shared secrets diverge: desktop=%x mobile=%x", desktopShared, mobileShared)
	}
	if len(desktopShared) != 32 {
		t.Fatalf("expected 32-byte shared secret, got %d bytes", len(desktopShared))
	}
}

// TestSharedSecret_RejectsWrongKeyLength guards against a caller passing a
// truncated/corrupt key silently succeeding.
func TestSharedSecret_RejectsWrongKeyLength(t *testing.T) {
	ke := New()
	if _, err := ke.SharedSecret([]byte("too-short"), make([]byte, 32)); err == nil {
		t.Fatal("expected an error for a short private key, got nil")
	}
	if _, err := ke.SharedSecret(make([]byte, 32), []byte("too-short")); err == nil {
		t.Fatal("expected an error for a short peer public key, got nil")
	}
}
