package mysql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

// IssueListCacheRepository implements usecase.IssueListCache against MySQL
// — sibling of RateLimitCacheRepository, same "operational bookkeeping, not
// a copy of provider data" posture (BR-PI-01).
type IssueListCacheRepository struct {
	db *sql.DB
}

func NewIssueListCache(db *sql.DB) *IssueListCacheRepository {
	return &IssueListCacheRepository{db: db}
}

var _ usecase.IssueListCache = (*IssueListCacheRepository)(nil)

// filterHash mirrors internal/adapter/postgres.filterHash exactly (same
// sha256-of-normalized-filter scheme, dialect-independent).
func filterHash(key usecase.IssueCacheKey) string {
	b, _ := json.Marshal(key.Filter)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (r *IssueListCacheRepository) Get(ctx context.Context, key usecase.IssueCacheKey) (usecase.CachedIssueList, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT issues_json, cached_at FROM issue_list_cache
		WHERE tenant_id = ? AND provider = ? AND repo = ? AND filter_hash = ? AND expires_at > NOW(6)
	`, key.TenantID, string(key.Provider), key.Repo, filterHash(key))

	var raw []byte
	var cachedAt time.Time
	if err := row.Scan(&raw, &cachedAt); errors.Is(err, sql.ErrNoRows) {
		return usecase.CachedIssueList{}, false, nil
	} else if err != nil {
		return usecase.CachedIssueList{}, false, fmt.Errorf("mysql: query issue list cache: %w", err)
	}
	var issues []domain.Issue
	if err := json.Unmarshal(raw, &issues); err != nil {
		return usecase.CachedIssueList{}, false, fmt.Errorf("mysql: decode cached issues: %w", err)
	}
	return usecase.CachedIssueList{Issues: issues, CachedAt: cachedAt}, true, nil
}

// Put upserts by (tenant_id, provider, repo, filter_hash) via
// ON DUPLICATE KEY UPDATE against issue_list_cache's UNIQUE key. id has no
// MySQL-side DEFAULT (unlike Postgres's `DEFAULT gen_random_uuid()`: MySQL
// rejects UUID() in a column DEFAULT as non-deterministic) — generated
// here in Go instead. id is never read back by Get, so this is a pure
// plumbing detail — see migrations/mysql/0002_issue_list_cache.up.sql.
func (r *IssueListCacheRepository) Put(ctx context.Context, key usecase.IssueCacheKey, issues []domain.Issue, cachedAt time.Time, ttl time.Duration) error {
	raw, err := json.Marshal(issues)
	if err != nil {
		return fmt.Errorf("mysql: encode issues for cache: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO issue_list_cache (id, tenant_id, provider, repo, filter_hash, issues_json, cached_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			issues_json = VALUES(issues_json), cached_at = VALUES(cached_at), expires_at = VALUES(expires_at)
	`, uuid.NewString(), key.TenantID, string(key.Provider), key.Repo, filterHash(key), raw, cachedAt, cachedAt.Add(ttl))
	if err != nil {
		return fmt.Errorf("mysql: upsert issue list cache: %w", err)
	}
	return nil
}
