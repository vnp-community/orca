package usecase

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"sync"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/stablyai/orca-go/common/jwtauth"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// fakeOAuthRepository is an in-memory OAuthRepository. Its conditional
// updates mirror the SQL "WHERE used_at IS NULL" semantics.
type fakeOAuthRepository struct {
	mu       sync.Mutex
	clients  map[string]domain.OAuthClient
	statuses map[string]domain.OAuthClientTenantStatus // tenant|client
	codes    map[string]domain.OAuthAuthCode
	families map[string]domain.OAuthTokenFamily
	refresh  map[string]domain.OAuthRefreshToken
	grants   map[string]bool

	// forceCodeRace makes ClaimOAuthAuthCode lose the race once.
	forceCodeRace    bool
	forceRefreshRace bool
}

func newFakeOAuthRepository() *fakeOAuthRepository {
	return &fakeOAuthRepository{
		clients: map[string]domain.OAuthClient{}, statuses: map[string]domain.OAuthClientTenantStatus{},
		codes: map[string]domain.OAuthAuthCode{}, families: map[string]domain.OAuthTokenFamily{},
		refresh: map[string]domain.OAuthRefreshToken{}, grants: map[string]bool{},
	}
}

func statusKey(tenantID, clientID string) string { return tenantID + "|" + clientID }

func (f *fakeOAuthRepository) CountOAuthClients(context.Context) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.clients), nil
}

func (f *fakeOAuthRepository) CreateOAuthClient(_ context.Context, c domain.OAuthClient) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clients[c.ClientID] = c
	return nil
}

func (f *fakeOAuthRepository) GetOAuthClient(_ context.Context, id string) (domain.OAuthClient, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.clients[id]
	if !ok {
		return domain.OAuthClient{}, ErrOAuthClientNotFound
	}
	return c, nil
}

func (f *fakeOAuthRepository) TouchOAuthClientUsed(context.Context, string, time.Time) error {
	return nil
}

func (f *fakeOAuthRepository) EnsureOAuthClientTenantStatus(_ context.Context, s domain.OAuthClientTenantStatus) (domain.OAuthClientTenantStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	k := statusKey(s.TenantID, s.ClientID)
	if cur, ok := f.statuses[k]; ok {
		return cur, nil
	}
	f.statuses[k] = s
	return s, nil
}

func (f *fakeOAuthRepository) GetOAuthClientTenantStatus(_ context.Context, tenantID, clientID string) (domain.OAuthClientTenantStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.statuses[statusKey(tenantID, clientID)]
	if !ok {
		return domain.OAuthClientTenantStatus{}, ErrOAuthClientStatusNotFound
	}
	return s, nil
}

func (f *fakeOAuthRepository) SetOAuthClientTenantStatus(_ context.Context, s domain.OAuthClientTenantStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	k := statusKey(s.TenantID, s.ClientID)
	if _, ok := f.statuses[k]; !ok {
		return ErrOAuthClientStatusNotFound
	}
	f.statuses[k] = s
	return nil
}

func (f *fakeOAuthRepository) ListOAuthClientViews(_ context.Context, tenantID string) ([]domain.OAuthClientView, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.OAuthClientView
	for _, s := range f.statuses {
		if s.TenantID == tenantID {
			out = append(out, domain.OAuthClientView{Client: f.clients[s.ClientID], Status: s})
		}
	}
	return out, nil
}

func (f *fakeOAuthRepository) CreateOAuthFamilyWithCode(_ context.Context, fam domain.OAuthTokenFamily, c domain.OAuthAuthCode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.families[fam.FamilyID] = fam
	f.codes[c.CodeHash] = c
	return nil
}

func (f *fakeOAuthRepository) GetOAuthAuthCode(_ context.Context, h string) (domain.OAuthAuthCode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.codes[h]
	if !ok {
		return domain.OAuthAuthCode{}, ErrOAuthCodeNotFound
	}
	return c, nil
}

