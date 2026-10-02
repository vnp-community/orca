package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

type fakeMcpTokenRepository struct {
	tokens map[string]domain.McpToken
}

func (f *fakeMcpTokenRepository) CreateMcpToken(_ context.Context, t domain.McpToken) error {
	f.tokens[t.JTI] = t
	return nil
}
func (f *fakeMcpTokenRepository) ListMcpTokens(_ context.Context, tenantID, userID string) ([]domain.McpToken, error) {
	var out []domain.McpToken
	for _, t := range f.tokens {
		if t.TenantID == tenantID && t.UserID == userID {
			out = append(out, t)
		}
	}
	return out, nil
}
func (f *fakeMcpTokenRepository) GetMcpToken(_ context.Context, tenantID, jti string) (domain.McpToken, error) {
	t, ok := f.tokens[jti]
	if !ok || t.TenantID != tenantID {
		return domain.McpToken{}, ErrMcpTokenNotFound
	}
	return t, nil
}
func (f *fakeMcpTokenRepository) RevokeMcpToken(_ context.Context, tenantID, userID, jti string, at time.Time) error {
	t, ok := f.tokens[jti]
	if !ok || t.TenantID != tenantID || t.UserID != userID || t.RevokedAt != nil {
		return ErrMcpTokenNotFound
	}
	t.RevokedAt = &at
	f.tokens[jti] = t
	return nil
}
func (f *fakeMcpTokenRepository) MarkMcpTokenFirstUsed(_ context.Context, tenantID, jti string, at time.Time) (bool, error) {
	t := f.tokens[jti]
	if t.FirstUsedAt != nil {
		return false, nil
	}
	t.FirstUsedAt = &at
	f.tokens[jti] = t
	return true, nil
}
func (f *fakeMcpTokenRepository) TouchMcpTokenLastUsed(_ context.Context, _, jti string, at, _ time.Time) error {
	t := f.tokens[jti]
	t.LastUsedAt = &at
	f.tokens[jti] = t
	return nil
}

type mcpHarness struct {
	t      *testing.T
	repo   *fakeMcpTokenRepository
	oauth  *fakeOAuthRepository
	users  *fakeUserRepository
	audit  *fakeAuditRepository
	signer *rsaTestSigner
	clock  *fakeClock
	tenant string
	user   domain.User
	issue  *IssueMcpToken
	revoke *RevokeMcpToken
	res    *ResolveMcpPrincipal
}

func newMcpHarness(t *testing.T, role domain.Role) *mcpHarness {
	t.Helper()
	h := &mcpHarness{t: t, repo: &fakeMcpTokenRepository{tokens: map[string]domain.McpToken{}}, oauth: newFakeOAuthRepository(),
		users: newFakeUserRepository(), audit: &fakeAuditRepository{}, signer: newRSATestSigner(),
		clock: &fakeClock{now: time.Now().UTC().Truncate(time.Second)}, tenant: uuid.NewString()}
	u, err := domain.NewUser(uuid.NewString(), h.tenant, "p@example.com", "P", role, true, h.clock.now)
	if err != nil {
		t.Fatal(err)
	}
	h.user = u
	h.users.seed(u, "x")
	h.issue = NewIssueMcpToken(h.users, h.repo, h.audit, h.signer, h.clock, testResource)
	h.revoke = NewRevokeMcpToken(h.repo, h.audit, h.clock)
	h.res = NewResolveMcpPrincipal(h.users, h.repo, h.oauth, h.audit, h.clock)
	return h
}

func (h *mcpHarness) create(scopes []string, days int) (IssueMcpTokenOutput, error) {
	return h.issue.Execute(context.Background(), IssueMcpTokenInput{TenantID: h.tenant, UserID: h.user.ID, Name: "ci", Scopes: scopes, ExpiresInDays: days})
}

func (h *mcpHarness) resolve(jti string) ResolveMcpPrincipalOutput {
	h.t.Helper()
	out, err := h.res.Execute(context.Background(), ResolveMcpPrincipalInput{JTI: jti, UserID: h.user.ID, TenantID: h.tenant, TokenUse: "mcp_pat"})
	if err != nil {
		h.t.Fatal(err)
	}
	return out
}

func codeOf(err error) string {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}

