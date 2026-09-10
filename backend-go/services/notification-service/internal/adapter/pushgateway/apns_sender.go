// Package pushgateway implements usecase.PushSender against APNs/FCM —
// notification-service.md §6's deliver_push.go design, adapter half.
// Credential material never touches this package's own storage: every
// Send call resolves fresh via usecase.PushCredentialResolver
// (internal/adapter/credentialbroker), per architecture/06's
// "credential-broker-service mediates all tenant secret material" rule.
package pushgateway

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
	"github.com/stablyai/orca-go/services/notification-service/internal/usecase"
)

// apnsCredential is the shape this package JSON-decodes
// PushCredentialResolver's opaque blob into for owner_id "apns" —
// TASK-BE-MOBILE-004's port doc comment names this contract; the writer
// side (whatever admin flow populates credential-broker-service for
// owner_id "apns") is out of scope for this task.
type apnsCredential struct {
	TeamID        string `json:"team_id"`
	KeyID         string `json:"key_id"`
	PrivateKeyPEM string `json:"private_key_pem"`
}

const apnsProductionBaseURL = "https://api.push.apple.com"

type APNsSender struct {
	credentials usecase.PushCredentialResolver
	httpClient  *http.Client // constructed by caller with an http2.Transport
	bundleID    string       // APNs apns-topic header — mobile/app.json's ios.bundleIdentifier ("com.stably.orca.mobile")
	// apnsBaseURL overrides apnsProductionBaseURL — test-only (set directly
	// on the struct after NewAPNsSender, unexported so no production caller
	// can point this at anything but Apple's real endpoint).
	apnsBaseURL string
}

func NewAPNsSender(credentials usecase.PushCredentialResolver, httpClient *http.Client, bundleID string) *APNsSender {
	return &APNsSender{credentials: credentials, httpClient: httpClient, bundleID: bundleID, apnsBaseURL: apnsProductionBaseURL}
}

var _ usecase.PushSender = (*APNsSender)(nil)

func (s *APNsSender) Send(ctx context.Context, sub domain.PushSubscription, event domain.NotificationEvent) error {
	raw, err := s.credentials.Resolve(ctx, sub.TenantID, "apns")
	if err != nil {
		return fmt.Errorf("pushgateway: resolving apns credential: %w", err)
	}
	var cred apnsCredential
	if err := json.Unmarshal(raw, &cred); err != nil {
		return fmt.Errorf("pushgateway: decoding apns credential: %w", err)
	}

	token, err := signAPNsProviderToken(cred)
	if err != nil {
		return fmt.Errorf("pushgateway: signing apns provider token: %w", err)
	}

	payload, err := json.Marshal(map[string]any{
		"aps": map[string]any{"alert": map[string]string{"title": event.Title, "body": event.Body}},
	})
	if err != nil {
		return fmt.Errorf("pushgateway: encoding apns payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.apnsBaseURL+"/3/device/"+sub.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("pushgateway: building apns request: %w", err)
	}
	req.Header.Set("authorization", "bearer "+token)
	req.Header.Set("apns-topic", s.bundleID)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("pushgateway: apns request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusGone { // 410 == BadDeviceToken
		return usecase.ErrDeviceTokenInvalid
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pushgateway: apns returned %d: %s", resp.StatusCode, body)
	}
	return nil
}

// signAPNsProviderToken builds the ES256 JWT APNs requires on the
// authorization header — stdlib crypto/ecdsa only, no new JWT dependency
// (see TASK-BE-MOBILE-005's "quyết định khoá trước khi code": this repo
// has 0 references to golang-jwt/jwt, and hand-rolling header.payload.sig
// for exactly 1 fixed claim set is simpler than adding a dependency for
// it).
func signAPNsProviderToken(cred apnsCredential) (string, error) {
	block, _ := pem.Decode([]byte(cred.PrivateKeyPEM))
	if block == nil {
		return "", fmt.Errorf("pushgateway: apns private_key_pem is not valid PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("pushgateway: parsing apns private key: %w", err)
	}
	priv, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("pushgateway: apns private key is not an ECDSA key (got %T)", parsed)
	}

	header := base64URL([]byte(fmt.Sprintf(`{"alg":"ES256","kid":%q}`, cred.KeyID)))
	claims := base64URL([]byte(fmt.Sprintf(`{"iss":%q,"iat":%d}`, cred.TeamID, time.Now().Unix())))
	signingInput := header + "." + claims

	hash := sha256.Sum256([]byte(signingInput))
	der, err := ecdsa.SignASN1(rand.Reader, priv, hash[:])
	if err != nil {
		return "", fmt.Errorf("pushgateway: signing apns jwt: %w", err)
	}
	rawSig, err := derECDSASignatureToRawJWS(der)
	if err != nil {
		return "", err
	}

	return signingInput + "." + base64URL(rawSig), nil
}

func base64URL(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}
