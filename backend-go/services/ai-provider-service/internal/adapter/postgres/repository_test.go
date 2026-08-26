//go:build integration

// Integration tests run against a real Postgres via testcontainers-go, per
// specs/backend-go/standards/testing-strategy.md — gated behind the
// "integration" build tag so `go test ./...` (unit tests only) stays fast
// and Docker-free; run these explicitly with
// `go test -tags=integration ./internal/adapter/postgres/...`.
package postgres

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/usecase"
)

func setupRepository(t *testing.T) *Repository {
	t.Helper()
	dsn := testutil.StartPostgres(t, "ai_provider")

	migrationsPath, err := filepath.Abs("../../../migrations")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	// Uses the golang-migrate CLI directly rather than importing the
	// library, keeping this test's dependency footprint minimal — swap for
	// the library-based runner once the shared migration-runner helper
	// (referenced in architecture/05-data-architecture.md) exists in common/.
	cmd := exec.Command("migrate", "-path", migrationsPath, "-database", dsn, "up")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running migrations: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting to postgres: %v", err)
	}
	t.Cleanup(pool.Close)

	return New(pool)
}

func TestRepository_CreateAndGet_RoundTrips(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	account, err := domain.NewProviderAccount("acc-1", "11111111-1111-1111-1111-111111111111",
		domain.ProviderTypeAnthropic, domain.AccountStatusPending, "cred-ref-1",
		domain.ScopeServer, "", "", "", nil, now, now)
	if err != nil {
		t.Fatalf("building account: %v", err)
	}
	if err := repo.Create(ctx, account); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.Get(ctx, account.TenantID, account.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.CredentialRef != "cred-ref-1" {
		t.Errorf("expected credential_ref to round-trip, got %q", got.CredentialRef)
	}
	if got.Status != domain.AccountStatusPending {
		t.Errorf("expected status pending, got %q", got.Status)
	}
}

func TestRepository_List_FiltersByTenantAndScope(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	userAccount, _ := domain.NewProviderAccount("acc-user", "11111111-1111-1111-1111-111111111111",
		domain.ProviderTypeAnthropic, domain.AccountStatusActive, "cred-ref-1",
		domain.ScopeUser, "22222222-2222-2222-2222-222222222222", "", "", nil, now, now)
	otherTenant, _ := domain.NewProviderAccount("acc-other", "33333333-3333-3333-3333-333333333333",
		domain.ProviderTypeAnthropic, domain.AccountStatusActive, "cred-ref-2",
		domain.ScopeUser, "22222222-2222-2222-2222-222222222222", "", "", nil, now, now)
	_ = repo.Create(ctx, userAccount)
	_ = repo.Create(ctx, otherTenant)

	accounts, err := repo.List(ctx, usecase.ListAccountsFilter{
		TenantID: "11111111-1111-1111-1111-111111111111", Scope: domain.ScopeUser,
		ScopeRefID: "22222222-2222-2222-2222-222222222222",
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(accounts) != 1 || accounts[0].ID != "acc-user" {
		t.Errorf("expected only the matching tenant's user-scope account, got %+v", accounts)
	}
}

func TestRepository_UpdateStatus_UpdatesCredentialRefOnRotation(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	account, _ := domain.NewProviderAccount("acc-1", "11111111-1111-1111-1111-111111111111",
		domain.ProviderTypeAnthropic, domain.AccountStatusActive, "cred-ref-old",
		domain.ScopeServer, "", "", "", nil, now, now)
	_ = repo.Create(ctx, account)

	updated, err := repo.UpdateStatus(ctx, usecase.UpdateStatusInput{
		TenantID: account.TenantID, AccountID: account.ID,
		Status: domain.AccountStatusRotating, CredentialRef: "cred-ref-new",
	})
	if err != nil {
		t.Fatalf("update status: %v", err)
	}
	if updated.Status != domain.AccountStatusRotating || updated.CredentialRef != "cred-ref-new" {
		t.Errorf("expected rotating status + new credential_ref, got %+v", updated)
	}
}