func TestIssueMcpToken_SecretOnceHashOnly(t *testing.T) {
	h := newMcpHarness(t, domain.RoleUser)
	out, err := h.create([]string{"orca:read", "orca:write"}, 30)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.Secret, "omp_") {
		t.Fatalf("secret lacks recognisable prefix")
	}
	sum := sha256.Sum256([]byte(out.Secret))
	stored := h.repo.tokens[out.Token.JTI]
	if stored.TokenSHA256 != hex.EncodeToString(sum[:]) {
		t.Fatal("stored hash is not SHA-256 of the full secret")
	}
	// Nothing persisted or audited may contain the secret or the raw JWT.
	raw := strings.TrimPrefix(out.Secret, "omp_")
	blob, _ := json.Marshal([]any{h.repo.tokens, h.audit.entries})
	if strings.Contains(string(blob), raw) {
		t.Fatal("secret material leaked into stored record or audit")
	}
	c := h.signer.last
	if len(c.Audience) != 1 || c.Audience[0] != testResource || c.Role != "" || c.TokenUse != "mcp_pat" || c.Scope != "orca:read orca:write" || c.ID != out.Token.JTI {
		t.Fatalf("claims = %+v", c)
	}
	if got := out.Token.ExpiresAt.Sub(out.Token.CreatedAt); got != 30*24*time.Hour {
		t.Fatalf("ttl = %v", got)
	}
	if len(h.audit.entries) != 1 || h.audit.entries[0].Action != "mcp_token.created" {
		t.Fatalf("audit = %+v", h.audit.entries)
	}
}

func TestIssueMcpToken_Limits(t *testing.T) {
	h := newMcpHarness(t, domain.RoleUser)
	for _, days := range []int{0, -1, 91} {
		if _, err := h.create([]string{"orca:read"}, days); codeOf(err) != CodeMcpTokenTooLong {
			t.Errorf("days=%d: err=%v", days, err)
		}
	}
	if _, err := h.create([]string{"orca:read"}, 90); err != nil {
		t.Fatalf("90 days must be allowed: %v", err)
	}
	if _, err := h.create(nil, 5); codeOf(err) != CodeMcpScopeInvalid {
		t.Errorf("empty scopes: %v", err)
	}
	if _, err := h.create([]string{"orca:nope"}, 5); codeOf(err) != CodeMcpScopeInvalid {
		t.Errorf("unknown scope: %v", err)
	}
}

func TestIssueMcpToken_RoleCeiling(t *testing.T) {
	user := newMcpHarness(t, domain.RoleUser)
	if _, err := user.create([]string{"orca:read", "orca:admin"}, 5); codeOf(err) != CodeMcpScopeNotAllowed {
		t.Fatalf("user asking admin scope: %v", err)
	}
	if len(user.repo.tokens) != 0 {
		t.Fatal("nothing may be stored on failure")
	}
	admin := newMcpHarness(t, domain.RoleAdmin)
	if _, err := admin.create([]string{"orca:admin"}, 5); err != nil {
		t.Fatalf("admin may hold orca:admin: %v", err)
	}
}

func TestRevokeMcpToken_OwnershipAndAudit(t *testing.T) {
	h := newMcpHarness(t, domain.RoleUser)
	out, _ := h.create([]string{"orca:read"}, 5)
	other := uuid.NewString()
	if err := h.revoke.Execute(context.Background(), h.tenant, other, out.Token.JTI); codeOf(err) != CodeMcpTokenNotFound {
		t.Fatalf("someone else's jti must look unknown: %v", err)
	}
	if err := h.revoke.Execute(context.Background(), h.tenant, h.user.ID, out.Token.JTI); err != nil {
		t.Fatal(err)
	}
	if err := h.revoke.Execute(context.Background(), h.tenant, h.user.ID, out.Token.JTI); codeOf(err) != CodeMcpTokenNotFound {
		t.Fatalf("second revoke: %v", err)
	}
	last := h.audit.entries[len(h.audit.entries)-1]
	if last.Action != "mcp_token.revoked" || last.TargetID != out.Token.JTI {
		t.Fatalf("audit = %+v", last)
	}
}

