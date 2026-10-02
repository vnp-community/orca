//go:build integration

package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/jwtauth"
	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

type oauthFixture struct {
	repo     *Repository
	tenantID string
	userID   string
	clientID string
}

func newOAuthFixture(t *testing.T) oauthFixture {
	t.Helper()
	repo := setupRepository(t)
	ctx := context.Background()
	f := oauthFixture{repo: repo, tenantID: uuid.NewString(), clientID: "client-" + uuid.NewString()}
	u, err := domain.NewUser(uuid.NewString(), f.tenantID, "oauth@example.com", "O", domain.RoleUser, true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateUser(ctx, u, "hash"); err != nil {
		t.Fatal(err)
	}
	f.userID = u.ID
	if err := repo.CreateOAuthClient(ctx, domain.OAuthClient{
		ClientID: f.clientID, ClientName: "App", ClientURI: "https://app.example.com",
		RedirectURIs: []string{"https://app.example.com/cb", "http://127.0.0.1/cb"}, RegisteredVia: "dcr", CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f oauthFixture) family(t *testing.T, grantID string) (domain.OAuthTokenFamily, domain.OAuthAuthCode) {
	t.Helper()
	fam := domain.OAuthTokenFamily{
		FamilyID: uuid.NewString(), TenantID: f.tenantID, UserID: f.userID, ClientID: f.clientID, GrantID: grantID,
		Scope: "orca:read", Resource: "https://orca.example.com/mcp", CreatedAt: time.Now().UTC(),
	}
	code := domain.OAuthAuthCode{
		CodeHash: uuid.NewString(), FamilyID: fam.FamilyID, TenantID: f.tenantID, RedirectURI: "https://app.example.com/cb",
		CodeChallenge: "c", ExpiresAt: time.Now().UTC().Add(time.Minute),
	}
	if err := f.repo.CreateOAuthFamilyWithCode(context.Background(), fam, code); err != nil {
		t.Fatal(err)
	}
	return fam, code
}

func TestOAuthMigration_UpDownUp(t *testing.T) {
	dsn := testutil.StartPostgres(t, "auth")
	path, _ := filepath.Abs("../../../migrations/postgres")
	run := func(args ...string) {
		t.Helper()
		full := append([]string{"-path", path, "-database", dsn}, args...)
		if out, err := exec.Command("migrate", full...).CombinedOutput(); err != nil {
			t.Fatalf("migrate %v: %v\n%s", args, err, out)
		}
	}
	run("up")
	run("down", "1")
	run("up")
}

func TestOAuthClient_RoundTripAndStatus(t *testing.T) {
	f := newOAuthFixture(t)
	ctx := context.Background()

	c, err := f.repo.GetOAuthClient(ctx, f.clientID)
	if err != nil || len(c.RedirectURIs) != 2 || c.RedirectURIs[1] != "http://127.0.0.1/cb" || c.ClientURI != "https://app.example.com" {
		t.Fatalf("client = %+v err = %v", c, err)
	}
	if _, err := f.repo.GetOAuthClient(ctx, "nope"); err == nil {
		t.Fatal("expected not found")
	}
	if n, _ := f.repo.CountOAuthClients(ctx); n != 1 {
		t.Fatalf("count = %d", n)
	}
	if err := f.repo.CreateOAuthClient(ctx, domain.OAuthClient{ClientID: "bad", ClientName: "x", RedirectURIs: nil, RegisteredVia: "dcr"}); err == nil {
		t.Fatal("CHECK on redirect_uris cardinality must reject an empty list")
	}

	if _, err := f.repo.GetOAuthClientTenantStatus(ctx, f.tenantID, f.clientID); err != usecase.ErrOAuthClientStatusNotFound {
		t.Fatalf("want ErrOAuthClientStatusNotFound, got %v", err)
	}
	st, err := f.repo.EnsureOAuthClientTenantStatus(ctx, domain.OAuthClientTenantStatus{TenantID: f.tenantID, ClientID: f.clientID, Status: domain.OAuthClientPending, UpdatedAt: time.Now()})
	if err != nil || st.Status != domain.OAuthClientPending {
		t.Fatalf("ensure = %+v %v", st, err)
	}
	if err := f.repo.SetOAuthClientTenantStatus(ctx, domain.OAuthClientTenantStatus{TenantID: f.tenantID, ClientID: f.clientID, Status: domain.OAuthClientBlocked, UpdatedBy: f.userID, UpdatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	// A later ensure (new authorize attempt) must not undo the block.
	st, _ = f.repo.EnsureOAuthClientTenantStatus(ctx, domain.OAuthClientTenantStatus{TenantID: f.tenantID, ClientID: f.clientID, Status: domain.OAuthClientAllowed, UpdatedAt: time.Now()})
	if st.Status != domain.OAuthClientBlocked || st.UpdatedBy != f.userID {
		t.Fatalf("ensure overwrote block: %+v", st)
	}
	if err := f.repo.SetOAuthClientTenantStatus(ctx, domain.OAuthClientTenantStatus{TenantID: uuid.NewString(), ClientID: f.clientID, Status: domain.OAuthClientAllowed}); err != usecase.ErrOAuthClientStatusNotFound {
		t.Fatalf("other tenant has no row: %v", err)
	}

	views, err := f.repo.ListOAuthClientViews(ctx, f.tenantID)
	if err != nil || len(views) != 1 || views[0].Status.Status != domain.OAuthClientBlocked || views[0].Client.ClientName != "App" {
		t.Fatalf("views = %+v err = %v", views, err)
	}
	if other, _ := f.repo.ListOAuthClientViews(ctx, uuid.NewString()); len(other) != 0 {
		t.Fatal("another tenant sees this tenant's clients")
	}
}

func TestOAuthCode_ClaimIsAtomicUnderConcurrency(t *testing.T) {
	f := newOAuthFixture(t)
	fam, code := f.family(t, uuid.NewString())

	const racers = 12
	var won, lost atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := f.repo.ClaimOAuthAuthCode(context.Background(), code.CodeHash, time.Now(), domain.OAuthRefreshToken{
				TokenHash: uuid.NewString(), FamilyID: fam.FamilyID, TenantID: f.tenantID, ExpiresAt: time.Now().Add(time.Hour),
			})
			switch err {
			case nil:
				won.Add(1)
			case usecase.ErrOAuthCodeAlreadyUsed:
				lost.Add(1)
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if won.Load() != 1 || lost.Load() != racers-1 {
		t.Fatalf("won=%d lost=%d, want exactly one winner", won.Load(), lost.Load())
	}
	got, _ := f.repo.GetOAuthAuthCode(context.Background(), code.CodeHash)
	if got.UsedAt == nil {
		t.Fatal("used_at not set")
	}
}

func TestOAuthRefresh_RotationIsAtomicUnderConcurrency(t *testing.T) {
	f := newOAuthFixture(t)
	ctx := context.Background()
	fam, code := f.family(t, uuid.NewString())
	first := domain.OAuthRefreshToken{TokenHash: "first-" + uuid.NewString(), FamilyID: fam.FamilyID, TenantID: f.tenantID, ExpiresAt: time.Now().Add(time.Hour)}
	if err := f.repo.ClaimOAuthAuthCode(ctx, code.CodeHash, time.Now(), first); err != nil {
		t.Fatal(err)
	}

	var won atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := f.repo.RotateOAuthRefreshToken(ctx, first.TokenHash, time.Now(), domain.OAuthRefreshToken{
				TokenHash: uuid.NewString(), FamilyID: fam.FamilyID, TenantID: f.tenantID, ExpiresAt: time.Now().Add(time.Hour),
			})
			if err == nil {
				won.Add(1)
			} else if err != usecase.ErrOAuthRefreshAlreadyUsed {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	if won.Load() != 1 {
		t.Fatalf("%d rotations won, want exactly 1", won.Load())
	}
	rt, _ := f.repo.GetOAuthRefreshToken(ctx, first.TokenHash)
	if rt.UsedAt == nil || rt.ReplacedByHash == "" {
		t.Fatalf("rotated token not marked: %+v", rt)
	}
	if _, err := f.repo.GetOAuthRefreshToken(ctx, "ghost"); err != usecase.ErrOAuthRefreshNotFound {
		t.Fatalf("want ErrOAuthRefreshNotFound, got %v", err)
	}
}

func TestOAuthRevocation_GrantFamilyClientAndTenantIsolation(t *testing.T) {
	f := newOAuthFixture(t)
	ctx := context.Background()
	grant := uuid.NewString()
	famA, _ := f.family(t, grant)
	famB, _ := f.family(t, grant)
	famC, _ := f.family(t, uuid.NewString())
	now := time.Now().UTC()

	// Another tenant cannot touch this grant, and its revocation row does not
	// mark this tenant's grant revoked.
	if n, err := f.repo.RevokeOAuthGrant(ctx, uuid.NewString(), grant, domain.OAuthRevokeAdminRevoked, now); err != nil || n != 0 {
		t.Fatalf("foreign tenant revoke n=%d err=%v", n, err)
	}
	if revoked, _ := f.repo.IsOAuthGrantRevoked(ctx, f.tenantID, grant); revoked {
		t.Fatal("foreign tenant marked this tenant's grant revoked")
	}

	n, err := f.repo.RevokeOAuthGrant(ctx, f.tenantID, grant, domain.OAuthRevokeUserRevoked, now)
	if err != nil || n != 2 {
		t.Fatalf("revoke n=%d err=%v", n, err)
	}
	n, err = f.repo.RevokeOAuthGrant(ctx, f.tenantID, grant, domain.OAuthRevokeUserRevoked, now)
	if err != nil || n != 0 {
		t.Fatalf("idempotent revoke n=%d err=%v", n, err)
	}
	if revoked, _ := f.repo.IsOAuthGrantRevoked(ctx, f.tenantID, grant); !revoked {
		t.Fatal("grant not marked revoked")
	}
	for _, id := range []string{famA.FamilyID, famB.FamilyID} {
		got, _ := f.repo.GetOAuthFamily(ctx, id)
		if got.RevokedAt == nil || got.RevokeReason != domain.OAuthRevokeUserRevoked {
			t.Fatalf("family %s not revoked: %+v", id, got)
		}
	}
	if got, _ := f.repo.GetOAuthFamily(ctx, famC.FamilyID); got.RevokedAt != nil {
		t.Fatal("unrelated grant's family was revoked")
	}

	if n, err := f.repo.RevokeOAuthFamiliesForClient(ctx, f.tenantID, f.clientID, domain.OAuthRevokeClientBlocked, now); err != nil || n != 1 {
		t.Fatalf("client revoke n=%d err=%v (only famC was still live)", n, err)
	}
	// First reason wins: a later revoke must not rewrite why it was revoked.
	if err := f.repo.RevokeOAuthFamily(ctx, famA.FamilyID, domain.OAuthRevokeReuseDetected, now); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.repo.GetOAuthFamily(ctx, famA.FamilyID); got.RevokeReason != domain.OAuthRevokeUserRevoked {
		t.Fatalf("reason rewritten to %q", got.RevokeReason)
	}
	if _, err := f.repo.GetOAuthFamily(ctx, uuid.NewString()); err != usecase.ErrOAuthFamilyNotFound {
		t.Fatalf("want ErrOAuthFamilyNotFound, got %v", err)
	}
}

type staticTestSigner struct{}

func (staticTestSigner) Sign(context.Context, jwtauth.Claims) (string, error) {
	return "header.payload.sig", nil
}
func (staticTestSigner) PublicJWKS(context.Context) (jose.JSONWebKeySet, error) {
	return jose.JSONWebKeySet{}, nil
}

// Full flow against the real database: authorize -> exchange -> replay kills
// the family -> refresh with the original refresh token is refused.
func TestOAuthFlow_ReplayAndReuseAgainstRealDatabase(t *testing.T) {
	f := newOAuthFixture(t)
	ctx := context.Background()
	const resource = "https://orca.example.com/mcp"
	cfg := usecase.OAuthConfig{ResourceURL: resource}
	clock := usecase.SystemClock{}

	if _, err := f.repo.EnsureOAuthClientTenantStatus(ctx, domain.OAuthClientTenantStatus{TenantID: f.tenantID, ClientID: f.clientID, Status: domain.OAuthClientAllowed, UpdatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	verifier := "v" + base64.RawURLEncoding.EncodeToString([]byte(uuid.NewString() + uuid.NewString()))[:50]
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	grant := uuid.NewString()

	issue := usecase.NewOAuthIssueAuthCode(f.repo, f.repo, f.repo, clock, cfg)
	out, err := issue.Execute(ctx, usecase.OAuthIssueAuthCodeInput{
		TenantID: f.tenantID, UserID: f.userID, ClientID: f.clientID, RedirectURI: "https://app.example.com/cb",
		Scopes: []string{"orca:read"}, CodeChallenge: challenge, Resource: resource, GrantID: grant,
	})
	if err != nil {
		t.Fatal(err)
	}
	ex := usecase.NewOAuthExchangeToken(f.repo, f.repo, staticTestSigner{}, f.repo, clock, cfg)
	in := usecase.OAuthExchangeTokenInput{GrantType: "authorization_code", Code: out.Code, RedirectURI: "https://app.example.com/cb", CodeVerifier: verifier, ClientID: f.clientID, Resource: resource}
	tok, err := ex.Execute(ctx, in)
	if err != nil || tok.RefreshToken == "" {
		t.Fatalf("exchange: %+v %v", tok, err)
	}
	if _, err := ex.Execute(ctx, in); err == nil {
		t.Fatal("code replay accepted")
	}
	_, err = ex.Execute(ctx, usecase.OAuthExchangeTokenInput{GrantType: "refresh_token", RefreshToken: tok.RefreshToken, ClientID: f.clientID})
	if err == nil {
		t.Fatal("refresh accepted after code replay revoked the family")
	}
	// The audit trail recorded the replay without any secret in it.
	entries, _, err := f.repo.Query(ctx, usecase.AuditQueryFilter{TenantID: f.tenantID, Since: time.Now().Add(-time.Hour), Action: "oauth.code_replay"}, "", 10)
	if err != nil || len(entries) != 1 {
		t.Fatalf("audit entries = %d err = %v", len(entries), err)
	}
}
