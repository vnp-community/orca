package jwtauth

import (
	"fmt"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// KeyID extracts the "kid" header from a compact-serialized JWT without
// verifying its signature — the standard first step of kid-based
// verification (RFC 7515 §4.1.4: look up which key to verify against
// before you can verify against it). Reading an unverified header is safe;
// callers must still call VerifyWithKey or Verify before trusting anything
// else about the token — this alone proves nothing.
func KeyID(tokenString string) (string, error) {
	tok, err := jwt.ParseSigned(tokenString, []jose.SignatureAlgorithm{Algorithm})
	if err != nil {
		return "", fmt.Errorf("jwtauth: parsing token: %w", err)
	}
	if len(tok.Headers) != 1 {
		return "", fmt.Errorf("jwtauth: unexpected signature count %d", len(tok.Headers))
	}
	kid := tok.Headers[0].KeyID
	if kid == "" {
		return "", fmt.Errorf("jwtauth: token has no kid header")
	}
	return kid, nil
}

// VerifyWithKey validates tokenString's signature against a single
// already-resolved key — the shape api-gateway's pre-existing JWKSClient
// port (PublicKey(ctx, kid) (any, error)) hands back after a KeyID-based
// lookup — plus its exp/iat/iss, returning the parsed claims. Same
// fail-closed semantics as Verify, just for a caller that already resolved
// which key to use instead of handing over a whole JWKSet.
func VerifyWithKey(key any, tokenString string) (Claims, error) {
	tok, err := jwt.ParseSigned(tokenString, []jose.SignatureAlgorithm{Algorithm})
	if err != nil {
		return Claims{}, fmt.Errorf("jwtauth: parsing token: %w", err)
	}
	var claims Claims
	if err := tok.Claims(key, &claims); err != nil {
		return Claims{}, fmt.Errorf("jwtauth: signature verification failed: %w", err)
	}
	if err := claims.Validate(jwt.Expected{Issuer: Issuer, Time: time.Now()}); err != nil {
		return Claims{}, fmt.Errorf("jwtauth: claims validation: %w", err)
	}
	return claims, nil
}

// Verify validates a compact-serialized JWT's signature against jwks (as
// published by auth-service's GetJWKS) plus its exp/iat/iss, returning the
// parsed claims. Fails closed on any of: unparseable token, non-RS256 alg,
// missing/unknown "kid", bad signature, wrong issuer, or an
// expired/not-yet-valid token — there is no partial-trust path here,
// unlike the unverified placeholder this replaces.
func Verify(jwks jose.JSONWebKeySet, tokenString string) (Claims, error) {
	kid, err := KeyID(tokenString)
	if err != nil {
		return Claims{}, err
	}
	candidates := jwks.Key(kid)
	if len(candidates) == 0 {
		return Claims{}, fmt.Errorf("jwtauth: no key found for kid %q", kid)
	}

	var lastErr error
	for _, k := range candidates {
		claims, err := VerifyWithKey(k.Key, tokenString)
		if err == nil {
			return claims, nil
		}
		lastErr = err
	}
	return Claims{}, fmt.Errorf("jwtauth: signature verification failed: %w", lastErr)
}