func TestResolveMcpPrincipal_PATLifecycle(t *testing.T) {
	h := newMcpHarness(t, domain.RoleUser)
	out, _ := h.create([]string{"orca:read"}, 1)
	jti := out.Token.JTI

	r := h.resolve(jti)
	if !r.Active || r.Role != "user" {
		t.Fatalf("fresh token: %+v", r)
	}
	h.resolve(jti)
	first := 0
	for _, e := range h.audit.entries {
		if e.Action == "mcp_token.first_used" {
			first++
		}
	}
	if first != 1 {
		t.Fatalf("first_used audits = %d, want exactly 1", first)
	}

	// Live role: a promotion/demotion is visible immediately.
	h.users.byID[h.user.ID] = func() domain.User { u := h.user; u.Role = domain.RoleAdmin; return u }()
	if r := h.resolve(jti); r.Role != "admin" {
		t.Fatalf("role not live: %+v", r)
	}

	// Expiry.
	h.clock.now = h.clock.now.Add(25 * time.Hour)
	if r := h.resolve(jti); r.Active || r.InactiveReason != McpInactiveExpired {
		t.Fatalf("expired: %+v", r)
	}
	h.clock.now = h.clock.now.Add(-25 * time.Hour)

	// Revocation.
	if err := h.revoke.Execute(context.Background(), h.tenant, h.user.ID, jti); err != nil {
		t.Fatal(err)
	}
	if r := h.resolve(jti); r.Active || r.InactiveReason != McpInactiveRevoked {
		t.Fatalf("revoked: %+v", r)
	}
}

func TestResolveMcpPrincipal_RevokedAndDeactivatedUsers(t *testing.T) {
	h := newMcpHarness(t, domain.RoleUser)
	out, _ := h.create([]string{"orca:read"}, 5)
	deactivated := h.user
	deactivated.IsActive = false
	h.users.byID[h.user.ID] = deactivated
	if r := h.resolve(out.Token.JTI); r.Active || r.InactiveReason != McpInactiveUserInactive || r.Role != "" {
		t.Fatalf("deactivated user: %+v", r)
	}
	// Unknown jti, wrong tenant, wrong token_use all fail closed.
	h.users.byID[h.user.ID] = h.user
	if r := h.resolve("nope"); r.Active || r.InactiveReason != McpInactiveUnknown {
		t.Fatalf("unknown jti: %+v", r)
	}
	r, _ := h.res.Execute(context.Background(), ResolveMcpPrincipalInput{JTI: out.Token.JTI, UserID: h.user.ID, TenantID: uuid.NewString(), TokenUse: "mcp_pat"})
	if r.Active {
		t.Fatal("other tenant must not resolve")
	}
	r, _ = h.res.Execute(context.Background(), ResolveMcpPrincipalInput{JTI: out.Token.JTI, UserID: h.user.ID, TenantID: h.tenant, TokenUse: "weird"})
	if r.Active {
		t.Fatal("unknown token_use must not resolve")
	}
}

func TestResolveMcpPrincipal_OAuthBranch(t *testing.T) {
	h := newMcpHarness(t, domain.RoleUser)
	fam := domain.OAuthTokenFamily{FamilyID: uuid.NewString(), TenantID: h.tenant, UserID: h.user.ID, ClientID: "c1", GrantID: uuid.NewString(), Scope: "orca:read", Resource: testResource, CreatedAt: h.clock.now}
	h.oauth.families[fam.FamilyID] = fam
	in := ResolveMcpPrincipalInput{JTI: "j", UserID: h.user.ID, TenantID: h.tenant, TokenUse: "mcp_oauth", FamilyID: fam.FamilyID, GrantID: fam.GrantID, ClientID: "c1"}
	run := func() ResolveMcpPrincipalOutput {
		out, err := h.res.Execute(context.Background(), in)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	if r := run(); !r.Active || r.Role != "user" {
		t.Fatalf("active family: %+v", r)
	}
	h.oauth.statuses[statusKey(h.tenant, "c1")] = domain.OAuthClientTenantStatus{TenantID: h.tenant, ClientID: "c1", Status: domain.OAuthClientBlocked}
	if r := run(); r.Active || r.InactiveReason != McpInactiveClientBlock {
		t.Fatalf("blocked client: %+v", r)
	}
	delete(h.oauth.statuses, statusKey(h.tenant, "c1"))
	h.oauth.grants[fam.GrantID] = true
	if r := run(); r.Active || r.InactiveReason != McpInactiveGrantRevoked {
		t.Fatalf("revoked grant: %+v", r)
	}
	delete(h.oauth.grants, fam.GrantID)
	now := h.clock.now
	fam.RevokedAt = &now
	h.oauth.families[fam.FamilyID] = fam
	if r := run(); r.Active || r.InactiveReason != McpInactiveRevoked {
		t.Fatalf("revoked family: %+v", r)
	}
	in.FamilyID = "missing"
	if r := run(); r.Active || r.InactiveReason != McpInactiveUnknown {
		t.Fatalf("unknown family: %+v", r)
	}
}
