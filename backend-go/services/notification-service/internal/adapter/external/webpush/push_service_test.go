package webpush

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/hkdf"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
	"github.com/stablyai/orca-go/services/notification-service/internal/usecase"
)

// vaultLikeSigner mimics Vault Transit: ECDSA P-256/SHA-256, ASN.1 DER,
// "vault:v1:" + std base64.
type vaultLikeSigner struct {
	key   *ecdsa.PrivateKey
	calls atomic.Int64
}

func (v *vaultLikeSigner) SignVapidPayload(_ context.Context, _ string, payload []byte) (string, error) {
	v.calls.Add(1)
	h := sha256.Sum256(payload)
	der, err := ecdsa.SignASN1(rand.Reader, v.key, h[:])
	if err != nil {
		return "", err
	}
	return "vault:v1:" + base64.StdEncoding.EncodeToString(der), nil
}

type fixedKeys struct{ pub string }

func (f fixedKeys) GetPublicKey(context.Context, string) (domain.VapidKeyMetadata, error) {
	return domain.VapidKeyMetadata{PublicKey: f.pub}, nil
}

func newVapid(t *testing.T) (*VapidAuthorizer, *vaultLikeSigner, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ecdhPub, _ := key.PublicKey.ECDH()
	signer := &vaultLikeSigner{key: key}
	a := NewVapidAuthorizer(signer, fixedKeys{pub: base64.RawURLEncoding.EncodeToString(ecdhPub.Bytes())}, "mailto:ops@example.com")
	return a, signer, key
}

func TestVapidAuthorization_FormatAndSignatureVerify(t *testing.T) {
	a, _, key := newVapid(t)
	hdr, err := a.Authorization(context.Background(), "t1", "https://push.example.com/wpush/v2/SECRETTOKEN")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hdr, "vapid t=") || !strings.Contains(hdr, ", k=") {
		t.Fatalf("bad header shape: %q", hdr)
	}
	tPart, kPart, _ := strings.Cut(strings.TrimPrefix(hdr, "vapid t="), ", k=")
	parts := strings.Split(tPart, ".")
	if len(parts) != 3 {
		t.Fatalf("jwt parts = %d", len(parts))
	}
	var claims struct {
		Aud string `json:"aud"`
		Sub string `json:"sub"`
		Exp int64  `json:"exp"`
	}
	raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
	if err := json.Unmarshal(raw, &claims); err != nil {
		t.Fatal(err)
	}
	if claims.Aud != "https://push.example.com" || claims.Sub != "mailto:ops@example.com" {
		t.Errorf("claims = %+v", claims)
	}
	if d := time.Until(time.Unix(claims.Exp, 0)); d <= 0 || d > 24*time.Hour {
		t.Errorf("exp must be within 24h (RFC 8292), got %v", d)
	}
	hdrJSON, _ := base64.RawURLEncoding.DecodeString(parts[0])
	if string(hdrJSON) != `{"typ":"JWT","alg":"ES256"}` {
		t.Errorf("jwt header = %s", hdrJSON)
	}
	// ES256 requires raw 64-byte R||S, not DER.
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(sig) != 64 {
		t.Fatalf("signature must be 64 raw bytes, got %d (%v)", len(sig), err)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if !ecdsa.Verify(&key.PublicKey, digest[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])) {
		t.Fatal("JWT signature does not verify against the VAPID public key")
	}
	pubRaw, err := base64.RawURLEncoding.DecodeString(kPart)
	if err != nil || len(pubRaw) != 65 || pubRaw[0] != 4 {
		t.Errorf("k must be base64url uncompressed P-256 point, got %q", kPart)
	}
}

func TestVapidAuthorization_CachesPerAudience(t *testing.T) {
	a, signer, _ := newVapid(t)
	for i := 0; i < 3; i++ {
		if _, err := a.Authorization(context.Background(), "t1", "https://push.example.com/a/"+string(rune('a'+i))); err != nil {
			t.Fatal(err)
		}
	}
	if signer.calls.Load() != 1 {
		t.Errorf("vault sign calls = %d, want 1 for same tenant+audience", signer.calls.Load())
	}
	if _, err := a.Authorization(context.Background(), "t1", "https://other.example.com/x"); err != nil {
		t.Fatal(err)
	}
	if signer.calls.Load() != 2 {
		t.Errorf("a different audience needs its own JWT, calls = %d", signer.calls.Load())
	}
}

func TestVapidAuthorization_Failures(t *testing.T) {
	a, _, _ := newVapid(t)
	if _, err := a.Authorization(context.Background(), "t1", "not a url"); err == nil {
		t.Error("expected error for invalid endpoint")
	}
	a.subject = ""
	if _, err := a.Authorization(context.Background(), "t1", "https://p.example/x"); err == nil {
		t.Error("expected error when subject is unset")
	}
}

