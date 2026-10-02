//go:build integration

package mysql

import (
	"context"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

func newOAuthMySQLFixture(t *testing.T) (*Repository, string, string, string) {
	t.Helper()
	repo := setupRepository(t)
	tenantID := uuid.NewString()
	user := createTestUser(t, repo, tenantID, "oauth@example.com")
	clientID := "client-" + uuid.NewString()
	if err := repo.CreateOAuthClient(context.Background(), domain.OAuthClient{
		ClientID: clientID, ClientName: "App", RedirectURIs: []string{"https://app.example.com/cb", "http://127.0.0.1/cb"},
		RegisteredVia: "dcr", CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	return repo, tenantID, user.ID, clientID
}

func oauthFamily(t *testing.T, repo *Repository, tenantID, userID, clientID, grantID string) (domain.OAuthTokenFamily, domain.OAuthAuthCode) {
	t.Helper()
	fam := domain.OAuthTokenFamily{FamilyID: uuid.NewString(), TenantID: tenantID, UserID: userID, ClientID: clientID, GrantID: grantID,
		Scope: "orca:read", Resource: "https://orca.example.com/mcp", CreatedAt: time.Now().UTC()}
	code := domain.OAuthAuthCode{CodeHash: uuid.NewString(), FamilyID: fam.FamilyID, TenantID: tenantID,
		RedirectURI: "https://app.example.com/cb", CodeChallenge: "c", ExpiresAt: time.Now().UTC().Add(time.Minute)}
	if err := repo.CreateOAuthFamilyWithCode(context.Background(), fam, code); err != nil {
		t.Fatal(err)
	}
	return fam, code
}

func TestOAuthMySQL_ClientAndStatus(t *testing.T) {
	repo, tenantID, _, clientID := newOAuthMySQLFixture(t)
	ctx := context.Background()

	c, err := repo.GetOAuthClient(ctx, clientID)
	if err != nil || len(c.RedirectURIs) != 2 || c.RedirectURIs[1] != "http://127.0.0.1/cb" {
		t.Fatalf("client = %+v err = %v", c, err)
	}
	if _, err := repo.GetOAuthClient(ctx, "nope"); err != usecase.ErrOAuthClientNotFound {
		t.Fatalf("want not found, got %v", err)
	}
	if _, err := repo.GetOAuthClientTenantStatus(ctx, tenantID, clientID); err != usecase.ErrOAuthClientStatusNotFound {
		t.Fatalf("want status not found, got %v", err)
	}
	if _, err := repo.EnsureOAuthClientTenantStatus(ctx, domain.OAuthClientTenantStatus{TenantID: tenantID, ClientID: clientID, Status: domain.OAuthClientPending, UpdatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	same := domain.OAuthClientTenantStatus{TenantID: tenantID, ClientID: clientID, Status: domain.OAuthClientBlocked, UpdatedAt: time.Now().Truncate(time.Second)}
	if err := repo.SetOAuthClientTenantStatus(ctx, same); err != nil {
		t.Fatal(err)
	}
	// Re-applying identical values must still succeed (MySQL reports 0 changed rows).
	if err := repo.SetOAuthClientTenantStatus(ctx, same); err != nil {
		t.Fatalf("idempotent set failed: %v", err)
	}
	st, _ := repo.EnsureOAuthClientTenantStatus(ctx, domain.OAuthClientTenantStatus{TenantID: tenantID, ClientID: clientID, Status: domain.OAuthClientAllowed, UpdatedAt: time.Now()})
	if st.Status != domain.OAuthClientBlocked {
		t.Fatalf("ensure overwrote block: %+v", st)
	}
	if err := repo.SetOAuthClientTenantStatus(ctx, domain.OAuthClientTenantStatus{TenantID: uuid.NewString(), ClientID: clientID, Status: domain.OAuthClientAllowed}); err != usecase.ErrOAuthClientStatusNotFound {
		t.Fatalf("other tenant: %v", err)
	}
	views, err := repo.ListOAuthClientViews(ctx, tenantID)
	if err != nil || len(views) != 1 || views[0].Status.Status != domain.OAuthClientBlocked {
		t.Fatalf("views = %+v err = %v", views, err)
	}
}

func TestOAuthMySQL_ClaimAndRotateAreAtomic(t *testing.T) {
	repo, tenantID, userID, clientID := newOAuthMySQLFixture(t)
	ctx := context.Background()
	fam, code := oauthFamily(t, repo, tenantID, userID, clientID, uuid.NewString())

	var winner atomic.Value
	var won atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tok := domain.OAuthRefreshToken{TokenHash: uuid.NewString(), FamilyID: fam.FamilyID, TenantID: tenantID, ExpiresAt: time.Now().Add(time.Hour)}
			err := repo.ClaimOAuthAuthCode(ctx, code.CodeHash, time.Now(), tok)
			if err == nil {
				won.Add(1)
				winner.Store(tok.TokenHash)
			} else if err != usecase.ErrOAuthCodeAlreadyUsed {
				t.Errorf("unexpected: %v", err)
			}
		}()
	}
	wg.Wait()
	if won.Load() != 1 {
		t.Fatalf("%d claims won, want 1", won.Load())
	}
	// Only the winner's refresh token may exist: losers' inserts rolled back.
	rotTarget := winner.Load().(string)

	won.Store(0)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := repo.RotateOAuthRefreshToken(ctx, rotTarget, time.Now(), domain.OAuthRefreshToken{
				TokenHash: uuid.NewString(), FamilyID: fam.FamilyID, TenantID: tenantID, ExpiresAt: time.Now().Add(time.Hour)})
			if err == nil {
				won.Add(1)
			} else if err != usecase.ErrOAuthRefreshAlreadyUsed {
				t.Errorf("unexpected: %v", err)
			}
		}()
	}
	wg.Wait()
	if won.Load() != 1 {
		t.Fatalf("%d rotations won, want 1", won.Load())
	}
}

func TestOAuthMySQL_RevocationAndTenantIsolation(t *testing.T) {
	repo, tenantID, userID, clientID := newOAuthMySQLFixture(t)
	ctx := context.Background()
	grant := uuid.NewString()
	famA, _ := oauthFamily(t, repo, tenantID, userID, clientID, grant)
	oauthFamily(t, repo, tenantID, userID, clientID, grant)
	famC, _ := oauthFamily(t, repo, tenantID, userID, clientID, uuid.NewString())
	now := time.Now().UTC()

	if n, err := repo.RevokeOAuthGrant(ctx, uuid.NewString(), grant, domain.OAuthRevokeAdminRevoked, now); err != nil || n != 0 {
		t.Fatalf("foreign tenant n=%d err=%v", n, err)
	}
	if r, _ := repo.IsOAuthGrantRevoked(ctx, tenantID, grant); r {
		t.Fatal("foreign tenant revoked this grant")
	}
	if n, err := repo.RevokeOAuthGrant(ctx, tenantID, grant, domain.OAuthRevokeUserRevoked, now); err != nil || n != 2 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if n, err := repo.RevokeOAuthGrant(ctx, tenantID, grant, domain.OAuthRevokeUserRevoked, now); err != nil || n != 0 {
		t.Fatalf("idempotent n=%d err=%v", n, err)
	}
	if r, _ := repo.IsOAuthGrantRevoked(ctx, tenantID, grant); !r {
		t.Fatal("grant not revoked")
	}
	if got, _ := repo.GetOAuthFamily(ctx, famA.FamilyID); got.RevokedAt == nil || got.RevokeReason != "user_revoked" {
		t.Fatalf("famA = %+v", got)
	}
	if n, err := repo.RevokeOAuthFamiliesForClient(ctx, tenantID, clientID, domain.OAuthRevokeClientBlocked, now); err != nil || n != 1 {
		t.Fatalf("client revoke n=%d err=%v", n, err)
	}
	if got, _ := repo.GetOAuthFamily(ctx, famC.FamilyID); got.RevokeReason != "client_blocked" {
		t.Fatalf("famC = %+v", got)
	}
}

func TestOAuthMySQL_MigrationUpDownUp(t *testing.T) {
	dsn := testutil.StartMySQL(t, "auth")
	path, _ := filepath.Abs("../../../migrations/mysql")
	for _, args := range [][]string{{"up"}, {"down", "1"}, {"up"}} {
		full := append([]string{"-path", path, "-database", dsn}, args...)
		if out, err := exec.Command("migrate", full...).CombinedOutput(); err != nil {
			t.Fatalf("migrate %v: %v\n%s", args, err, out)
		}
	}
}
