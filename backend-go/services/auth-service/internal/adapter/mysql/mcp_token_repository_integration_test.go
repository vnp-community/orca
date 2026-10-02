//go:build integration

package mysql

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

func mcpTokenFixture(t *testing.T) (*Repository, string, string) {
	t.Helper()
	repo := setupRepository(t)
	tenantID := uuid.NewString()
	return repo, tenantID, createTestUser(t, repo, tenantID, "pat@example.com").ID
}

func newMcpToken(tenantID, userID string, ttl time.Duration) domain.McpToken {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return domain.McpToken{JTI: uuid.NewString(), TenantID: tenantID, UserID: userID, Name: "ci", Scope: "orca:read",
		TokenSHA256: uuid.NewString(), CreatedAt: now, ExpiresAt: now.Add(ttl)}
}

func TestMcpTokensMigration_UpDownUp(t *testing.T) {
	dsn := testutil.StartMySQL(t, "auth")
	path, _ := filepath.Abs("../../../migrations/mysql")
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

func TestMcpToken_CRUDOwnershipAndUsage(t *testing.T) {
	repo, tenantID, userID := mcpTokenFixture(t)
	ctx := context.Background()
	tok := newMcpToken(tenantID, userID, 24*time.Hour)
	if err := repo.CreateMcpToken(ctx, tok); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetMcpToken(ctx, tenantID, tok.JTI)
	if err != nil || got.Name != "ci" || got.TokenSHA256 != tok.TokenSHA256 || got.RevokedAt != nil {
		t.Fatalf("get = %+v %v", got, err)
	}
	if _, err := repo.GetMcpToken(ctx, uuid.NewString(), tok.JTI); err != usecase.ErrMcpTokenNotFound {
		t.Fatalf("other tenant must not see token: %v", err)
	}
	if l, _ := repo.ListMcpTokens(ctx, tenantID, userID); len(l) != 1 {
		t.Fatalf("list = %d", len(l))
	}
	if l, _ := repo.ListMcpTokens(ctx, tenantID, uuid.NewString()); len(l) != 0 {
		t.Fatal("other user must see nothing")
	}

	first, err := repo.MarkMcpTokenFirstUsed(ctx, tenantID, tok.JTI, time.Now())
	if err != nil || !first {
		t.Fatalf("first use = %v %v", first, err)
	}
	if again, _ := repo.MarkMcpTokenFirstUsed(ctx, tenantID, tok.JTI, time.Now()); again {
		t.Fatal("first_used must be set exactly once")
	}
	now := time.Now().UTC()
	if err := repo.TouchMcpTokenLastUsed(ctx, tenantID, tok.JTI, now, now.Add(-5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.GetMcpToken(ctx, tenantID, tok.JTI)
	if got.LastUsedAt == nil || got.FirstUsedAt == nil {
		t.Fatalf("usage not recorded: %+v", got)
	}

	if err := repo.RevokeMcpToken(ctx, tenantID, uuid.NewString(), tok.JTI, now); err != usecase.ErrMcpTokenNotFound {
		t.Fatalf("non-owner revoke: %v", err)
	}
	if err := repo.RevokeMcpToken(ctx, tenantID, userID, tok.JTI, now); err != nil {
		t.Fatal(err)
	}
	if err := repo.RevokeMcpToken(ctx, tenantID, userID, tok.JTI, now); err != usecase.ErrMcpTokenNotFound {
		t.Fatalf("double revoke: %v", err)
	}
	got, _ = repo.GetMcpToken(ctx, tenantID, tok.JTI)
	if got.RevokedAt == nil || got.RevokedBy != userID {
		t.Fatalf("revocation not recorded: %+v", got)
	}
}

func TestMcpToken_Constraints(t *testing.T) {
	repo, tenantID, userID := mcpTokenFixture(t)
	ctx := context.Background()
	if err := repo.CreateMcpToken(ctx, newMcpToken(tenantID, userID, 91*24*time.Hour)); err == nil {
		t.Fatal("CHECK must reject a token longer than 90 days")
	}
	if err := repo.CreateMcpToken(ctx, newMcpToken(tenantID, userID, 90*24*time.Hour)); err != nil {
		t.Fatalf("exactly 90 days must pass: %v", err)
	}
	if err := repo.CreateMcpToken(ctx, newMcpToken(tenantID, userID, -time.Hour)); err == nil {
		t.Fatal("CHECK must reject expires_at <= created_at")
	}
	a := newMcpToken(tenantID, userID, time.Hour)
	b := newMcpToken(tenantID, userID, time.Hour)
	b.TokenSHA256 = a.TokenSHA256
	if err := repo.CreateMcpToken(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateMcpToken(ctx, b); err == nil {
		t.Fatal("token_sha256 must be unique")
	}
	long := newMcpToken(tenantID, userID, time.Hour)
	long.Name = "0123456789012345678901234567890123456789012345678901234567890123456789012345678901"
	if err := repo.CreateMcpToken(ctx, long); err == nil {
		t.Fatal("CHECK must reject names over 80 chars")
	}
}
