// Package webpush implements usecase.WebPushClient (defined in
// internal/usecase/deliver_push.go) — POSTs to a subscription's Web Push
// endpoint per RFC 8030, encrypting the body per RFC 8291 (aes128gcm
// content-coding, layered over RFC 8188) using the subscription's
// p256dh/auth keys. VAPID auth (the vapidJWT parameter) is signed upstream
// by credential-broker-service (see internal/adapter/vaultsigner) — this
// package only carries it in the Authorization header, never signs
// anything itself.
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
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/crypto/hkdf"
)

// Client implements usecase.WebPushClient.
type Client struct {
	httpClient *http.Client
}

func New() *Client { return &Client{httpClient: &http.Client{Timeout: 10 * time.Second}} }

// defaultTTL is RFC 8030's TTL header (seconds) — 4 weeks, a typical Web
// Push provider default.
const defaultTTL = "2419200"

// Send encrypts body (RFC 8291 aes128gcm, using the subscription's
// p256dh/auth keys) and POSTs it to endpoint with a VAPID Authorization
// header. ciphertext/nonce are whatever DeliverPush handed in — a
// NaCl-sealed E2E payload for a paired mobile-companion subscription
// (nonce non-nil), or a plaintext NotificationEvent JSON blob for a
// standard (non-paired) Web Push subscription (nonce nil); either way,
// THIS function is what actually encrypts it before it leaves the process
// per RFC 8291, satisfying BR-MB-05 regardless of which case produced the
// input bytes.
func (c *Client) Send(ctx context.Context, endpoint, p256dh, auth string, ciphertext, nonce []byte, vapidJWT string) error {
	plaintext := framePlaintext(ciphertext, nonce)

	encoded, err := encryptAES128GCM(plaintext, p256dh, auth)
	if err != nil {
		return fmt.Errorf("webpush: encrypting payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("webpush: building request: %w", err)
	}
	req.Header.Set("content-encoding", "aes128gcm")
	req.Header.Set("content-type", "application/octet-stream")
	req.Header.Set("ttl", defaultTTL)
	req.Header.Set("authorization", "vapid t="+vapidJWT)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("webpush: request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("webpush: endpoint returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// framePlaintext lets the receiving mobile/browser client tell a
// NaCl-sealed E2E payload (nonce present) apart from a plain JSON body
// (nonce nil): 1-byte flag + 4-byte big-endian nonce length + nonce +
// ciphertext.
func framePlaintext(ciphertext, nonce []byte) []byte {
	if len(nonce) == 0 {
		return append([]byte{0x00}, ciphertext...)
	}
	out := make([]byte, 0, 1+4+len(nonce)+len(ciphertext))
	out = append(out, 0x01)
	nonceLen := make([]byte, 4)
	binary.BigEndian.PutUint32(nonceLen, uint32(len(nonce)))
	out = append(out, nonceLen...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)
	return out
}

// encryptAES128GCM implements RFC 8291 §3-4 (which layers RFC 8188's
// aes128gcm content-coding over a one-time ECDH key agreement with the
// subscriber's p256dh key, salted by the subscription's auth secret).
func encryptAES128GCM(plaintext []byte, p256dhB64, authB64 string) ([]byte, error) {
	uaPublicRaw, err := base64.RawURLEncoding.DecodeString(p256dhB64)
	if err != nil {
		return nil, fmt.Errorf("decoding p256dh: %w", err)
	}
	authSecret, err := base64.RawURLEncoding.DecodeString(authB64)
	if err != nil {
		return nil, fmt.Errorf("decoding auth secret: %w", err)
	}

	curve := ecdh.P256()
	uaPublic, err := curve.NewPublicKey(uaPublicRaw)
	if err != nil {
		return nil, fmt.Errorf("parsing subscriber public key: %w", err)
	}
	asPrivate, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating ephemeral key: %w", err)
	}
	asPublicRaw := asPrivate.PublicKey().Bytes() // uncompressed point, 65 bytes

	ecdhSecret, err := asPrivate.ECDH(uaPublic)
	if err != nil {
		return nil, fmt.Errorf("ecdh: %w", err)
	}

	// PRK_key = HKDF-Extract(salt=auth_secret, ikm=ecdh_secret); expand
	// with the "WebPush: info" context binding both public keys — RFC
	// 8291 §3.4.
	keyInfo := append([]byte("WebPush: info\x00"), uaPublicRaw...)
	keyInfo = append(keyInfo, asPublicRaw...)
	ikm := make([]byte, 32)
	if _, err := io.ReadFull(hkdf.New(sha256.New, ecdhSecret, authSecret, keyInfo), ikm); err != nil {
		return nil, fmt.Errorf("deriving ikm: %w", err)
	}

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generating salt: %w", err)
	}

	cek := make([]byte, 16)
	if _, err := io.ReadFull(hkdf.New(sha256.New, ikm, salt, []byte("Content-Encoding: aes128gcm\x00")), cek); err != nil {
		return nil, fmt.Errorf("deriving cek: %w", err)
	}
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(hkdf.New(sha256.New, ikm, salt, []byte("Content-Encoding: nonce\x00")), nonce); err != nil {
		return nil, fmt.Errorf("deriving nonce: %w", err)
	}

	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	// RFC 8188 §2's padding delimiter: 0x02 marks the final (only) record.
	record := gcm.Seal(nil, nonce, append(plaintext, 0x02), nil)

	// RFC 8188 §2 header: salt(16) || record_size(4, big-endian) || idlen(1) || keyid(idlen)
	header := make([]byte, 0, 16+4+1+len(asPublicRaw))
	header = append(header, salt...)
	rs := make([]byte, 4)
	binary.BigEndian.PutUint32(rs, uint32(4096))
	header = append(header, rs...)
	header = append(header, byte(len(asPublicRaw)))
	header = append(header, asPublicRaw...)

	return append(header, record...), nil
}
