package apns

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"math/big"
	"testing"
)

// TestAsn1ECDSAToJWS_ProducesVerifiableRawSignature signs a message with a
// real P-256 key, ASN.1-DER-encodes the signature the way Vault Transit's
// default marshaling does, converts it via asn1ECDSAToJWS, and verifies the
// raw r||s output against the public key — confirming the conversion is
// byte-for-byte correct, not just "doesn't error".
func TestAsn1ECDSAToJWS_ProducesVerifiableRawSignature(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	msg := []byte("header.claims")
	digest := sha256.Sum256(msg)

	r, s, err := ecdsa.Sign(rand.Reader, priv, digest[:])
	if err != nil {
		t.Fatalf("signing: %v", err)
	}
	der, err := asn1.Marshal(struct{ R, S *big.Int }{r, s})
	if err != nil {
		t.Fatalf("asn1.Marshal: %v", err)
	}

	raw, err := asn1ECDSAToJWS(der)
	if err != nil {
		t.Fatalf("asn1ECDSAToJWS: %v", err)
	}
	if len(raw) != 64 {
		t.Fatalf("expected 64-byte raw r||s signature for P-256, got %d", len(raw))
	}

	gotR := new(big.Int).SetBytes(raw[:32])
	gotS := new(big.Int).SetBytes(raw[32:])
	if !ecdsa.Verify(&priv.PublicKey, digest[:], gotR, gotS) {
		t.Fatal("converted raw signature failed to verify against the original public key")
	}
}

func TestDecodeVaultSignature_StripsWirePrefix(t *testing.T) {
	got, err := decodeVaultSignature("vault:v1:aGVsbG8=")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("expected decoded %q, got %q", "hello", got)
	}
	if _, err := decodeVaultSignature("not-a-vault-sig"); err == nil {
		t.Fatal("expected an error for a malformed wire signature")
	}
}
