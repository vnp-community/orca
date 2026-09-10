package webpush

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
)

// This file implements RFC 8291 (Message Encryption for Web Push) message
// encryption directly against the RFC text (§3.1-3.4) — deliberately not
// reinterpreted or simplified, because a single byte-offset mistake breaks
// the whole scheme silently (it would still "encrypt something", just not
// anything the browser vendor's push service or the receiving service
// worker could decrypt). HKDF (RFC 5869) is implemented by hand with
// stdlib crypto/hmac+crypto/sha256 rather than importing
// golang.org/x/crypto/hkdf, to keep the derivation steps fully explicit
// and auditable against the RFC's numbered steps, and to avoid adding a
// new direct dependency for ~15 lines of well-specified math.

// hkdfExtract implements RFC 5869 §2.2 — HKDF-Extract(salt, ikm) = HMAC-Hash(salt, ikm).
func hkdfExtract(salt, ikm []byte) []byte {
	mac := hmac.New(sha256.New, salt)
	mac.Write(ikm)
	return mac.Sum(nil)
}

// hkdfExpand implements RFC 5869 §2.3 — HKDF-Expand(prk, info, length).
// length must be <= 255*32 (sha256 output size) per the RFC; every call
// site in this file requests well under that, so this doesn't validate it.
func hkdfExpand(prk, info []byte, length int) []byte {
	var t, okm []byte
	for i := byte(1); len(okm) < length; i++ {
		mac := hmac.New(sha256.New, prk)
		mac.Write(t)
		mac.Write(info)
		mac.Write([]byte{i})
		t = mac.Sum(nil)
		okm = append(okm, t...)
	}
	return okm[:length]
}

// aes128gcmRecordPaddingDelimiter is RFC 8188 §2's "last record" delimiter
// byte appended to the plaintext before encryption — Web Push messages are
// always a single record (no multi-record streaming), so this is always
// 0x02 ("last record"), never 0x01 ("more records follow").
const aes128gcmRecordPaddingDelimiter = 0x02

// encryptedPushMessage is what encrypt returns: the RFC 8188 aes128gcm
// wire format (salt || record_size || keyid_length || keyid=as_public ||
// ciphertext), ready to POST as the request body with
// Content-Encoding: aes128gcm.
func encrypt(plaintext []byte, p256dhKeyB64, authKeyB64 string) ([]byte, error) {
	uaPublicBytes, err := base64.RawURLEncoding.DecodeString(p256dhKeyB64)
	if err != nil {
		return nil, fmt.Errorf("webpush: decode p256dh key: %w", err)
	}
	authSecret, err := base64.RawURLEncoding.DecodeString(authKeyB64)
	if err != nil {
		return nil, fmt.Errorf("webpush: decode auth key: %w", err)
	}

	curve := ecdh.P256()
	uaPublic, err := curve.NewPublicKey(uaPublicBytes)
	if err != nil {
		return nil, fmt.Errorf("webpush: parse subscriber p256dh public key: %w", err)
	}

	// §3.1: a fresh, single-use EC keypair every send — reusing this
	// across sends (even to the same subscription) would let 2 messages'
	// ECDH shared secrets be correlated, defeating forward secrecy. This
	// is the single most important invariant this file has to hold; see
	// TestEncrypt_DifferentSaltEachCall.
	asPrivate, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("webpush: generate ephemeral keypair: %w", err)
	}
	asPublicBytes := asPrivate.PublicKey().Bytes() // uncompressed point, 65 bytes for P-256

	ecdhSecret, err := asPrivate.ECDH(uaPublic)
	if err != nil {
		return nil, fmt.Errorf("webpush: ecdh: %w", err)
	}

	// §3.4 "Combining Public Keys": PRK_key = HMAC-SHA-256(auth_secret, ecdh_secret);
	// IKM = HKDF-Expand(PRK_key, key_info, 32), where key_info is
	// "WebPush: info" || 0x00 || ua_public || as_public (exact byte
	// sequence per §3.4 — not "WebPush: info" alone).
	keyInfo := make([]byte, 0, len("WebPush: info")+1+len(uaPublicBytes)+len(asPublicBytes))
	keyInfo = append(keyInfo, "WebPush: info"...)
	keyInfo = append(keyInfo, 0x00)
	keyInfo = append(keyInfo, uaPublicBytes...)
	keyInfo = append(keyInfo, asPublicBytes...)
	prkKey := hkdfExtract(authSecret, ecdhSecret)
	ikm := hkdfExpand(prkKey, keyInfo, 32)

	// RFC 8188 §2.1: this message's own random salt (16 bytes, MUST be
	// unique per message — reusing a salt with the same key would let an
	// attacker forge/replay ciphertext under GCM's nonce-reuse weakness).
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("webpush: generate salt: %w", err)
	}
	prk := hkdfExtract(salt, ikm)
	cekInfo := append([]byte("Content-Encoding: aes128gcm"), 0x00)
	nonceInfo := append([]byte("Content-Encoding: nonce"), 0x00)
	cek := hkdfExpand(prk, cekInfo, 16)
	nonce := hkdfExpand(prk, nonceInfo, 12)

	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, fmt.Errorf("webpush: aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("webpush: gcm: %w", err)
	}

	// RFC 8188 §2: append the (single, last) record's padding delimiter
	// before sealing — this is part of the plaintext GCM authenticates,
	// not a separate wire field.
	padded := append([]byte{}, plaintext...)
	padded = append(padded, aes128gcmRecordPaddingDelimiter)
	ciphertext := gcm.Seal(nil, nonce, padded, nil)

	// RFC 8188 §2 wire header: salt(16) || record_size(4, big-endian) ||
	// keyid_length(1) || keyid(as_public, here 65 bytes) — followed by the
	// ciphertext. record_size is the size of THIS record's ciphertext
	// (Web Push always sends exactly one record, so this equals
	// len(ciphertext)); a decrypting client uses it to know where one
	// record ends and, for multi-record streams, the next begins.
	header := make([]byte, 16+4+1+len(asPublicBytes))
	copy(header[0:16], salt)
	binary.BigEndian.PutUint32(header[16:20], uint32(len(ciphertext)))
	header[20] = byte(len(asPublicBytes))
	copy(header[21:], asPublicBytes)

	return append(header, ciphertext...), nil
}
