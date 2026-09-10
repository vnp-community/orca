// Package apns implements usecase.APNsClient (defined in
// internal/usecase/deliver_push.go) — signs a provider JWT with an ES256
// .p8 key via Vault Transit (own credential, distinct from the web
// channel's VAPID key — notification-service.md's diagram conflates the
// two; this package is what actually keeps them separate, see
// TASK-MB-02-08's Context), then POSTs to Apple's HTTP/2 push gateway.
//
// Real request/response shapes per Apple's APNs HTTP/2 provider API.
// Delivering a live push needs an actual Apple Push Auth Key (.p8)
// provisioned in Vault Transit under apnsSigningKeyName — an
// operational/product prerequisite outside this package's scope; until
// that's provisioned, Send returns a clear error from Vault ("key not
// found") rather than fabricating a signature.
package apns

import (
	"bytes"
	"context"
	"crypto/elliptic"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"
)

// apnsSigningKeyName is the Vault Transit ES256 (ecdsa-p256) key backing
// APNs provider-token auth — provisioning it with a real Apple Push Auth
// Key is an operational prerequisite, not this package's job.
const apnsSigningKeyName = "apns-provider-key"

// TransitSigner is the narrow subset of common/secrets.Client's Transit
// surface this package needs — *secrets.Client satisfies this directly
// (method name/signature match exactly), so no wrapper adapter is needed
// in cmd/server/main.go: the RSA/ECDSA private key never leaves Vault,
// only signatures Vault already computed do.
type TransitSigner interface {
	TransitSign(ctx context.Context, keyName string, input []byte) (string, error)
}

// Config carries the non-secret identifiers APNs's provider-token auth
// needs alongside the Transit-mediated signature — team id / key id /
// bundle id are public identifiers Apple assigns, not secret material;
// only the .p8 private key (never present in this process) is.
type Config struct {
	TeamID   string // Apple Developer Team ID (JWT "iss")
	KeyID    string // APNs Auth Key ID (JWT header "kid")
	Topic    string // app bundle id (apns-topic header)
	Endpoint string // https://api.push.apple.com (prod) or https://api.sandbox.push.apple.com (sandbox)
}

// Client implements usecase.APNsClient.
type Client struct {
	signer     TransitSigner
	httpClient *http.Client
	cfg        Config
}

func New(signer TransitSigner, cfg Config) *Client {
	return &Client{signer: signer, httpClient: &http.Client{Timeout: 10 * time.Second}, cfg: cfg}
}

type apnsPayload struct {
	Ciphertext string `json:"ciphertext"`
	Nonce      string `json:"nonce"`
}

// Send POSTs an E2E-sealed push (BL-MB-02: NaCl-secretbox ciphertext+nonce,
// SOL-MB-01's shared secret) to Apple's HTTP/2 gateway. The mobile app
// decrypts client-side with the same paired-device shared secret — this
// process never has the plaintext notification body for an iOS device
// (BR-MB-05).
func (c *Client) Send(ctx context.Context, deviceToken string, ciphertext, nonce []byte) error {
	if c.cfg.TeamID == "" || c.cfg.KeyID == "" || c.cfg.Topic == "" {
		return fmt.Errorf("apns: not configured (APNS_TEAM_ID/APNS_KEY_ID/APNS_TOPIC unset) — cannot send")
	}
	jwt, err := c.providerToken(ctx)
	if err != nil {
		return fmt.Errorf("apns: signing provider jwt: %w", err)
	}
	body, err := json.Marshal(apnsPayload{
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
	})
	if err != nil {
		return fmt.Errorf("apns: marshaling payload: %w", err)
	}
	url := fmt.Sprintf("%s/3/device/%s", strings.TrimRight(c.cfg.Endpoint, "/"), deviceToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("apns: building request: %w", err)
	}
	req.Header.Set("authorization", "bearer "+jwt)
	req.Header.Set("apns-topic", c.cfg.Topic)
	req.Header.Set("apns-push-type", "alert")
	req.Header.Set("content-type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("apns: request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		// 410 Gone / BadDeviceToken means the token is permanently invalid
		// (the subscription should eventually be pruned by a future
		// cleanup pass) vs. a 5xx/429, which is transient and safe to let
		// DeliverPush's buffering + StreamNotifications reconnect drain
		// retry — see TASK-MB-02-08's error-classification note. Both are
		// surfaced identically as an error here; callers distinguish by
		// inspecting resp.StatusCode's text if they need to.
		return fmt.Errorf("apns: gateway returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// providerToken builds and Transit-signs an ES256 JWT per Apple's
// provider-token auth spec (compact JWS serialization).
func (c *Client) providerToken(ctx context.Context) (string, error) {
	header := map[string]string{"alg": "ES256", "kid": c.cfg.KeyID}
	claims := map[string]any{"iss": c.cfg.TeamID, "iat": time.Now().Unix()}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)

	wireSig, err := c.signer.TransitSign(ctx, apnsSigningKeyName, []byte(signingInput))
	if err != nil {
		return "", fmt.Errorf("vault transit sign: %w", err)
	}
	rawSig, err := decodeVaultSignature(wireSig)
	if err != nil {
		return "", err
	}
	jwsSig, err := asn1ECDSAToJWS(rawSig)
	if err != nil {
		return "", err
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(jwsSig), nil
}

// decodeVaultSignature strips Vault's "vault:v<N>:" wire-format prefix and
// base64-decodes the remainder (common/secrets.Client.TransitSign's own
// doc comment: callers that need raw signature bytes must strip this
// themselves).
func decodeVaultSignature(wire string) ([]byte, error) {
	parts := strings.SplitN(wire, ":", 3)
	if len(parts) != 3 || parts[0] != "vault" {
		return nil, fmt.Errorf("apns: unexpected vault signature format %q", wire)
	}
	return base64.StdEncoding.DecodeString(parts[2])
}

// asn1ECDSAToJWS converts Vault Transit's default ASN.1 DER-encoded ECDSA
// signature into the fixed-length raw r||s format JWS ES256 requires (RFC
// 7518 §3.4). common/secrets.Client.TransitSign doesn't expose a
// marshaling_algorithm=jws option, so this conversion happens client-side.
func asn1ECDSAToJWS(der []byte) ([]byte, error) {
	var sig struct{ R, S *big.Int }
	if _, err := asn1.Unmarshal(der, &sig); err != nil {
		return nil, fmt.Errorf("apns: decoding DER ecdsa signature: %w", err)
	}
	size := (elliptic.P256().Params().BitSize + 7) / 8 // 32 bytes for P-256
	out := make([]byte, 2*size)
	sig.R.FillBytes(out[:size])
	sig.S.FillBytes(out[size:])
	return out, nil
}
