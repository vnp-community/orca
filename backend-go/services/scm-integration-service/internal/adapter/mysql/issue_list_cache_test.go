//go:build integration

package mysql

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

func setupIssueListCacheRepository(t *testing.T) *IssueListCacheRepository {
	t.Helper()
	return NewIssueListCache(setupMySQLDB(t))
}

func TestIssueListCacheRepository_MissWhenNothingStored(t *testing.T) {
	repo := setupIssueListCacheRepository(t)
	key := usecase.IssueCacheKey{TenantID: testTenant1, Provider: domain.ScmProviderGitHub, Repo: "acme/widgets"}

	_, ok, err := repo.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected a miss for a key with nothing cached")
	}
}

func TestIssueListCacheRepository_PutThenGet_RoundTrips(t *testing.T) {
	repo := setupIssueListCacheRepository(t)
	ctx := context.Background()
	key := usecase.IssueCacheKey{TenantID: testTenant1, Provider: domain.ScmProviderGitHub, Repo: "acme/widgets", Filter: usecase.IssueFilter{State: "open"}}
	issues := []domain.Issue{{Number: 1, Title: "first"}, {Number: 2, Title: "second"}}
	cachedAt := time.Now().Truncate(time.Second)

	if err := repo.Put(ctx, key, issues, cachedAt, 5*time.Minute); err != nil {
		t.Fatalf("unexpected error putting: %v", err)
	}

	got, ok, err := repo.Get(ctx, key)
	if err != nil {
		t.Fatalf("unexpected error getting: %v", err)
	}
	if !ok {
		t.Fatal("expected a hit right after Put")
	}
	if len(got.Issues) != 2 || got.Issues[0].Title != "first" || got.Issues[1].Title != "second" {
		t.Errorf("unexpected round-tripped issues: %+v", got.Issues)
	}
}

func TestIssueListCacheRepository_Get_MissAfterExpiry(t *testing.T) {
	repo := setupIssueListCacheRepository(t)
	ctx := context.Background()
	key := usecase.IssueCacheKey{TenantID: testTenant1, Provider: domain.ScmProviderGitHub, Repo: "acme/widgets"}

	// A negative TTL sets expires_at in the past — the WHERE expires_at >
	// NOW(6) filter in Get must treat this as a miss, not a stale hit.
	if err := repo.Put(ctx, key, []domain.Issue{{Number: 1}}, time.Now(), -time.Hour); err != nil {
		t.Fatalf("unexpected error putting: %v", err)
	}

	_, ok, err := repo.Get(ctx, key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected a miss for an already-expired cache row")
	}
}

func TestIssueListCacheRepository_Put_IsUpsertOnSameKey(t *testing.T) {
	repo := setupIssueListCacheRepository(t)
	ctx := context.Background()
	key := usecase.IssueCacheKey{TenantID: testTenant1, Provider: domain.ScmProviderGitHub, Repo: "acme/widgets"}

	_ = repo.Put(ctx, key, []domain.Issue{{Number: 1, Title: "stale"}}, time.Now(), 5*time.Minute)
	_ = repo.Put(ctx, key, []domain.Issue{{Number: 2, Title: "fresh"}}, time.Now(), 5*time.Minute)

	got, ok, err := repo.Get(ctx, key)
	if err != nil || !ok {
		t.Fatalf("unexpected result: ok=%v err=%v", ok, err)
	}
	if len(got.Issues) != 1 || got.Issues[0].Title != "fresh" {
		t.Errorf("expected the second Put to overwrite the first via ON DUPLICATE KEY UPDATE, got %+v", got.Issues)
	}
}

// TestIssueListCacheRepository_DoesNotLeakAcrossTenants is this table's
// TASK-BE-DB-003 tenant-isolation-without-RLS test — two tenants caching
// the SAME (provider, repo, filter) must not see each other's issues, with
// no RLS backstop on MySQL (migrations/mysql/0002_issue_list_cache.up.sql).
func TestIssueListCacheRepository_DoesNotLeakAcrossTenants(t *testing.T) {
	repo := setupIssueListCacheRepository(t)
	ctx := context.Background()
	key1 := usecase.IssueCacheKey{TenantID: testTenant1, Provider: domain.ScmProviderGitHub, Repo: "acme/widgets"}
	key2 := usecase.IssueCacheKey{TenantID: testTenant2, Provider: domain.ScmProviderGitHub, Repo: "acme/widgets"}

	_ = repo.Put(ctx, key1, []domain.Issue{{Number: 1, Title: "tenant-1 issue"}}, time.Now(), 5*time.Minute)
	_ = repo.Put(ctx, key2, []domain.Issue{{Number: 2, Title: "tenant-2 issue"}}, time.Now(), 5*time.Minute)

	got1, ok, err := repo.Get(ctx, key1)
	if err != nil || !ok {
		t.Fatalf("unexpected result for tenant-1: ok=%v err=%v", ok, err)
	}
	if len(got1.Issues) != 1 || got1.Issues[0].Title != "tenant-1 issue" {
		t.Errorf("tenant-1's Get leaked tenant-2's row: %+v", got1.Issues)
	}

	got2, ok, err := repo.Get(ctx, key2)
	if err != nil || !ok {
		t.Fatalf("unexpected result for tenant-2: ok=%v err=%v", ok, err)
	}
	if len(got2.Issues) != 1 || got2.Issues[0].Title != "tenant-2 issue" {
		t.Errorf("tenant-2's Get leaked tenant-1's row: %+v", got2.Issues)
	}
}
