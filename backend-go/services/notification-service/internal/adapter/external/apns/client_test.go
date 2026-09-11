package apns

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/base64"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stablyai/orca-go/services/notification-service/internal/usecase"
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

func TestIsTokenInvalid(t *testing.T) {
	cases := []struct {
		name       string
		statusCode int
		body       string
		want       bool
	}{
		{"410 Gone is always invalid", http.StatusGone, `{"reason":"Unregistered"}`, true},
		{"410 Gone with empty body is still invalid", http.StatusGone, ``, true},
		{"400 BadDeviceToken is invalid", http.StatusBadRequest, `{"reason":"BadDeviceToken"}`, true},
		{"400 with a different reason is NOT invalid", http.StatusBadRequest, `{"reason":"TopicDisallowed"}`, false},
		{"400 with unparseable body is NOT invalid", http.StatusBadRequest, `not json`, false},
		{"403 (bad provider token) is NOT invalid", http.StatusForbidden, `{"reason":"InvalidProviderToken"}`, false},
		{"5xx is NOT invalid (transient)", http.StatusInternalServerError, ``, false},
		{"429 is NOT invalid (transient)", http.StatusTooManyRequests, `{"reason":"TooManyRequests"}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isTokenInvalid(tc.statusCode, []byte(tc.body))
			if got != tc.want {
				t.Errorf("isTokenInvalid(%d, %q) = %v, want %v", tc.statusCode, tc.body, got, tc.want)
			}
		})
	}
}

type fakeTransitSigner struct{}

func (fakeTransitSigner) TransitSign(ctx context.Context, keyName string, input []byte) (string, error) {
	// A syntactically valid (if not cryptographically meaningful) ASN.1
	// DER ECDSA signature, vault-wire-prefixed — enough for providerToken's
	// decode/convert pipeline to succeed so Send() reaches the HTTP call.
	der, err := asn1.Marshal(struct{ R, S *big.Int }{big.NewInt(1), big.NewInt(1)})
	if err != nil {
		return "", err
	}
	return "vault:v1:" + base64.StdEncoding.EncodeToString(der), nil
}

// TestSend_WrapsErrDeviceTokenInvalid_On410Gone confirms end-to-end (not
// just the isTokenInvalid helper in isolation) that a real Send() call
// against a 410-returning server produces an error satisfying
// errors.Is(err, usecase.ErrDeviceTokenInvalid) — the actual contract
// DeliverPush.markExpiredOnDeadToken relies on.
func TestSend_WrapsErrDeviceTokenInvalid_On410Gone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write([]byte(`{"reason":"Unregistered"}`))
	}))
	defer srv.Close()

	c := New(fakeTransitSigner{}, Config{TeamID: "T1", KeyID: "K1", Topic: "com.example.app", Endpoint: srv.URL})
	err := c.Send(context.Background(), "dead-token", []byte("cipher"), []byte("nonce"))
	if err == nil {
		t.Fatal("expected an error for a 410 response")
	}
	if !errors.Is(err, usecase.ErrDeviceTokenInvalid) {
		t.Errorf("expected errors.Is(err, usecase.ErrDeviceTokenInvalid), got: %v", err)
	}
}

// TestSend_DoesNotWrapErrDeviceTokenInvalid_On5xx confirms a transient
// server error is NOT classified as a dead token — DeliverPush must let
// its normal buffer-and-retry path handle this, not mark the subscription
// expired.
func TestSend_DoesNotWrapErrDeviceTokenInvalid_On5xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := New(fakeTransitSigner{}, Config{TeamID: "T1", KeyID: "K1", Topic: "com.example.app", Endpoint: srv.URL})
	err := c.Send(context.Background(), "some-token", []byte("cipher"), []byte("nonce"))
	if err == nil {
		t.Fatal("expected an error for a 503 response")
	}
	if errors.Is(err, usecase.ErrDeviceTokenInvalid) {
		t.Errorf("expected a transient 503 NOT to be classified as ErrDeviceTokenInvalid, got: %v", err)
	}
}