func (f *fakeOAuthRepository) GetOAuthFamily(_ context.Context, id string) (domain.OAuthTokenFamily, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fam, ok := f.families[id]
	if !ok {
		return domain.OAuthTokenFamily{}, ErrOAuthFamilyNotFound
	}
	return fam, nil
}

func (f *fakeOAuthRepository) ClaimOAuthAuthCode(_ context.Context, h string, at time.Time, first domain.OAuthRefreshToken) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := f.codes[h]
	if c.UsedAt != nil || f.forceCodeRace {
		f.forceCodeRace = false
		return ErrOAuthCodeAlreadyUsed
	}
	c.UsedAt = &at
	f.codes[h] = c
	f.refresh[first.TokenHash] = first
	return nil
}

func (f *fakeOAuthRepository) GetOAuthRefreshToken(_ context.Context, h string) (domain.OAuthRefreshToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rt, ok := f.refresh[h]
	if !ok {
		return domain.OAuthRefreshToken{}, ErrOAuthRefreshNotFound
	}
	return rt, nil
}

func (f *fakeOAuthRepository) RotateOAuthRefreshToken(_ context.Context, old string, at time.Time, next domain.OAuthRefreshToken) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	rt := f.refresh[old]
	if rt.UsedAt != nil || f.forceRefreshRace {
		f.forceRefreshRace = false
		return ErrOAuthRefreshAlreadyUsed
	}
	rt.UsedAt = &at
	rt.ReplacedByHash = next.TokenHash
	f.refresh[old] = rt
	f.refresh[next.TokenHash] = next
	return nil
}

func (f *fakeOAuthRepository) revokeLocked(id, reason string, at time.Time) bool {
	fam, ok := f.families[id]
	if !ok || fam.RevokedAt != nil {
		return false
	}
	fam.RevokedAt = &at
	fam.RevokeReason = reason
	f.families[id] = fam
	return true
}

func (f *fakeOAuthRepository) RevokeOAuthFamily(_ context.Context, id, reason string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.revokeLocked(id, reason, at)
	return nil
}

func (f *fakeOAuthRepository) RevokeOAuthGrant(_ context.Context, tenantID, grantID, reason string, at time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.grants[grantID] = true
	n := 0
	for id, fam := range f.families {
		if fam.GrantID == grantID && fam.TenantID == tenantID && f.revokeLocked(id, reason, at) {
			n++
		}
	}
	return n, nil
}

func (f *fakeOAuthRepository) RevokeOAuthFamiliesForClient(_ context.Context, tenantID, clientID, reason string, at time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for id, fam := range f.families {
		if fam.ClientID == clientID && fam.TenantID == tenantID && f.revokeLocked(id, reason, at) {
			n++
		}
	}
	return n, nil
}

func (f *fakeOAuthRepository) IsOAuthGrantRevoked(_ context.Context, _, grantID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.grants[grantID], nil
}

// rsaTestSigner is a real RS256 TokenSigner/verifier pair with an in-process
// key, so revoke-by-access-token tests exercise real signature verification.
type rsaTestSigner struct {
	key  *rsa.PrivateKey
	kid  string
	last jwtauth.Claims
}

func newRSATestSigner() *rsaTestSigner {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return &rsaTestSigner{key: k, kid: "test-key"}
}

func (s *rsaTestSigner) Sign(_ context.Context, claims jwtauth.Claims) (string, error) {
	s.last = claims
	sig, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: s.key, KeyID: s.kid}},
		(&jose.SignerOptions{}).WithType("JWT"))
	if err != nil {
		return "", fmt.Errorf("test signer: %w", err)
	}
	return jwt.Signed(sig).Claims(claims).Serialize()
}

func (s *rsaTestSigner) PublicJWKS(context.Context) (jose.JSONWebKeySet, error) {
	return jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &s.key.PublicKey, KeyID: s.kid, Algorithm: string(jose.RS256), Use: "sig"}}}, nil
}
