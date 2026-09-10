package usecase

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"math/big"
	"strings"
	"testing"
)

// fakeVaultSigner adapts a plain func to VaultSigner — a func (not a
// canned response) because BuildVapidAuthHeader's signing_input embeds a
// live timestamp (exp claim), so a fixed pre-computed signature wouldn't
// verify against whatever input it actually constructs each call.
type fakeVaultSigner struct {
	fn func(ctx context.Context, tenantID string, payload []byte) (string, error)

	calls      int
	lastTenant string
	lastInput  []byte
}

func (f *fakeVaultSigner) SignVapidPayload(ctx context.Context, tenantID string, payload []byte) (string, error) {
	f.calls++
	f.lastTenant = tenantID
	f.lastInput = payload
	return f.fn(ctx, tenantID, payload)
}

// vaultWireSignature signs input with a real ECDSA P-256 key (ASN.1 DER,
// the shape crypto/ecdsa.SignASN1 and Vault Transit's default
// marshaling_algorithm=asn1 both produce) and wraps it in Vault's
// "vault:v1:<base64std>" wire format — a realistic stand-in for what
// credential-broker-service's SignVapidPayload RPC actually returns.
func vaultWireSignature(t *testing.T, priv *ecdsa.PrivateKey, input []byte) string {
	t.Helper()
	hash := sha256.Sum256(input)
	der, err := ecdsa.SignASN1(rand.Reader, priv, hash[:])
	if err != nil {
		t.Fatalf("sign asn1: %v", err)
	}
	return "vault:v1:" + base64.StdEncoding.EncodeToString(der)
}

func TestBuildVapidAuthHeader_CallsSignerExactlyOnce(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer := &fakeVaultSigner{fn: func(_ context.Context, _ string, payload []byte) (string, error) {
		return vaultWireSignature(t, priv, payload), nil
	}}

	header, err := BuildVapidAuthHeader(context.Background(), signer, "tenant-1", "https://push.example.com", "mailto:ops@orca.dev", "pubkey-b64")
	if err != nil {
		t.Fatalf("BuildVapidAuthHeader: %v", err)
	}
	if signer.calls != 1 {
		t.Fatalf("expected SignVapidPayload called exactly once, got %d", signer.calls)
	}
	if signer.lastTenant != "tenant-1" {
		t.Fatalf("expected tenantID %q, got %q", "tenant-1", signer.lastTenant)
	}
	if !strings.HasPrefix(header, "vapid t=") || !strings.Contains(header, ", k=pubkey-b64") {
		t.Fatalf("unexpected header shape: %s", header)
	}
}

func TestBuildVapidAuthHeader_SignerError_Propagates(t *testing.T) {
	signer := &fakeVaultSigner{fn: func(context.Context, string, []byte) (string, error) {
		return "", errors.New("vault unreachable")
	}}
	_, err := BuildVapidAuthHeader(context.Background(), signer, "tenant-1", "https://push.example.com", "mailto:ops@orca.dev", "pubkey-b64")
	if err == nil {
		t.Fatal("expected the signer error to propagate")
	}
}

// TestDerECDSASignatureToRawJWS_RoundTripsAndVerifies is the crypto
// correctness test for the ASN.1-DER → raw-r||s conversion this package
// does because Vault Transit's default marshaling_algorithm=asn1 doesn't
// match what a JWS ES256 signature needs (see derECDSASignatureToRawJWS's
// doc comment). Signs with the real stdlib ecdsa.SignASN1 (the same shape
// Vault produces), converts, then verifies the raw r||s bytes against the
// public key using the classic ecdsa.Verify API — proving the converted
// bytes are still a genuinely valid signature over the same message, not
// just "some 64 bytes."
func TestDerECDSASignatureToRawJWS_RoundTripsAndVerifies(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	msg := []byte("header.claims")
	hash := sha256.Sum256(msg)

	der, err := ecdsa.SignASN1(rand.Reader, priv, hash[:])
	if err != nil {
		t.Fatalf("sign asn1: %v", err)
	}

	raw, err := derECDSASignatureToRawJWS(der)
	if err != nil {
		t.Fatalf("derECDSASignatureToRawJWS: %v", err)
	}
	if len(raw) != 64 {
		t.Fatalf("expected a 64-byte raw r||s signature for P-256, got %d bytes", len(raw))
	}

	r := new(big.Int).SetBytes(raw[:32])
	s := new(big.Int).SetBytes(raw[32:])
	if !ecdsa.Verify(&priv.PublicKey, hash[:], r, s) {
		t.Fatal("converted raw signature does not verify against the original public key/message")
	}
}

func TestDerECDSASignatureToRawJWS_InvalidDER(t *testing.T) {
	if _, err := derECDSASignatureToRawJWS([]byte("not valid asn1")); err == nil {
		t.Fatal("expected an error for malformed ASN.1 input")
	}
}
