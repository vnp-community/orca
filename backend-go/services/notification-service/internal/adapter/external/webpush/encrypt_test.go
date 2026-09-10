package webpush

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"testing"
)

// testSubscriberKeys generates a fresh P-256 "UA" (subscriber) keypair and
// a random auth secret — the receiver-side material a real browser would
// hold. Returns the base64url-encoded p256dh/auth values encrypt() takes,
// plus the private key needed to decrypt in this test.
func testSubscriberKeys(t *testing.T) (uaPrivate *ecdh.PrivateKey, p256dhKeyB64, authKeyB64 string) {
	t.Helper()
	curve := ecdh.P256()
	priv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate subscriber keypair: %v", err)
	}
	authSecret := make([]byte, 16)
	if _, err := rand.Read(authSecret); err != nil {
		t.Fatalf("generate auth secret: %v", err)
	}
	return priv, base64.RawURLEncoding.EncodeToString(priv.PublicKey().Bytes()), base64.RawURLEncoding.EncodeToString(authSecret)
}

// decryptForTest is an independent RFC 8291/8188 decrypt, implemented from
// the RECEIVER's perspective (ua_private + the message's own header, not
// reusing encrypt()'s ephemeral key) — this is what proves encrypt()'s
// output is actually correct per the RFC, not just "some ciphertext that
// happens to match whatever encrypt() itself would produce."
func decryptForTest(t *testing.T, msg []byte, uaPrivate *ecdh.PrivateKey, authKeyB64 string) []byte {
	t.Helper()
	if len(msg) < 21 {
		t.Fatalf("message too short to contain aes128gcm header: %d bytes", len(msg))
	}
	salt := msg[0:16]
	recordSize := binary.BigEndian.Uint32(msg[16:20])
	keyIDLen := int(msg[20])
	if len(msg) < 21+keyIDLen {
		t.Fatalf("message too short for keyid of length %d", keyIDLen)
	}
	asPublicBytes := msg[21 : 21+keyIDLen]
	ciphertext := msg[21+keyIDLen:]
	if uint32(len(ciphertext)) != recordSize {
		t.Fatalf("record_size header (%d) does not match actual ciphertext length (%d)", recordSize, len(ciphertext))
	}

	authSecret, err := base64.RawURLEncoding.DecodeString(authKeyB64)
	if err != nil {
		t.Fatalf("decode auth key: %v", err)
	}

	curve := ecdh.P256()
	asPublic, err := curve.NewPublicKey(asPublicBytes)
	if err != nil {
		t.Fatalf("parse as_public from header: %v", err)
	}
	ecdhSecret, err := uaPrivate.ECDH(asPublic)
	if err != nil {
		t.Fatalf("ecdh: %v", err)
	}

	uaPublicBytes := uaPrivate.PublicKey().Bytes()
	keyInfo := make([]byte, 0, len("WebPush: info")+1+len(uaPublicBytes)+len(asPublicBytes))
	keyInfo = append(keyInfo, "WebPush: info"...)
	keyInfo = append(keyInfo, 0x00)
	keyInfo = append(keyInfo, uaPublicBytes...)
	keyInfo = append(keyInfo, asPublicBytes...)
	prkKey := hkdfExtract(authSecret, ecdhSecret)
	ikm := hkdfExpand(prkKey, keyInfo, 32)

	prk := hkdfExtract(salt, ikm)
	cekInfo := append([]byte("Content-Encoding: aes128gcm"), 0x00)
	nonceInfo := append([]byte("Content-Encoding: nonce"), 0x00)
	cek := hkdfExpand(prk, cekInfo, 16)
	nonce := hkdfExpand(prk, nonceInfo, 12)

	block, err := aes.NewCipher(cek)
	if err != nil {
		t.Fatalf("aes cipher: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("gcm: %v", err)
	}
	padded, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		t.Fatalf("gcm open (decryption failed — this is exactly what a real push service/browser would see if encrypt() were wrong): %v", err)
	}
	if len(padded) == 0 || padded[len(padded)-1] != aes128gcmRecordPaddingDelimiter {
		t.Fatalf("expected trailing 0x02 last-record delimiter, got %x", padded)
	}
	return padded[:len(padded)-1]
}

func TestEncrypt_RoundTripDecryptsCorrectly(t *testing.T) {
	uaPrivate, p256dhKeyB64, authKeyB64 := testSubscriberKeys(t)
	plaintext := []byte(`{"title":"New task","body":"assigned to you"}`)

	msg, err := encrypt(plaintext, p256dhKeyB64, authKeyB64)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	got := decryptForTest(t, msg, uaPrivate, authKeyB64)
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("round-trip mismatch: got %q, want %q", got, plaintext)
	}
}

func TestEncrypt_DifferentSaltEachCall(t *testing.T) {
	_, p256dhKeyB64, authKeyB64 := testSubscriberKeys(t)
	plaintext := []byte("same payload, same keys")

	msg1, err := encrypt(plaintext, p256dhKeyB64, authKeyB64)
	if err != nil {
		t.Fatalf("first encrypt: %v", err)
	}
	msg2, err := encrypt(plaintext, p256dhKeyB64, authKeyB64)
	if err != nil {
		t.Fatalf("second encrypt: %v", err)
	}

	if bytes.Equal(msg1, msg2) {
		t.Fatal("two calls with identical payload+keys produced identical ciphertext — salt/ephemeral key is being reused, a serious cryptographic weakness")
	}
	// The salt is the header's first 16 bytes — assert it specifically
	// differs, not just "the whole message differs" (which the ephemeral
	// keypair alone would already guarantee).
	if bytes.Equal(msg1[:16], msg2[:16]) {
		t.Fatal("salt (first 16 bytes of the header) is identical across calls — must be freshly random every send")
	}
}

func TestEncrypt_InvalidP256dhKeyReturnsError(t *testing.T) {
	_, _, authKeyB64 := testSubscriberKeys(t)
	if _, err := encrypt([]byte("x"), "not-valid-base64url-!!!", authKeyB64); err == nil {
		t.Fatal("expected an error for an invalid p256dh key")
	}
}

func TestEncrypt_InvalidAuthKeyReturnsError(t *testing.T) {
	_, p256dhKeyB64, _ := testSubscriberKeys(t)
	if _, err := encrypt([]byte("x"), p256dhKeyB64, "not-valid-base64url-!!!"); err == nil {
		t.Fatal("expected an error for an invalid auth key")
	}
}
