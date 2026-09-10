package pushgateway

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
	"github.com/stablyai/orca-go/services/notification-service/internal/usecase"
)

func timeNow() time.Time { return time.Now() }

func decodeSegment(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

func sha256Sum(b []byte) []byte {
	h := sha256.Sum256(b)
	return h[:]
}

// fakePushCredentialResolver is a minimal in-memory
// usecase.PushCredentialResolver double — no real credential-broker-service
// connection, matching this package's other fakes' "test against fakes"
// convention.
type fakePushCredentialResolver struct {
	blob []byte
	err  error
}

func (f *fakePushCredentialResolver) Resolve(ctx context.Context, tenantID, ownerID string) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.blob, nil
}

// testAPNsCredential generates a real ECDSA P-256 key (APNs auth keys are
// always P-256/ES256), PKCS8-PEM-encodes it (the shape apnsCredential's
// private_key_pem field expects), and returns both the credential blob and
// the public key for signature verification in tests.
func testAPNsCredential(t *testing.T) ([]byte, *ecdsa.PublicKey) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal pkcs8: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	cred := apnsCredential{TeamID: "TEAM123", KeyID: "KEY456", PrivateKeyPEM: string(pemBytes)}
	blob, err := json.Marshal(cred)
	if err != nil {
		t.Fatalf("marshal credential: %v", err)
	}
	return blob, &priv.PublicKey
}

func testWebSubscription(t *testing.T, endpoint string) domain.PushSubscription {
	t.Helper()
	sub, err := domain.NewPushSubscription("s1", "tenant-1", "user-1", domain.ChannelIOS, endpoint, nil, nil, "", timeNow())
	if err != nil {
		t.Fatalf("building ios subscription: %v", err)
	}
	return sub
}

func TestAPNsSender_Send_Success200(t *testing.T) {
	blob, _ := testAPNsCredential(t)
	var gotAuth, gotTopic string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("authorization")
		gotTopic = r.Header.Get("apns-topic")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender := NewAPNsSender(&fakePushCredentialResolver{blob: blob}, srv.Client(), "com.stably.orca.mobile")
	sender.apnsBaseURL = srv.URL // test-only override, see field doc comment

	sub := testWebSubscription(t, "device-token-1")
	event := domain.NotificationEvent{Title: "t", Body: "b"}
	if err := sender.Send(context.Background(), sub, event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(gotAuth, "bearer ") {
		t.Fatalf("expected authorization header to start with 'bearer ', got %q", gotAuth)
	}
	if gotTopic != "com.stably.orca.mobile" {
		t.Fatalf("apns-topic = %q, want %q", gotTopic, "com.stably.orca.mobile")
	}
}

func TestAPNsSender_Send_410ReturnsErrDeviceTokenInvalid(t *testing.T) {
	blob, _ := testAPNsCredential(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
	}))
	defer srv.Close()

	sender := NewAPNsSender(&fakePushCredentialResolver{blob: blob}, srv.Client(), "com.stably.orca.mobile")
	sender.apnsBaseURL = srv.URL

	err := sender.Send(context.Background(), testWebSubscription(t, "device-token-1"), domain.NotificationEvent{})
	if !errors.Is(err, usecase.ErrDeviceTokenInvalid) {
		t.Fatalf("expected usecase.ErrDeviceTokenInvalid, got %v", err)
	}
}

func TestAPNsSender_Send_OtherErrorStatusIsGenericError(t *testing.T) {
	blob, _ := testAPNsCredential(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	sender := NewAPNsSender(&fakePushCredentialResolver{blob: blob}, srv.Client(), "com.stably.orca.mobile")
	sender.apnsBaseURL = srv.URL

	err := sender.Send(context.Background(), testWebSubscription(t, "device-token-1"), domain.NotificationEvent{})
	if err == nil {
		t.Fatal("expected an error for a 503 response")
	}
	if errors.Is(err, usecase.ErrDeviceTokenInvalid) {
		t.Fatal("a 503 must NOT be classified as ErrDeviceTokenInvalid — DeliverPush would wrongly mark the subscription expired")
	}
}

func TestAPNsSender_Send_CredentialResolveFailurePropagates(t *testing.T) {
	sender := NewAPNsSender(&fakePushCredentialResolver{err: errors.New("broker unavailable")}, http.DefaultClient, "com.stably.orca.mobile")
	err := sender.Send(context.Background(), testWebSubscription(t, "device-token-1"), domain.NotificationEvent{})
	if err == nil {
		t.Fatal("expected the credential resolve failure to propagate")
	}
}

func TestSignAPNsProviderToken_ProducesValidES256JWT(t *testing.T) {
	blob, pub := testAPNsCredential(t)
	var cred apnsCredential
	if err := json.Unmarshal(blob, &cred); err != nil {
		t.Fatalf("unmarshal credential: %v", err)
	}

	token, err := signAPNsProviderToken(cred)
	if err != nil {
		t.Fatalf("signAPNsProviderToken: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected a 3-segment JWT, got %d segments", len(parts))
	}
	sig, err := decodeSegment(parts[2])
	if err != nil {
		t.Fatalf("decode signature segment: %v", err)
	}
	if len(sig) != 64 {
		t.Fatalf("expected a 64-byte raw r||s signature, got %d bytes", len(sig))
	}
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])

	signingInput := []byte(parts[0] + "." + parts[1])
	hash := sha256Sum(signingInput)
	if !ecdsa.Verify(pub, hash, r, s) {
		t.Fatal("APNs provider token signature does not verify against the credential's public key")
	}
}
