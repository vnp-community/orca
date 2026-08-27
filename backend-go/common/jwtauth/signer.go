package jwtauth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// signCaller/publicKeyCaller name the exact subset of
// common/secrets.Client's Transit methods TransitSigner needs, so this
// package doesn't import common/secrets directly — callers wire the
// concrete client's methods in (see auth-service's internal/adapter/vault).
type signCaller func(ctx context.Context, keyName string, input []byte) (string, error)
type publicKeyCaller func(ctx context.Context, keyName string) (versions map[int]string, latestVersion int, err error)

// TransitSigner implements jose.OpaqueSigner (RS256 only) by delegating the
// actual signature computation to Vault Transit — the RSA private key never
// enters this process's memory, only signatures Vault already computed do.
//
// jose.OpaqueSigner's methods don't accept a context.Context (matching
// crypto.Signer's own signature, which has the same limitation for the same
// reason: it's meant to also cover HSM/KMS-backed keys). Public()/SignPayload
// therefore use a bounded background context rather than a caller-supplied
// one — document this tradeoff rather than hide it.
type TransitSigner struct {
	KeyName string
	sign    signCaller
	pubKey  publicKeyCaller
	timeout time.Duration

	mu       sync.Mutex
	cached   *jose.JSONWebKey
	cachedAt time.Time
	cacheTTL time.Duration
}

// NewTransitSigner builds a TransitSigner over the given Transit key name.
// sign/pubKey are typically *secrets.Client.TransitSign/TransitPublicKeyVersions.
func NewTransitSigner(keyName string, sign signCaller, pubKey publicKeyCaller) *TransitSigner {
	return &TransitSigner{
		KeyName:  keyName,
		sign:     sign,
		pubKey:   pubKey,
		timeout:  10 * time.Second,
		cacheTTL: 60 * time.Second, // bounds how many stray Vault reads a burst of IssueServiceToken calls causes
	}
}

// Public returns the public half of the signing key currently in use,
// wrapped as a jose.JSONWebKey with KeyID set to the Transit key version —
// see PublicJWKS's doc comment for the kid convention.
func (s *TransitSigner) Public() *jose.JSONWebKey {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	jwk, err := s.currentJWK(ctx)
	if err != nil {
		return nil
	}
	return jwk
}

// Algs reports the one algorithm this signer supports.
func (s *TransitSigner) Algs() []jose.SignatureAlgorithm {
	return []jose.SignatureAlgorithm{Algorithm}
}

// SignPayload signs payload via Vault Transit and returns the raw signature
// bytes go-jose embeds in the JWS. alg must be RS256 — this signer supports
// nothing else (see Algorithm's doc comment on why).
func (s *TransitSigner) SignPayload(payload []byte, alg jose.SignatureAlgorithm) ([]byte, error) {
	if alg != Algorithm {
		return nil, fmt.Errorf("jwtauth: unsupported signature algorithm %q", alg)
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	wireSig, err := s.sign(ctx, s.KeyName, payload)
	if err != nil {
		return nil, fmt.Errorf("jwtauth: transit sign: %w", err)
	}
	return decodeVaultSignature(wireSig)
}

// currentJWK returns the signing key's current (latest) public half,
// cached briefly to avoid a Vault round trip on every SignPayload/Sign
// call.
func (s *TransitSigner) currentJWK(ctx context.Context) (*jose.JSONWebKey, error) {
	s.mu.Lock()
	if s.cached != nil && time.Since(s.cachedAt) < s.cacheTTL {
		defer s.mu.Unlock()
		return s.cached, nil
	}
	s.mu.Unlock()

	versions, latest, err := s.pubKey(ctx, s.KeyName)
	if err != nil {
		return nil, err
	}
	jwk, err := jwkFromPEM(versions[latest], strconv.Itoa(latest))
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.cached, s.cachedAt = jwk, time.Now()
	s.mu.Unlock()
	return jwk, nil
}

// PublicJWKS returns the RFC 7517 JWK Set for the signing key's current and
// immediately-previous version — the rotation-overlap window
// auth-service.md §9 requires: a rotation publishes the new key before it's
// used to sign anything, and keeps the previous key published until every
// JWT signed under it has expired, so a consumer's cached JWKS is never
// invalidated mid-flight. KeyID on each entry is the Transit key version,
// stringified — this makes "which key signed this JWT" and "which key
// version's public half do I need to verify it" the same lookup, with no
// separate kid bookkeeping.
func (s *TransitSigner) PublicJWKS(ctx context.Context) (jose.JSONWebKeySet, error) {
	versions, latest, err := s.pubKey(ctx, s.KeyName)
	if err != nil {
		return jose.JSONWebKeySet{}, err
	}
	set := jose.JSONWebKeySet{}
	for _, version := range []int{latest, latest - 1} {
		pem, ok := versions[version]
		if !ok {
			continue // no previous version yet (fresh key) — current-only JWKS is correct
		}
		jwk, err := jwkFromPEM(pem, strconv.Itoa(version))
		if err != nil {
			return jose.JSONWebKeySet{}, fmt.Errorf("jwtauth: version %d: %w", version, err)
		}
		set.Keys = append(set.Keys, *jwk)
	}
	return set, nil
}

// Sign mints a compact-serialized RS256 JWT for claims, signed by signer.
// The "kid" header is set from the signing key's current Transit version.
func Sign(ctx context.Context, signer *TransitSigner, claims Claims) (string, error) {
	jwk, err := signer.currentJWK(ctx)
	if err != nil {
		return "", fmt.Errorf("jwtauth: resolving signing key: %w", err)
	}
	opts := (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", jwk.KeyID)
	joseSigner, err := jose.NewSigner(jose.SigningKey{Algorithm: Algorithm, Key: signer}, opts)
	if err != nil {
		return "", fmt.Errorf("jwtauth: constructing signer: %w", err)
	}
	token, err := jwt.Signed(joseSigner).Claims(claims).Serialize()
	if err != nil {
		return "", fmt.Errorf("jwtauth: signing token: %w", err)
	}
	return token, nil
}

func jwkFromPEM(pemBlock, keyID string) (*jose.JSONWebKey, error) {
	pub, err := parseRSAPublicKeyPEM(pemBlock)
	if err != nil {
		return nil, err
	}
	return &jose.JSONWebKey{Key: pub, KeyID: keyID, Algorithm: string(Algorithm), Use: "sig"}, nil
}

func parseRSAPublicKeyPEM(pemBlock string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemBlock))
	if block == nil {
		return nil, fmt.Errorf("jwtauth: no PEM block found in public key")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("jwtauth: parsing PKIX public key: %w", err)
	}
	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("jwtauth: public key is %T, not RSA", key)
	}
	return rsaKey, nil
}

// decodeVaultSignature strips Vault Transit's "vault:v<N>:" wire-format
// prefix and base64-decodes the remainder into raw signature bytes.
func decodeVaultSignature(wire string) ([]byte, error) {
	parts := strings.SplitN(wire, ":", 3)
	if len(parts) != 3 || parts[0] != "vault" {
		return nil, fmt.Errorf("jwtauth: unexpected vault signature format %q", wire)
	}
	return base64.StdEncoding.DecodeString(parts[2])
}
