package webpush

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/crypto/hkdf"

	"github.com/stablyai/orca-go/services/notification-service/internal/usecase"
)

// TestEncryptAES128GCM_RoundTrips independently re-derives the receiver
// side of RFC 8291 (as a real browser/push-service would) and verifies it
// recovers the exact plaintext encryptAES128GCM produced — a genuine
// protocol conformance check, not just "it doesn't error".
func TestEncryptAES128GCM_RoundTrips(t *testing.T) {
	curve := ecdh.P256()
	uaPrivate, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating subscriber key: %v", err)
	}
	uaPublicRaw := uaPrivate.PublicKey().Bytes()

	authSecret := make([]byte, 16)
	if _, err := rand.Read(authSecret); err != nil {
		t.Fatalf("generating auth secret: %v", err)
	}

	p256dhB64 := base64.RawURLEncoding.EncodeToString(uaPublicRaw)
	authB64 := base64.RawURLEncoding.EncodeToString(authSecret)

	plaintext := []byte(`{"type":"agent_completed"}`)
	encoded, err := encryptAES128GCM(plaintext, p256dhB64, authB64)
	if err != nil {
		t.Fatalf("encryptAES128GCM: %v", err)
	}
	if len(encoded) < 16+4+1 {
		t.Fatalf("encoded output too short: %d bytes", len(encoded))
	}

	// Parse RFC 8188 header.
	salt := encoded[0:16]
	idlen := int(encoded[20])
	asPublicRaw := encoded[21 : 21+idlen]
	record := encoded[21+idlen:]

	asPublic, err := curve.NewPublicKey(asPublicRaw)
	if err != nil {
		t.Fatalf("parsing ephemeral public key from header: %v", err)
	}
	ecdhSecret, err := uaPrivate.ECDH(asPublic)
	if err != nil {
		t.Fatalf("ecdh: %v", err)
	}

	keyInfo := append([]byte("WebPush: info\x00"), uaPublicRaw...)
	keyInfo = append(keyInfo, asPublicRaw...)
	ikm := make([]byte, 32)
	if _, err := io.ReadFull(hkdf.New(sha256.New, ecdhSecret, authSecret, keyInfo), ikm); err != nil {
		t.Fatalf("deriving ikm: %v", err)
	}

	cek := make([]byte, 16)
	if _, err := io.ReadFull(hkdf.New(sha256.New, ikm, salt, []byte("Content-Encoding: aes128gcm\x00")), cek); err != nil {
		t.Fatalf("deriving cek: %v", err)
	}
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(hkdf.New(sha256.New, ikm, salt, []byte("Content-Encoding: nonce\x00")), nonce); err != nil {
		t.Fatalf("deriving nonce: %v", err)
	}

	block, err := aes.NewCipher(cek)
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("cipher.NewGCM: %v", err)
	}
	opened, err := gcm.Open(nil, nonce, record, nil)
	if err != nil {
		t.Fatalf("gcm.Open: %v", err)
	}
	// Strip RFC 8188 §2's 0x02 final-record padding delimiter.
	if len(opened) == 0 || opened[len(opened)-1] != 0x02 {
		t.Fatalf("expected trailing 0x02 padding delimiter, got %x", opened)
	}
	got := opened[:len(opened)-1]
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("round-trip mismatch: got %q, want %q", got, plaintext)
	}
}

func TestFramePlaintext_RoundTripsNonceFlag(t *testing.T) {
	withNonce := framePlaintext([]byte("cipher"), []byte("123456789012345678901234"))
	if withNonce[0] != 0x01 {
		t.Fatalf("expected flag byte 0x01 when nonce present, got %x", withNonce[0])
	}
	gotLen := binary.BigEndian.Uint32(withNonce[1:5])
	if gotLen != 24 {
		t.Fatalf("expected encoded nonce length 24, got %d", gotLen)
	}

	noNonce := framePlaintext([]byte("cipher"), nil)
	if noNonce[0] != 0x00 {
		t.Fatalf("expected flag byte 0x00 when nonce absent, got %x", noNonce[0])
	}
}

// testSubscriptionKeys generates a real ECDH P-256 keypair's public key and
// a random auth secret, both base64url-encoded — the shape
// encryptAES128GCM requires, matching how a real browser's
// PushSubscription.toJSON() keys look (see TestEncryptAES128GCM_RoundTrips).
// Send() encrypts before ever reaching the HTTP call, so a Send()-level
// test needs real key material to exercise the HTTP status handling at all.
func testSubscriptionKeys(t *testing.T) (p256dh, auth string) {
	t.Helper()
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating subscriber key: %v", err)
	}
	authSecret := make([]byte, 16)
	if _, err := rand.Read(authSecret); err != nil {
		t.Fatalf("generating auth secret: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(priv.PublicKey().Bytes()), base64.RawURLEncoding.EncodeToString(authSecret)
}

func TestSend_WrapsErrDeviceTokenInvalid_OnGoneAndNotFound(t *testing.T) {
	p256dh, auth := testSubscriptionKeys(t)
	for _, status := range []int{http.StatusGone, http.StatusNotFound} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		}))
		c := New()
		err := c.Send(context.Background(), srv.URL, p256dh, auth, []byte("cipher"), nil, "vapid-jwt")
		srv.Close()
		if err == nil {
			t.Fatalf("status %d: expected an error", status)
		}
		if !errors.Is(err, usecase.ErrDeviceTokenInvalid) {
			t.Errorf("status %d: expected errors.Is(err, usecase.ErrDeviceTokenInvalid), got: %v", status, err)
		}
	}
}

func TestSend_DoesNotWrapErrDeviceTokenInvalid_On5xx(t *testing.T) {
	p256dh, auth := testSubscriptionKeys(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := New()
	err := c.Send(context.Background(), srv.URL, p256dh, auth, []byte("cipher"), nil, "vapid-jwt")
	if err == nil {
		t.Fatal("expected an error for a 503 response")
	}
	if errors.Is(err, usecase.ErrDeviceTokenInvalid) {
		t.Errorf("expected a transient 503 NOT to be classified as ErrDeviceTokenInvalid, got: %v", err)
	}
}
