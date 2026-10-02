package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

const (
	testResource    = "https://orca.example.com/mcp"
	testRedirectURI = "https://client.example.com/callback"
)

type oauthHarness struct {
	t        *testing.T
	repo     *fakeOAuthRepository
	users    *fakeUserRepository
	audit    *fakeAuditRepository
	signer   *rsaTestSigner
	clock    *fakeClock
	cfg      OAuthConfig
	tenantID string
	user     domain.User
	clientID string

	issue    *OAuthIssueAuthCode
	exchange *OAuthExchangeToken
	revoke   *OAuthRevokeToken
	revokeG  *OAuthRevokeGrant
	setSt    *OAuthSetClientStatus
}

func newOAuthHarness(t *testing.T) *oauthHarness {
	t.Helper()
	h := &oauthHarness{
		t: t, repo: newFakeOAuthRepository(), users: newFakeUserRepository(), audit: &fakeAuditRepository{},
		signer: newRSATestSigner(), clock: &fakeClock{now: time.Now().UTC().Truncate(time.Second)}, // real time: jwtauth.Verify checks exp against the wall clock
		tenantID: uuid.NewString(), clientID: "client-1",
		cfg: OAuthConfig{ResourceURL: testResource, DCREnabled: true},
	}
	u, err := domain.NewUser(uuid.NewString(), h.tenantID, "u@example.com", "U", domain.RoleUser, true, h.clock.now)
	if err != nil {
		t.Fatal(err)
	}
	h.user = u
	h.users.seed(u, "x")
	h.repo.clients[h.clientID] = domain.OAuthClient{
		ClientID: h.clientID, ClientName: "Test App", RedirectURIs: []string{testRedirectURI},
		RegisteredVia: domain.OAuthRegisteredViaDCR, CreatedAt: h.clock.now,
	}
	h.setStatus(domain.OAuthClientAllowed)
	h.issue = NewOAuthIssueAuthCode(h.repo, h.users, h.audit, h.clock, h.cfg)
	h.exchange = NewOAuthExchangeToken(h.repo, h.users, h.signer, h.audit, h.clock, h.cfg)
	h.revoke = NewOAuthRevokeToken(h.repo, h.signer, h.audit, h.clock)
	h.revokeG = NewOAuthRevokeGrant(h.repo, h.audit, h.clock)
	h.setSt = NewOAuthSetClientStatus(h.repo, h.audit, h.clock)
	return h
}

func (h *oauthHarness) setStatus(s domain.OAuthClientStatus) {
	h.repo.statuses[statusKey(h.tenantID, h.clientID)] = domain.OAuthClientTenantStatus{TenantID: h.tenantID, ClientID: h.clientID, Status: s}
}

func pkcePair(seed string) (verifier, challenge string) {
	sum := sha256.Sum256([]byte(seed))
	verifier = base64.RawURLEncoding.EncodeToString(append(sum[:], sum[:12]...)) // 59 chars
	c := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(c[:])
}

type authorized struct {
	code, verifier, challenge, grantID string
}

// authorize walks consent -> code issuance for the harness user.
func (h *oauthHarness) authorize(scopes ...string) authorized {
	h.t.Helper()
	if len(scopes) == 0 {
		scopes = []string{domain.OAuthScopeRead, domain.OAuthScopeWrite}
	}
	v, c := pkcePair(uuid.NewString())
	a := authorized{verifier: v, challenge: c, grantID: uuid.NewString()}
	out, err := h.issue.Execute(context.Background(), OAuthIssueAuthCodeInput{
		TenantID: h.tenantID, UserID: h.user.ID, ClientID: h.clientID, RedirectURI: testRedirectURI,
		Scopes: scopes, CodeChallenge: c, Resource: testResource, GrantID: a.grantID,
	})
	if err != nil {
		h.t.Fatalf("issue code: %v", err)
	}
	a.code = out.Code
	return a
}

func (h *oauthHarness) codeInput(a authorized) OAuthExchangeTokenInput {
	return OAuthExchangeTokenInput{
		GrantType: "authorization_code", Code: a.code, RedirectURI: testRedirectURI,
		CodeVerifier: a.verifier, ClientID: h.clientID, Resource: testResource,
	}
}

func (h *oauthHarness) tokens(a authorized) OAuthTokenOutput {
	h.t.Helper()
	out, err := h.exchange.Execute(context.Background(), h.codeInput(a))
	if err != nil {
		h.t.Fatalf("exchange: %v", err)
	}
	return out
}

func (h *oauthHarness) refreshInput(rt string) OAuthExchangeTokenInput {
	return OAuthExchangeTokenInput{GrantType: "refresh_token", RefreshToken: rt, ClientID: h.clientID}
}

func (h *oauthHarness) familyOfGrant(grantID string) domain.OAuthTokenFamily {
	h.t.Helper()
	for _, f := range h.repo.families {
		if f.GrantID == grantID {
			return f
		}
	}
	h.t.Fatal("family not found")
	return domain.OAuthTokenFamily{}
}

func errCode(err error) string {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("want error %s, got nil", code)
	}
	if got := errCode(err); got != code {
		t.Fatalf("want error code %s, got %q (%v)", code, got, err)
	}
}

func blockedStatus(tenantID, clientID string) domain.OAuthClientTenantStatus {
	return domain.OAuthClientTenantStatus{TenantID: tenantID, ClientID: clientID, Status: domain.OAuthClientBlocked}
}
