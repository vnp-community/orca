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
// "operational bookkeeping, not a copy of provider data" posture (BR-PI-01).
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
