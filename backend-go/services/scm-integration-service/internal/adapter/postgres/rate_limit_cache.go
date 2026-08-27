// Package postgres implements usecase.RateLimitCache (this service's only
// database port so far) against scm-integration-service's own Postgres
// database — see specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule: this is the ONLY package in this service that
// knows SQL exists. See scm-integration-service.md §5: this database holds
// operational bookkeeping only, never a copy of provider issue/PR/MR data.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

// rateLimitBucket is always "core" for every provider this service
// supports today — only GitHub exposes multiple buckets per token
// (core/graphql/search), and only its "core" REST bucket is what this
// service's adapter currently reports (see internal/adapter/github's
// GetRateLimitStatus doc comment). The column exists in the schema for
// when that changes; this package always writes/reads the one bucket.
const rateLimitBucket = "core"

// RateLimitCacheRepository implements usecase.RateLimitCache against the
// scm.rate_limit_cache table (migrations/0001_init.up.sql).
type RateLimitCacheRepository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *RateLimitCacheRepository {
	return &RateLimitCacheRepository{pool: pool}
}

var _ usecase.RateLimitCache = (*RateLimitCacheRepository)(nil)

func (r *RateLimitCacheRepository) Get(ctx context.Context, tenantID string, provider domain.ScmProvider, freshWithin time.Duration) (domain.RateLimitStatus, bool, error) {
	// Computed in Go (not `now() - $N::interval`) — a Go time.Duration's
	// string form ("1m0s") isn't a valid Postgres interval literal, and this
	// sidesteps that mismatch entirely.
	cutoff := time.Now().Add(-freshWithin)
	row := r.pool.QueryRow(ctx, `
		SELECT remaining, "limit", reset_at
		FROM scm.rate_limit_cache
		WHERE tenant_id = $1 AND provider = $2 AND bucket = $3 AND last_checked_at > $4
	`, tenantID, string(provider), rateLimitBucket, cutoff)

	var status domain.RateLimitStatus
	status.Provider = provider
	err := row.Scan(&status.Remaining, &status.Limit, &status.ResetAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RateLimitStatus{}, false, nil // miss, not an error — see this method's contract in ports.go
	}
	if err != nil {
		return domain.RateLimitStatus{}, false, fmt.Errorf("postgres: query rate limit cache: %w", err)
	}
	return status, true, nil
}

func (r *RateLimitCacheRepository) Set(ctx context.Context, tenantID string, provider domain.ScmProvider, status domain.RateLimitStatus) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO scm.rate_limit_cache (tenant_id, provider, bucket, remaining, "limit", reset_at, last_checked_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())
		ON CONFLICT (tenant_id, provider, bucket) DO UPDATE SET
			remaining = EXCLUDED.remaining,
			"limit" = EXCLUDED.limit,
			reset_at = EXCLUDED.reset_at,
			last_checked_at = now()
	`, tenantID, string(provider), rateLimitBucket, status.Remaining, status.Limit, status.ResetAt)
	if err != nil {
		return fmt.Errorf("postgres: upsert rate limit cache: %w", err)
	}
	return nil
}
