package usecase

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"testing"

	jose "github.com/go-jose/go-jose/v4"
)

func TestGetJWKS_MarshalsPublicKeySet(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating rsa key: %v", err)
	}
	signer := &fakeTokenSigner{
		jwks: jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
			{Key: &key.PublicKey, KeyID: "1", Algorithm: string(jose.RS256), Use: "sig"},
		}},
	}

	uc := NewGetJWKS(signer)
	out, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.JWKSJSON == "" {
		t.Fatal("expected non-empty jwks json")
	}

	var parsed jose.JSONWebKeySet
	if err := json.Unmarshal([]byte(out.JWKSJSON), &parsed); err != nil {
		t.Fatalf("expected valid JSON JWK Set, got parse error: %v", err)
	}
	if len(parsed.Keys) != 1 || parsed.Keys[0].KeyID != "1" {
		t.Errorf("expected one key with kid=1, got %+v", parsed.Keys)
	}
}

func TestGetJWKS_SignerFailurePropagates(t *testing.T) {
	signer := &fakeTokenSigner{jwksErr: errors.New("fake: jwks failed")}
	uc := NewGetJWKS(signer)

	if _, err := uc.Execute(context.Background()); err == nil {
		t.Fatal("expected the signer's error to propagate")
	}
}
