# TASK-PI-01-04: `issue_list_cache` Postgres adapter + migration

**From Solution:** SOL-PI-01
**Priority:** P1
**Service:** `scm-integration-service`
**File:** `backend-go/services/scm-integration-service/internal/adapter/postgres/issue_list_cache.go` (new), `backend-go/services/scm-integration-service/migrations/0002_issue_list_cache.up.sql` (new)
**Depends on:** TASK-PI-01-02
**Status:** `[ ]` TODO

---

## Context

BR-PI-01's 5-minute issue cache is a sibling of `rate_limit_cache`
(`scm-integration-service.md` §5: "a hot local read... not a source of
truth"), in the same database, following the exact adapter shape
`adapter/postgres/rate_limit_cache.go` already establishes. This task
implements `usecase.IssueListCache` against a new table.

## Changes to make

`migrations/0002_issue_list_cache.up.sql` (new, next number after
`0001_init`):

```sql
CREATE TABLE scm.issue_list_cache (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    provider        TEXT NOT NULL,
    repo            TEXT NOT NULL,
    filter_hash     TEXT NOT NULL,      -- sha256 of the normalized IssueFilter
    issues_json     JSONB NOT NULL,
    cached_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ NOT NULL,
    UNIQUE (tenant_id, provider, repo, filter_hash)
);
CREATE INDEX idx_issue_list_cache_expires ON scm.issue_list_cache (expires_at);

ALTER TABLE scm.issue_list_cache ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON scm.issue_list_cache
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
```

Corresponding `0002_issue_list_cache.down.sql`: `DROP TABLE scm.issue_list_cache;`

`internal/adapter/postgres/issue_list_cache.go` (new):

```go
package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

// IssueListCacheRepository implements usecase.IssueListCache against
// scm.issue_list_cache — sibling of RateLimitCacheRepository, same
// "operational bookkeeping, not a copy of provider data" posture.
type IssueListCacheRepository struct {
	pool *pgxpool.Pool
}

func NewIssueListCache(pool *pgxpool.Pool) *IssueListCacheRepository {
	return &IssueListCacheRepository{pool: pool}
}

var _ usecase.IssueListCache = (*IssueListCacheRepository)(nil)

func filterHash(key usecase.IssueCacheKey) string {
	b, _ := json.Marshal(key.Filter)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (r *IssueListCacheRepository) Get(ctx context.Context, key usecase.IssueCacheKey) (usecase.CachedIssueList, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT issues_json, cached_at FROM scm.issue_list_cache
		WHERE tenant_id = $1 AND provider = $2 AND repo = $3 AND filter_hash = $4 AND expires_at > now()
	`, key.TenantID, string(key.Provider), key.Repo, filterHash(key))

	var raw []byte
	var cachedAt time.Time
	if err := row.Scan(&raw, &cachedAt); errors.Is(err, pgx.ErrNoRows) {
		return usecase.CachedIssueList{}, false, nil
	} else if err != nil {
		return usecase.CachedIssueList{}, false, fmt.Errorf("postgres: query issue list cache: %w", err)
	}
	var issues []domain.Issue
	if err := json.Unmarshal(raw, &issues); err != nil {
		return usecase.CachedIssueList{}, false, fmt.Errorf("postgres: decode cached issues: %w", err)
	}
	return usecase.CachedIssueList{Issues: issues, CachedAt: cachedAt}, true, nil
}

func (r *IssueListCacheRepository) Put(ctx context.Context, key usecase.IssueCacheKey, issues []domain.Issue, cachedAt time.Time, ttl time.Duration) error {
	raw, err := json.Marshal(issues)
	if err != nil {
		return fmt.Errorf("postgres: encode issues for cache: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO scm.issue_list_cache (tenant_id, provider, repo, filter_hash, issues_json, cached_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (tenant_id, provider, repo, filter_hash) DO UPDATE SET
			issues_json = EXCLUDED.issues_json, cached_at = EXCLUDED.cached_at, expires_at = EXCLUDED.expires_at
	`, key.TenantID, string(key.Provider), key.Repo, filterHash(key), raw, cachedAt, cachedAt.Add(ttl))
	if err != nil {
		return fmt.Errorf("postgres: upsert issue list cache: %w", err)
	}
	return nil
}
```

Wire `NewIssueListCache(pool)` into `cmd/server/main.go`'s composition root
alongside the existing `postgres.New(pool)` (rate-limit cache) call, and pass
it into `usecase.NewListIssues(...)` (TASK-PI-01-03).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/scm-integration-service/...
go vet ./services/scm-integration-service/...
```
