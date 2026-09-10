package usecase

import (
	"context"
	"encoding/asn1"
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// This file builds RFC 8292 VAPID JWT auth headers for DeliverPush
// (deliver_push.go) via the VaultSigner port — it lives in usecase/, not
// internal/adapter/external/webpush/ (where it was first drafted while
// implementing TASK-BE-NOTIF-010), because it only depends on VaultSigner
// (already a usecase-layer port) and stdlib crypto, and moving it into an
// adapter package would force deliver_push.go to import that adapter —
// this codebase's usecase/ never imports internal/adapter/* in
// non-test code (confirmed by repo-wide grep before writing this file);
// every other usecase file's port doc comments describe the same
// direction (port declared here, implemented in internal/adapter/*).
// internal/adapter/external/webpush/sender.go+encrypt.go correctly stay in
// the adapter package — they do real HTTP + RFC 8291 payload encryption,
// genuine adapter I/O, unlike this file's pure JWT construction.

func base64url(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

// asn1ECDSASignature mirrors the ASN.1 SEQUENCE { r INTEGER, s INTEGER }
// shape crypto/ecdsa.SignASN1 produces, and — per Vault Transit's `sign`
// endpoint docs — what it returns by default for an ECDSA key
// (`marshaling_algorithm` defaults to "asn1"; this codebase's
// common/secrets.Client.TransitSign doesn't override it, and is left
// untouched here rather than special-cased for this one caller — see
// derECDSASignatureToRawJWS's doc comment for why).
type asn1ECDSASignature struct {
	R, S *big.Int
}

// ecdsaP256SignatureLen is the byte length of one coordinate (r or s) for
// a P-256 (256-bit curve) ECDSA signature — RFC 8292 §2 requires the JWS
// ES256 signature to be the raw, fixed-length r||s concatenation (64 bytes
// total for P-256), NOT the ASN.1 DER encoding.
const ecdsaP256SignatureLen = 32

// derECDSASignatureToRawJWS converts a Vault Transit `sign` response's
// default ASN.1-DER-encoded ECDSA signature into the raw, fixed-length
// r||s form a JWS ES256 signature requires (RFC 7518 §3.4). Vault's
// TransitSign is a single shared primitive (common/secrets.Client) with no
// other real caller today (confirmed by repo-wide grep when this was
// written) — converting here, at this one VAPID-specific call site, rather
// than adding a `marshaling_algorithm=jws` request parameter to the shared
// TransitSign function, keeps that shared primitive's existing (RSA-JWT-
// shaped, "just base64-decode after stripping the vault:vN: prefix")
// contract untouched for whatever future caller its doc comment was
// written for, instead of silently changing its wire behavior for a
// use case (ECDSA/VAPID) it wasn't necessarily designed around.
func derECDSASignatureToRawJWS(der []byte) ([]byte, error) {
	var sig asn1ECDSASignature
	if _, err := asn1.Unmarshal(der, &sig); err != nil {
		return nil, fmt.Errorf("vapid: parse ASN.1 ECDSA signature: %w", err)
	}
	raw := make([]byte, 2*ecdsaP256SignatureLen)
	sig.R.FillBytes(raw[:ecdsaP256SignatureLen])
	sig.S.FillBytes(raw[ecdsaP256SignatureLen:])
	return raw, nil
}

// stripVaultVersionPrefix removes Vault's "vault:v<N>:" wrapper (see
// common/secrets.Client.TransitSign's doc comment) from a Transit
// sign/encrypt response, returning the base64-std-encoded payload
// underneath it.
func stripVaultVersionPrefix(s string) (string, error) {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) != 3 || parts[0] != "vault" {
		return "", fmt.Errorf("vapid: unexpected vault signature wire format: %q", s)
	}
	return parts[2], nil
}

// BuildVapidAuthHeader builds the RFC 8292 `Authorization: vapid t=..., k=...`
// header value for one Web Push send. Called once per subscription's
// audience (aud claim = the push endpoint's origin) by DeliverPush
// (TASK-BE-NOTIF-011) before calling WebPushSender.Send — kept separate
// from the Sender adapter because building the JWT needs tenantID (to
// call VaultSigner), which the Sender adapter deliberately doesn't hold
// (see internal/adapter/external/webpush/sender.go's package doc comment).
func BuildVapidAuthHeader(ctx context.Context, signer VaultSigner, tenantID, endpointOrigin, subject, publicKeyB64 string) (string, error) {
	header := base64url([]byte(`{"typ":"JWT","alg":"ES256"}`))
	claims := base64url([]byte(fmt.Sprintf(
		`{"aud":%q,"exp":%d,"sub":%q}`,
		endpointOrigin, time.Now().Add(12*time.Hour).Unix(), subject,
	)))
	signingInput := header + "." + claims

	rawSig, err := signer.SignVapidPayload(ctx, tenantID, []byte(signingInput))
	if err != nil {
		return "", fmt.Errorf("vapid: sign vapid jwt: %w", err)
	}
	derB64, err := stripVaultVersionPrefix(rawSig)
	if err != nil {
		return "", err
	}
	der, err := base64.StdEncoding.DecodeString(derB64)
	if err != nil {
		return "", fmt.Errorf("vapid: decode vault signature payload: %w", err)
	}
	rawJWSSig, err := derECDSASignatureToRawJWS(der)
	if err != nil {
		return "", err
	}

	jwt := signingInput + "." + base64url(rawJWSSig)
	return fmt.Sprintf("vapid t=%s, k=%s", jwt, publicKeyB64), nil
}
