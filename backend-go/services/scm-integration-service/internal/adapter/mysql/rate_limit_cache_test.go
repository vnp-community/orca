//go:build integration

package mysql

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

func setupRateLimitCacheRepository(t *testing.T) *RateLimitCacheRepository {
	t.Helper()
	return New(setupMySQLDB(t))
}

func TestRateLimitCacheRepository_MissWhenNothingStored(t *testing.T) {
	repo := setupRateLimitCacheRepository(t)
	_, ok, err := repo.Get(context.Background(), testTenant1, domain.ScmProviderGitHub, time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected a miss for a tenant/provider with nothing cached")
	}
}

func TestRateLimitCacheRepository_SetThenGet_WithinFreshWindow(t *testing.T) {
	repo := setupRateLimitCacheRepository(t)
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
	repo := setupRateLimitCacheRepository(t)
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
	repo := setupRateLimitCacheRepository(t)
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

// TestRateLimitCacheRepository_ScopedByTenantAndProvider is this table's
// TASK-BE-DB-003 tenant-isolation-without-RLS test: MySQL has no RLS
// equivalent (migrations/mysql/0001_init.up.sql), so this proves the
// application-layer tenant_id filter in Get is the sole isolation
// mechanism, mirroring internal/adapter/postgres's own test of the same
// name 1:1.
func TestRateLimitCacheRepository_ScopedByTenantAndProvider(t *testing.T) {
	repo := setupRateLimitCacheRepository(t)
	ctx := context.Background()

	_ = repo.Set(ctx, testTenant1, domain.ScmProviderGitHub, domain.RateLimitStatus{Remaining: 1, Limit: 1, ResetAt: time.Now()})
	_ = repo.Set(ctx, testTenant1, domain.ScmProviderGitLab, domain.RateLimitStatus{Remaining: 2, Limit: 2, ResetAt: time.Now()})
	_ = repo.Set(ctx, testTenant2, domain.ScmProviderGitHub, domain.RateLimitStatus{Remaining: 3, Limit: 3, ResetAt: time.Now()})

	got, ok, err := repo.Get(ctx, testTenant1, domain.ScmProviderGitHub, time.Minute)
	if err != nil || !ok {
		t.Fatalf("unexpected result: ok=%v err=%v", ok, err)
	}
	if got.Remaining != 1 {
		t.Errorf("expected tenant-1/github's own row, got remaining=%d — tenant-2's write leaked across the primary key", got.Remaining)
	}
}
