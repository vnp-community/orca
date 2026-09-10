package nacl

import (
	"bytes"
	"crypto/rand"
	"testing"

	"golang.org/x/crypto/nacl/secretbox"
)

func TestSealer_Seal_RoundTripsWithSecretboxOpen(t *testing.T) {
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		t.Fatalf("generating test key: %v", err)
	}
	plaintext := []byte(`{"type":"agent_completed","title":"done"}`)

	s := New()
	ciphertext, nonce, err := s.Seal(plaintext, key[:])
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if len(nonce) != nonceSize {
		t.Fatalf("expected %d-byte nonce, got %d", nonceSize, len(nonce))
	}
	if bytes.Contains(ciphertext, plaintext) {
		t.Fatalf("ciphertext must not contain the plaintext")
	}

	var nonceArr [24]byte
	copy(nonceArr[:], nonce)
	opened, ok := secretbox.Open(nil, ciphertext, &nonceArr, &key)
	if !ok {
		t.Fatalf("secretbox.Open failed to authenticate/decrypt Seal's output")
	}
	if !bytes.Equal(opened, plaintext) {
		t.Fatalf("round-trip mismatch: got %q, want %q", opened, plaintext)
	}
}

func TestSealer_Seal_RejectsWrongKeySize(t *testing.T) {
	s := New()
	if _, _, err := s.Seal([]byte("hi"), []byte("too-short")); err == nil {
		t.Fatal("expected an error for a non-32-byte shared secret")
	}
}
