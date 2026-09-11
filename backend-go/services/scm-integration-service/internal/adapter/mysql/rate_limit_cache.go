// Package mysql implements scm-integration-service's database ports
// (defined in internal/usecase) against MySQL/TiDB via database/sql +
// github.com/go-sql-driver/mysql — mirroring internal/adapter/postgres's
// behavior 1:1 against the dialect-safe schema in migrations/mysql (tables
// `rate_limit_cache`, `webhook_delivery_log`, `issue_list_cache`,
// `outbox_events`, no schema/database prefix). See
// specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-007.md,
// following the pattern set by BE-DB-SOL-002 (usage-service, the pilot).
//
// Like internal/adapter/postgres, this is one package with one file per
// table/port — RateLimitCacheRepository, IssueListCacheRepository,
// OutboxRepository, WebhookDeliveryRepository — rather than a single
// combined Repository type, matching the Postgres adapter's own shape so
// cmd/server/main.go's dialect switch can construct each one symmetrically.
package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

// rateLimitBucket mirrors internal/adapter/postgres.rateLimitBucket — see
// that file's doc comment.
const rateLimitBucket = "core"

// RateLimitCacheRepository implements usecase.RateLimitCache against MySQL.
// No RLS equivalent exists in MySQL — every query below filters by
// tenant_id explicitly, which is the ONLY tenant-isolation enforcement for
// this adapter (see TASK-BE-DB-003's finding: this was already true for
// the Postgres adapter too, since no Go code anywhere calls
// SET LOCAL app.tenant_id — this doesn't lower the bar, it just doesn't add
// a backstop that was never real).
type RateLimitCacheRepository struct {
	db *sql.DB
}

func New(db *sql.DB) *RateLimitCacheRepository {
	return &RateLimitCacheRepository{db: db}
}

var _ usecase.RateLimitCache = (*RateLimitCacheRepository)(nil)

func (r *RateLimitCacheRepository) Get(ctx context.Context, tenantID string, provider domain.ScmProvider, freshWithin time.Duration) (domain.RateLimitStatus, bool, error) {
	// Computed in Go, same reason as the Postgres adapter: a Go
	// time.Duration's string form isn't a valid interval literal in either
	// dialect, so this sidesteps the mismatch identically on both.
	cutoff := time.Now().Add(-freshWithin)
	row := r.db.QueryRowContext(ctx, "SELECT remaining, `limit`, reset_at FROM rate_limit_cache WHERE tenant_id = ? AND provider = ? AND bucket = ? AND last_checked_at > ?",
		tenantID, string(provider), rateLimitBucket, cutoff)

	var status domain.RateLimitStatus
	status.Provider = provider
	err := row.Scan(&status.Remaining, &status.Limit, &status.ResetAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.RateLimitStatus{}, false, nil // miss, not an error — see this method's contract in ports.go
	}
	if err != nil {
		return domain.RateLimitStatus{}, false, fmt.Errorf("mysql: query rate limit cache: %w", err)
	}
	return status, true, nil
}

func (r *RateLimitCacheRepository) Set(ctx context.Context, tenantID string, provider domain.ScmProvider, status domain.RateLimitStatus) error {
	_, err := r.db.ExecContext(ctx, "INSERT INTO rate_limit_cache (tenant_id, provider, bucket, remaining, `limit`, reset_at, last_checked_at) VALUES (?, ?, ?, ?, ?, ?, NOW(6)) "+
		"ON DUPLICATE KEY UPDATE remaining = VALUES(remaining), `limit` = VALUES(`limit`), reset_at = VALUES(reset_at), last_checked_at = NOW(6)",
		tenantID, string(provider), rateLimitBucket, status.Remaining, status.Limit, status.ResetAt)
	if err != nil {
		return fmt.Errorf("mysql: upsert rate limit cache: %w", err)
	}
	return nil
}
