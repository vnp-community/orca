package jwtauth_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/stablyai/orca-go/common/jwtauth"
)

// fakeTransit signs with an in-memory RSA key exactly the way Vault
// Transit would (PKCS#1v1.5 over a SHA-256 digest), returning the same
// "vault:v<N>:<base64>" wire format TransitSign strips — this exercises
// jwtauth's real signing/verification logic without needing a live Vault.
type fakeTransit struct {
	key     *rsa.PrivateKey
	version int
	pem     string
}

func newFakeTransit(t *testing.T, version int) *fakeTransit {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating test RSA key: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshaling public key: %v", err)
	}
	pemBlock := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	return &fakeTransit{key: key, version: version, pem: string(pemBlock)}
}

func (f *fakeTransit) sign(_ context.Context, _ string, input []byte) (string, error) {
	digest := sha256.Sum256(input)
	sig, err := rsa.SignPKCS1v15(rand.Reader, f.key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("vault:v%d:%s", f.version, base64.StdEncoding.EncodeToString(sig)), nil
}

func (f *fakeTransit) publicKeyVersions(_ context.Context, _ string) (map[int]string, int, error) {
	return map[int]string{f.version: f.pem}, f.version, nil
}

func TestSignVerify_RoundTrip(t *testing.T) {
	transit := newFakeTransit(t, 1)
	signer := jwtauth.NewTransitSigner("jwt-signing", transit.sign, transit.publicKeyVersions)

	claims := jwtauth.Claims{
		Claims: jwt.Claims{
			Issuer:   jwtauth.Issuer,
			Subject:  "user-1",
			Audience: jwt.Audience{"git-gateway-service"},
			IssuedAt: jwt.NewNumericDate(time.Now()),
			Expiry:   jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			ID:       "jti-1",
		},
		TenantID: "tenant-1",
	}

	token, err := jwtauth.Sign(context.Background(), signer, claims)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	jwks, err := signer.PublicJWKS(context.Background())
	if err != nil {
		t.Fatalf("PublicJWKS: %v", err)
	}
	if len(jwks.Keys) != 1 {
		t.Fatalf("expected 1 key (no previous version yet), got %d", len(jwks.Keys))
	}

	got, err := jwtauth.Verify(jwks, token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got.Subject != "user-1" || got.TenantID != "tenant-1" {
		t.Fatalf("unexpected claims: %+v", got)
	}
}

func TestVerify_RejectsTamperedSignature(t *testing.T) {
	transit := newFakeTransit(t, 1)
	signer := jwtauth.NewTransitSigner("jwt-signing", transit.sign, transit.publicKeyVersions)
	claims := jwtauth.Claims{Claims: jwt.Claims{
		Issuer: jwtauth.Issuer, Subject: "user-1",
		IssuedAt: jwt.NewNumericDate(time.Now()), Expiry: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}}
	token, err := jwtauth.Sign(context.Background(), signer, claims)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	jwks, err := signer.PublicJWKS(context.Background())
	if err != nil {
		t.Fatalf("PublicJWKS: %v", err)
	}

	tampered := token[:len(token)-4] + "abcd"
	if _, err := jwtauth.Verify(jwks, tampered); err == nil {
		t.Fatal("expected tampered signature to be rejected")
	}
}

func TestVerify_RejectsExpiredToken(t *testing.T) {
	transit := newFakeTransit(t, 1)
	signer := jwtauth.NewTransitSigner("jwt-signing", transit.sign, transit.publicKeyVersions)
	claims := jwtauth.Claims{Claims: jwt.Claims{
		Issuer: jwtauth.Issuer, Subject: "user-1",
		IssuedAt: jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		Expiry:   jwt.NewNumericDate(time.Now().Add(-time.Hour)),
	}}
	token, err := jwtauth.Sign(context.Background(), signer, claims)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	jwks, err := signer.PublicJWKS(context.Background())
	if err != nil {
		t.Fatalf("PublicJWKS: %v", err)
	}

	if _, err := jwtauth.Verify(jwks, token); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestVerify_RejectsUnknownKid(t *testing.T) {
	signingTransit := newFakeTransit(t, 1)
	signer := jwtauth.NewTransitSigner("jwt-signing", signingTransit.sign, signingTransit.publicKeyVersions)
	claims := jwtauth.Claims{Claims: jwt.Claims{
		Issuer: jwtauth.Issuer, Subject: "user-1",
		IssuedAt: jwt.NewNumericDate(time.Now()), Expiry: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}}
	token, err := jwtauth.Sign(context.Background(), signer, claims)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	// A JWKS from an unrelated key (different "kid") — the verifier must
	// not find a match and must fail closed, not fall back to trusting the
	// claims unverified.
	otherTransit := newFakeTransit(t, 2)
	otherJWKS, err := jwtauth.NewTransitSigner("jwt-signing", otherTransit.sign, otherTransit.publicKeyVersions).PublicJWKS(context.Background())
	if err != nil {
		t.Fatalf("PublicJWKS: %v", err)
	}

	if _, err := jwtauth.Verify(otherJWKS, token); err == nil {
		t.Fatal("expected unknown kid to be rejected")
	}
}

func TestPublicJWKS_IncludesPreviousVersionDuringRotation(t *testing.T) {
	// Simulate rotation: pubKey now reports both version 1 (previous) and
	// version 2 (current) — auth-service.md §9's overlap window.
	pubKey := func(_ context.Context, _ string) (map[int]string, int, error) {
		t1 := newFakeTransit(t, 1)
		t2 := newFakeTransit(t, 2)
		return map[int]string{1: t1.pem, 2: t2.pem}, 2, nil
	}
	signer := jwtauth.NewTransitSigner("jwt-signing", nil, pubKey)
	jwks, err := signer.PublicJWKS(context.Background())
	if err != nil {
		t.Fatalf("PublicJWKS: %v", err)
	}
	if len(jwks.Keys) != 2 {
		t.Fatalf("expected 2 keys (current + previous) during rotation overlap, got %d", len(jwks.Keys))
	}
}

var _ jose.OpaqueSigner = (*jwtauth.TransitSigner)(nil)
