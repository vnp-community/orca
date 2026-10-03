package webpush

import (
	"context"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/stablyai/orca-go/services/notification-service/internal/usecase"
)

// vapidJWTLifetime is below RFC 8292's 24h cap; tokens are cached and renewed
// with vapidRenewBefore left so a cached token never expires in flight.
const (
	vapidJWTLifetime = 12 * time.Hour
	vapidRenewBefore = 1 * time.Hour
)

// VapidAuthorizer implements usecase.VapidAuthorizer (RFC 8292).
// Why self-assembled: Go Web Push libraries sign with a local private key,
// but VAPID keys live only in Vault Transit (architecture 06 / TDD section 9).
// Only the JWT signing input is sent to usecase.VaultSigner; the returned
// signature is joined locally.
type VapidAuthorizer struct {
	signer  usecase.VaultSigner
	keys    usecase.VapidKeyRepository
	subject string
	now     func() time.Time

	mu    sync.Mutex
	cache map[string]cachedVapid
}

type cachedVapid struct {
	header  string
	expires time.Time
}

// NewVapidAuthorizer: subject is the RFC 8292 "sub" claim (mailto: or https: URL).
func NewVapidAuthorizer(signer usecase.VaultSigner, keys usecase.VapidKeyRepository, subject string) *VapidAuthorizer {
	return &VapidAuthorizer{signer: signer, keys: keys, subject: subject, now: time.Now, cache: map[string]cachedVapid{}}
}

// Authorization returns "vapid t=<jwt>, k=<public key>" for endpoint's origin.
func (a *VapidAuthorizer) Authorization(ctx context.Context, tenantID, endpoint string) (string, error) {
	if a.subject == "" {
		return "", errors.New("webpush: VAPID_SUBJECT is not configured")
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", errors.New("webpush: invalid push endpoint")
	}
	aud := u.Scheme + "://" + u.Host
	cacheKey := tenantID + "|" + aud

	a.mu.Lock()
	c, ok := a.cache[cacheKey]
	a.mu.Unlock()
	if ok && a.now().Add(vapidRenewBefore).Before(c.expires) {
		return c.header, nil
	}

	key, err := a.keys.GetPublicKey(ctx, tenantID)
	if err != nil {
		return "", fmt.Errorf("webpush: loading vapid public key: %w", err)
	}
	exp := a.now().Add(vapidJWTLifetime)
	claims, _ := json.Marshal(map[string]any{"aud": aud, "exp": exp.Unix(), "sub": a.subject})
	signingInput := base64.RawURLEncoding.EncodeToString([]byte(`{"typ":"JWT","alg":"ES256"}`)) + "." +
		base64.RawURLEncoding.EncodeToString(claims)

	vaultSig, err := a.signer.SignVapidPayload(ctx, tenantID, []byte(signingInput))
	if err != nil {
		return "", fmt.Errorf("webpush: signing vapid jwt: %w", err)
	}
	sig, err := jwsSignature(vaultSig)
	if err != nil {
		return "", err
	}
	header := "vapid t=" + signingInput + "." + sig + ", k=" + key.PublicKey

	a.mu.Lock()
	a.cache[cacheKey] = cachedVapid{header: header, expires: exp}
	a.mu.Unlock()
	return header, nil
}

// jwsSignature converts Vault Transit's "vault:v<N>:<std base64>" ECDSA
// signature to the base64url R||S form JWS ES256 requires. Vault emits ASN.1
// DER by default; a 64-byte value (marshaling_algorithm=jws) is passed through.
func jwsSignature(vaultSig string) (string, error) {
	raw := vaultSig
	if strings.HasPrefix(raw, "vault:") {
		parts := strings.SplitN(raw, ":", 3)
		if len(parts) != 3 {
			return "", errors.New("webpush: malformed vault signature")
		}
		raw = parts[2]
	}
	der, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		if der, err = base64.RawURLEncoding.DecodeString(raw); err != nil {
			return "", errors.New("webpush: vault signature is not base64")
		}
	}
	if len(der) == 64 {
		return base64.RawURLEncoding.EncodeToString(der), nil
	}
	var sig struct{ R, S *big.Int }
	if rest, err := asn1.Unmarshal(der, &sig); err != nil || len(rest) != 0 || sig.R == nil || sig.S == nil {
		return "", errors.New("webpush: vault signature is not a P-256 ECDSA signature")
	}
	if sig.R.Sign() <= 0 || sig.S.Sign() <= 0 || sig.R.BitLen() > 256 || sig.S.BitLen() > 256 {
		return "", errors.New("webpush: vault signature is not a P-256 ECDSA signature")
	}
	out := make([]byte, 64)
	sig.R.FillBytes(out[:32])
	sig.S.FillBytes(out[32:])
	return base64.RawURLEncoding.EncodeToString(out), nil
}
