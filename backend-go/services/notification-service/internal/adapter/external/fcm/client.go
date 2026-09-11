// Package fcm implements usecase.FCMClient (defined in
// internal/usecase/deliver_push.go) — an OAuth2 service-account token (own
// credential, NOT VAPID) obtained via Vault Transit-mediated signing of the
// service-account JWT assertion (RFC 7523), then a standard FCM HTTP v1
// send call. Delivering a live push needs a real Firebase service-account
// key provisioned in Vault Transit under fcmServiceAccountKeyName — an
// operational/product prerequisite outside this package's scope.
package fcm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/stablyai/orca-go/services/notification-service/internal/usecase"
)

// fcmServiceAccountKeyName is the Vault Transit RSA key backing the FCM
// service-account JWT assertion — provisioning it with a real Firebase
// service-account key is an operational prerequisite, not this package's
// job.
const fcmServiceAccountKeyName = "fcm-service-account-key"

// TransitSigner mirrors apns.TransitSigner — *secrets.Client satisfies
// this directly.
type TransitSigner interface {
	TransitSign(ctx context.Context, keyName string, input []byte) (string, error)
}

type Config struct {
	ProjectID           string // Firebase project id
	ServiceAccountEmail string // service account's client_email (JWT "iss"/"sub")
	TokenURL            string // https://oauth2.googleapis.com/token
}

// Client implements usecase.FCMClient.
type Client struct {
	signer     TransitSigner
	httpClient *http.Client
	cfg        Config
}

func New(signer TransitSigner, cfg Config) *Client {
	if cfg.TokenURL == "" {
		cfg.TokenURL = "https://oauth2.googleapis.com/token"
	}
	return &Client{signer: signer, httpClient: &http.Client{Timeout: 10 * time.Second}, cfg: cfg}
}

type fcmMessage struct {
	Message fcmMessageBody `json:"message"`
}

type fcmMessageBody struct {
	Token string            `json:"token"`
	Data  map[string]string `json:"data"`
}

// Send exchanges a Transit-signed service-account JWT assertion for an
// OAuth2 access token, then POSTs to FCM's HTTP v1 send endpoint. Like
// apns.Client.Send, the mobile app decrypts client-side — this process
// never has the plaintext body for an Android device (BR-MB-05).
func (c *Client) Send(ctx context.Context, registrationToken string, ciphertext, nonce []byte) error {
	if c.cfg.ProjectID == "" || c.cfg.ServiceAccountEmail == "" {
		return fmt.Errorf("fcm: not configured (FCM_PROJECT_ID/FCM_SERVICE_ACCOUNT_EMAIL unset) — cannot send")
	}
	accessToken, err := c.oauthAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("fcm: obtaining oauth2 access token: %w", err)
	}
	body, err := json.Marshal(fcmMessage{Message: fcmMessageBody{
		Token: registrationToken,
		Data: map[string]string{
			"ciphertext": base64.StdEncoding.EncodeToString(ciphertext),
			"nonce":      base64.StdEncoding.EncodeToString(nonce),
		},
	}})
	if err != nil {
		return fmt.Errorf("fcm: marshaling message: %w", err)
	}
	sendURL := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", c.cfg.ProjectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sendURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("fcm: building request: %w", err)
	}
	req.Header.Set("authorization", "Bearer "+accessToken)
	req.Header.Set("content-type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fcm: request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		// UNREGISTERED (404) means the token is permanently invalid — wrap
		// usecase.ErrDeviceTokenInvalid so DeliverPush marks the
		// subscription expired instead of buffering/retrying forever
		// (TASK-MB-02-08's error-classification note, CR-MOBILE-001's
		// acceptance criteria). UNAVAILABLE/QUOTA_EXCEEDED (5xx/429) is
		// transient and left unwrapped, flowing through DeliverPush's
		// normal buffer-and-retry path.
		if isTokenInvalid(resp.StatusCode, respBody) {
			return fmt.Errorf("fcm: send returned %d: %s: %w", resp.StatusCode, string(respBody), usecase.ErrDeviceTokenInvalid)
		}
		return fmt.Errorf("fcm: send returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// fcmErrorBody is FCM HTTP v1's documented error shape (google.rpc.Status):
// {"error":{"code":404,"message":"...","status":"UNREGISTERED",...}}.
type fcmErrorBody struct {
	Error struct {
		Status string `json:"status"`
	} `json:"error"`
}

// isTokenInvalid reports whether an FCM HTTP v1 error response means the
// registration token is permanently dead. HTTP 404 on this endpoint means
// UNREGISTERED per FCM's docs even when the body doesn't parse (a
// malformed/empty error body is not reason to treat a documented-404-only
// condition as retriable), but a parseable body with a different status
// (e.g. a future API revision reusing 404 for something else) is trusted
// over the bare status code.
func isTokenInvalid(statusCode int, body []byte) bool {
	if statusCode != http.StatusNotFound {
		return false
	}
	var parsed fcmErrorBody
	if err := json.Unmarshal(body, &parsed); err != nil || parsed.Error.Status == "" {
		return true
	}
	return parsed.Error.Status == "UNREGISTERED"
}

// oauthAccessToken builds a Transit-signed RS256 JWT assertion (RFC 7523)
// and exchanges it for a short-lived OAuth2 bearer token via Google's token
// endpoint — the standard service-account, no-user-consent flow.
func (c *Client) oauthAccessToken(ctx context.Context) (string, error) {
	now := time.Now()
	header := map[string]string{"alg": "RS256", "typ": "JWT"}
	claims := map[string]any{
		"iss":   c.cfg.ServiceAccountEmail,
		"sub":   c.cfg.ServiceAccountEmail,
		"scope": "https://www.googleapis.com/auth/firebase.messaging",
		"aud":   c.cfg.TokenURL,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)

	wireSig, err := c.signer.TransitSign(ctx, fcmServiceAccountKeyName, []byte(signingInput))
	if err != nil {
		return "", fmt.Errorf("vault transit sign: %w", err)
	}
	// RSA (RS256) signatures from Vault Transit are the raw PKCS#1 v1.5
	// signature bytes directly — unlike ECDSA (see apns.asn1ECDSAToJWS),
	// there is no DER/JWS marshaling mismatch to correct here.
	rawSig, err := decodeVaultSignature(wireSig)
	if err != nil {
		return "", err
	}
	assertion := signingInput + "." + base64.RawURLEncoding.EncodeToString(rawSig)

	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", assertion)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decoding token response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("token endpoint returned no access_token (error=%q, status=%d)", tokenResp.Error, resp.StatusCode)
	}
	return tokenResp.AccessToken, nil
}

func decodeVaultSignature(wire string) ([]byte, error) {
	parts := strings.SplitN(wire, ":", 3)
	if len(parts) != 3 || parts[0] != "vault" {
		return nil, fmt.Errorf("fcm: unexpected vault signature format %q", wire)
	}
	return base64.StdEncoding.DecodeString(parts[2])
}