func TestJWSSignature_RejectsGarbageAndAcceptsRaw(t *testing.T) {
	if _, err := jwsSignature("vault:v1:%%%"); err == nil {
		t.Error("expected base64 error")
	}
	if _, err := jwsSignature("vault:v1:" + base64.StdEncoding.EncodeToString([]byte("short"))); err == nil {
		t.Error("expected non-DER error")
	}
	raw := make([]byte, 64)
	if got, err := jwsSignature("vault:v1:" + base64.StdEncoding.EncodeToString(raw)); err != nil || got != base64.RawURLEncoding.EncodeToString(raw) {
		t.Errorf("raw passthrough failed: %v", err)
	}
	// R with the high bit set needs a DER leading zero; must still be 32 bytes.
	r := new(big.Int).SetBytes(append([]byte{0xff}, make([]byte, 31)...))
	der, _ := asn1.Marshal(struct{ R, S *big.Int }{r, big.NewInt(1)})
	got, err := jwsSignature("vault:v1:" + base64.StdEncoding.EncodeToString(der))
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := base64.RawURLEncoding.DecodeString(got); len(b) != 64 || b[0] != 0xff || b[63] != 1 {
		t.Errorf("padding wrong: %x", b)
	}
}

// decryptAES128GCM is the browser side of RFC 8291 for the push-service fake.
func decryptAES128GCM(t *testing.T, body []byte, uaPriv *ecdh.PrivateKey, authSecret []byte) []byte {
	t.Helper()
	salt := body[:16]
	idlen := int(body[20])
	asPubRaw := body[21 : 21+idlen]
	record := body[21+idlen:]
	asPub, err := ecdh.P256().NewPublicKey(asPubRaw)
	if err != nil {
		t.Fatal(err)
	}
	secret, _ := uaPriv.ECDH(asPub)
	info := append([]byte("WebPush: info\x00"), uaPriv.PublicKey().Bytes()...)
	info = append(info, asPubRaw...)
	ikm := make([]byte, 32)
	_, _ = io.ReadFull(hkdf.New(sha256.New, secret, authSecret, info), ikm)
	cek := make([]byte, 16)
	_, _ = io.ReadFull(hkdf.New(sha256.New, ikm, salt, []byte("Content-Encoding: aes128gcm\x00")), cek)
	nonce := make([]byte, 12)
	_, _ = io.ReadFull(hkdf.New(sha256.New, ikm, salt, []byte("Content-Encoding: nonce\x00")), nonce)
	block, _ := aes.NewCipher(cek)
	gcm, _ := cipher.NewGCM(block)
	pt, err := gcm.Open(nil, nonce, record, nil)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	return pt[:len(pt)-1]
}

func TestSend_PushServiceSeesEncryptedBodyAndRequiredHeaders(t *testing.T) {
	uaPriv, _ := ecdh.P256().GenerateKey(rand.Reader)
	authSecret := make([]byte, 16)
	_, _ = rand.Read(authSecret)

	var gotHeaders http.Header
	var gotBody []byte
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders, gotMethod = r.Header.Clone(), r.Method
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	a, _, _ := newVapid(t)
	hdr, err := a.Authorization(context.Background(), "t1", srv.URL+"/push/abc")
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"title":"Approval needed","body":"x","deepLink":"/","tag":"mcp.approval:1"}`)
	err = New().Send(context.Background(), srv.URL+"/push/abc",
		base64.RawURLEncoding.EncodeToString(uaPriv.PublicKey().Bytes()), base64.RawURLEncoding.EncodeToString(authSecret),
		payload, nil, hdr, usecase.WebPushOptions{TTLSeconds: 600, Urgency: "high"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s", gotMethod)
	}
	for k, want := range map[string]string{"Content-Encoding": "aes128gcm", "Ttl": "600", "Urgency": "high", "Content-Type": "application/octet-stream"} {
		if gotHeaders.Get(k) != want {
			t.Errorf("%s = %q, want %q", k, gotHeaders.Get(k), want)
		}
	}
	if auth := gotHeaders.Get("Authorization"); auth != hdr || !strings.HasPrefix(auth, "vapid t=") {
		t.Errorf("Authorization = %q", auth)
	}
	if strings.Contains(string(gotBody), "Approval needed") {
		t.Error("body must be encrypted on the wire")
	}
	if got := decryptAES128GCM(t, gotBody, uaPriv, authSecret); string(got) != string(payload) {
		t.Errorf("decrypted = %s", got)
	}
}

func TestSend_TwoSendsNeverReuseSaltOrEphemeralKey(t *testing.T) {
	uaPriv, _ := ecdh.P256().GenerateKey(rand.Reader)
	p := base64.RawURLEncoding.EncodeToString(uaPriv.PublicKey().Bytes())
	a := base64.RawURLEncoding.EncodeToString(make([]byte, 16))
	e1, _ := encryptAES128GCM([]byte("same"), p, a)
	e2, _ := encryptAES128GCM([]byte("same"), p, a)
	if string(e1[:16]) == string(e2[:16]) || string(e1[21:86]) == string(e2[21:86]) {
		t.Error("salt / ephemeral public key reused between sends")
	}
}

func TestSend_ErrorDoesNotLeakEndpointURL(t *testing.T) {
	uaPriv, _ := ecdh.P256().GenerateKey(rand.Reader)
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL + "/push/SECRETTOKEN"
	srv.Close() // connection refused
	err := New().Send(context.Background(), url,
		base64.RawURLEncoding.EncodeToString(uaPriv.PublicKey().Bytes()), base64.RawURLEncoding.EncodeToString(make([]byte, 16)),
		[]byte("x"), nil, "vapid t=x", usecase.WebPushOptions{})
	if err == nil || strings.Contains(err.Error(), "SECRETTOKEN") {
		t.Fatalf("error must exist and not contain the endpoint: %v", err)
	}
}
