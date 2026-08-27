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
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// testTenant1/testTenant2 — scm.rate_limit_cache.tenant_id is UUID (matches
// every other service's tenant_id column in this codebase), so tests need
// real UUID strings, not the short "tenant-1"-style IDs the usecase-layer
// fakes use (those never touch a real column type).
const (
	testTenant1 = "11111111-1111-1111-1111-111111111111"
	testTenant2 = "22222222-2222-2222-2222-222222222222"
)

func setupRepository(t *testing.T) *RateLimitCacheRepository {
	t.Helper()
	dsn := testutil.StartPostgres(t, "scm")

	migrationsPath, err := filepath.Abs("../../../migrations")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	// Uses the golang-migrate CLI directly rather than importing the
	// library — matches usage-service's own integration test (same
	// not-yet-shared-helper rationale noted there).
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

func TestRateLimitCacheRepository_MissWhenNothingStored(t *testing.T) {
	repo := setupRepository(t)
	_, ok, err := repo.Get(context.Background(), testTenant1, domain.ScmProviderGitHub, time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected a miss for a tenant/provider with nothing cached")
	}
}

func TestRateLimitCacheRepository_SetThenGet_WithinFreshWindow(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	want := domain.RateLimitStatus{Remaining: 340, Limit: 5000, ResetAt: time.Now().Add(time.Hour).Truncate(time.Second)}
	if err := repo.Set(ctx, testTenant1, domain.ScmProviderGitHub, want); err != nil {
		t.Fatalf("unexpected error setting: %v", err)
	}

	got, ok, err := repo.Get(ctx, testTenant1, domain.ScmProviderGitHub, time.Minute)
	if err != nil {
		t.Fatalf("unexpected error getting: %v", err)
	}
	if !ok {
		t.Fatal("expected a hit right after Set")
	}
	if got.Remaining != want.Remaining || got.Limit != want.Limit {
		t.Errorf("unexpected cached status: got %+v, want %+v", got, want)
	}
}

func TestRateLimitCacheRepository_Get_StaleOutsideFreshWindow(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	if err := repo.Set(ctx, testTenant1, domain.ScmProviderGitHub, domain.RateLimitStatus{Remaining: 1, Limit: 1, ResetAt: time.Now()}); err != nil {
		t.Fatalf("unexpected error setting: %v", err)
	}

	// freshWithin=0 means "must have been checked in the future", which
	// nothing ever is — proves the freshness window is actually enforced,
	// not just "any row present".
	_, ok, err := repo.Get(ctx, testTenant1, domain.ScmProviderGitHub, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected a miss once the cached row falls outside the freshness window")
	}
}

func TestRateLimitCacheRepository_Set_IsUpsert(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	_ = repo.Set(ctx, testTenant1, domain.ScmProviderGitHub, domain.RateLimitStatus{Remaining: 500, Limit: 5000, ResetAt: time.Now()})
	_ = repo.Set(ctx, testTenant1, domain.ScmProviderGitHub, domain.RateLimitStatus{Remaining: 100, Limit: 5000, ResetAt: time.Now()})

	got, ok, err := repo.Get(ctx, testTenant1, domain.ScmProviderGitHub, time.Minute)
	if err != nil || !ok {
		t.Fatalf("unexpected result: ok=%v err=%v", ok, err)
	}
	if got.Remaining != 100 {
		t.Errorf("expected the second Set to overwrite the first, got remaining=%d", got.Remaining)
	}
}

func TestRateLimitCacheRepository_ScopedByTenantAndProvider(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	_ = repo.Set(ctx, testTenant1, domain.ScmProviderGitHub, domain.RateLimitStatus{Remaining: 1, Limit: 1, ResetAt: time.Now()})
	_ = repo.Set(ctx, testTenant1, domain.ScmProviderGitLab, domain.RateLimitStatus{Remaining: 2, Limit: 2, ResetAt: time.Now()})
	_ = repo.Set(ctx, testTenant2, domain.ScmProviderGitHub, domain.RateLimitStatus{Remaining: 3, Limit: 3, ResetAt: time.Now()})

	got, ok, err := repo.Get(ctx, testTenant1, domain.ScmProviderGitHub, time.Minute)
	if err != nil || !ok {
		t.Fatalf("unexpected result: ok=%v err=%v", ok, err)
	}
	if got.Remaining != 1 {
		t.Errorf("expected tenant-1/github's own row, got remaining=%d", got.Remaining)
	}
}
