// Package mysql implements usage-service's Repository port (defined in
// internal/usecase) against MySQL/TiDB via database/sql +
// github.com/go-sql-driver/mysql — the multi-dialect pilot adapter for
// CR-DB-003, mirroring internal/adapter/postgres's behavior (idempotency,
// tenant scoping, transactional outbox) 1:1 against the dialect-safe
// schema created by TASK-BE-DB-004
// (migrations/mysql/{0001_init,0002_outbox}.up.sql — tables `sessions`,
// `daily_rollups`, `outbox_events`, no schema/database prefix). See
// specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-002.md.
package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/usage-service/internal/domain"
)

// Repository implements usecase.Repository against MySQL/TiDB via
// database/sql. No RLS equivalent exists in MySQL — every query below
// filters by tenant_id explicitly, which is the ONLY tenant-isolation
// enforcement for this adapter (see TASK-BE-DB-003's finding: this was
// already true for the Postgres adapter too, since RLS never actually
// activated there — this doesn't lower the bar, it just doesn't add a
// backstop that was never real).
type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// SaveSession mirrors internal/adapter/postgres.Repository.SaveSession's
// transaction shape 1:1: insert the session, upsert its day's rollup, and
// enqueue the outbox event, all in ONE transaction, idempotent on
// (tenant_id, request_id) via INSERT IGNORE + the `uniq_tenant_request`
// UNIQUE KEY (MySQL's equivalent of Postgres's ON CONFLICT DO NOTHING).
func (r *Repository) SaveSession(ctx context.Context, s domain.UsageSession, event domain.OutboxEvent) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `
		INSERT IGNORE INTO sessions (
			id, tenant_id, user_id, provider, worktree_id,
			input_tokens, output_tokens, cache_read_tokens, cache_write_tokens,
			cost_usd, started_at, ended_at, request_id
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
	`,
		s.ID, s.TenantID, s.UserID, string(s.Provider), s.WorktreeID,
		s.InputTokens, s.OutputTokens, s.CacheReadTokens, s.CacheWriteTokens,
		s.CostUSD, nullableTime(s.StartedAt), nullableTime(s.EndedAt), s.RequestID,
	)
	if err != nil {
		return fmt.Errorf("mysql: insert session: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: insert session rows affected: %w", err)
	}
	if affected == 0 {
		// Idempotent replay of an already-recorded request_id — nothing
		// more to do, the rollup and outbox row were already written on
		// the first attempt.
		return tx.Commit()
	}

	day := domain.DayKey(s.StartedAt)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO daily_rollups (
			tenant_id, user_id, provider, day,
			total_input_tokens, total_output_tokens, total_cost_usd, session_count
		) VALUES (?,?,?,?,?,?,?,1)
		ON DUPLICATE KEY UPDATE
			total_input_tokens = total_input_tokens + VALUES(total_input_tokens),
			total_output_tokens = total_output_tokens + VALUES(total_output_tokens),
			total_cost_usd = total_cost_usd + VALUES(total_cost_usd),
			session_count = session_count + 1
	`,
		s.TenantID, s.UserID, string(s.Provider), day.Format("2006-01-02"),
		s.InputTokens, s.OutputTokens, s.CostUSD,
	)
	if err != nil {
		return fmt.Errorf("mysql: upsert daily rollup: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO outbox_events (id, tenant_id, subject, occurred_at, version, payload)
		VALUES (?, ?, ?, ?, 1, ?)
	`, event.ID, s.TenantID, event.Subject, event.OccurredAt, event.PayloadJSON)
	if err != nil {
		return fmt.Errorf("mysql: insert outbox event: %w", err)
	}

	return tx.Commit()
}

// FetchUnpublished and MarkPublished implement common/outbox.Store — see
// internal/adapter/postgres.Repository's doc comment for why this stays on
// the same Repository as SaveSession rather than a separate type.
func (r *Repository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, tenant_id, subject, occurred_at, version, payload
		FROM outbox_events
		WHERE published_at IS NULL
		ORDER BY created_at
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("mysql: query unpublished outbox events: %w", err)
	}
	defer rows.Close()

	var out []outbox.Record
	for rows.Next() {
		var rec outbox.Record
		if err := rows.Scan(&rec.ID, &rec.Event.TenantID, &rec.Subject, &rec.Event.OccurredAt, &rec.Event.Version, &rec.Event.Payload); err != nil {
			return nil, fmt.Errorf("mysql: scan outbox event row: %w", err)
		}
		rec.Event.ID = rec.ID
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate outbox event rows: %w", err)
	}
	return out, nil
}

func (r *Repository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	query := fmt.Sprintf(`UPDATE outbox_events SET published_at = NOW(6) WHERE id IN (%s)`, placeholders)
	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("mysql: mark outbox events published: %w", err)
	}
	return nil
}

func (r *Repository) GetDailyRollup(ctx context.Context, tenantID, userID string, provider domain.Provider, day time.Time) (domain.DailyUsageRollup, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT total_input_tokens, total_output_tokens, total_cost_usd, session_count
		FROM daily_rollups
		WHERE tenant_id = ? AND user_id = ? AND provider = ? AND day = ?
	`, tenantID, userID, string(provider), domain.DayKey(day).Format("2006-01-02"))

	rollup := domain.DailyUsageRollup{TenantID: tenantID, UserID: userID, Provider: provider, Day: day}
	err := row.Scan(&rollup.TotalInputTokens, &rollup.TotalOutputTokens, &rollup.TotalCostUSD, &rollup.SessionCount)
	if errors.Is(err, sql.ErrNoRows) {
		return rollup, nil // no usage yet today is not an error, just zeros
	}
	if err != nil {
		return domain.DailyUsageRollup{}, fmt.Errorf("mysql: query daily rollup: %w", err)
	}
	return rollup, nil
}

func (r *Repository) ListSessions(ctx context.Context, tenantID, userID, pageToken string, pageSize int32) ([]domain.UsageSession, string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, tenant_id, user_id, provider, worktree_id,
		       input_tokens, output_tokens, cache_read_tokens, cache_write_tokens,
		       cost_usd, started_at, ended_at, request_id
		FROM sessions
		WHERE tenant_id = ? AND (? = '' OR user_id = ?) AND id > ?
		ORDER BY id
		LIMIT ?
	`, tenantID, userID, userID, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("mysql: query sessions: %w", err)
	}
	defer rows.Close()

	var out []domain.UsageSession
	for rows.Next() {
		var s domain.UsageSession
		var provider string
		var started, ended sql.NullTime
		if err := rows.Scan(&s.ID, &s.TenantID, &s.UserID, &provider, &s.WorktreeID,
			&s.InputTokens, &s.OutputTokens, &s.CacheReadTokens, &s.CacheWriteTokens,
			&s.CostUSD, &started, &ended, &s.RequestID); err != nil {
			return nil, "", fmt.Errorf("mysql: scan session row: %w", err)
		}
		s.Provider = domain.Provider(provider)
		s.StartedAt = started.Time
		s.EndedAt = ended.Time
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("mysql: iterate session rows: %w", err)
	}

	next := ""
	if int32(len(out)) == pageSize && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

// RecomputeDailyRollup rebuilds a day's rollup from its sessions — the
// reconciliation safety net, run periodically rather than on the hot path
// (see usage-service.md §6).
func (r *Repository) RecomputeDailyRollup(ctx context.Context, tenantID, userID string, provider domain.Provider, day time.Time) error {
	dayStr := domain.DayKey(day).Format("2006-01-02")
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO daily_rollups (tenant_id, user_id, provider, day, total_input_tokens, total_output_tokens, total_cost_usd, session_count)
		SELECT tenant_id, user_id, provider, ?,
		       COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0),
		       COALESCE(SUM(cost_usd), 0), COUNT(*)
		FROM sessions
		WHERE tenant_id = ? AND user_id = ? AND provider = ? AND DATE(started_at) = ?
		GROUP BY tenant_id, user_id, provider
		ON DUPLICATE KEY UPDATE
			total_input_tokens = VALUES(total_input_tokens),
			total_output_tokens = VALUES(total_output_tokens),
			total_cost_usd = VALUES(total_cost_usd),
			session_count = VALUES(session_count)
	`, dayStr, tenantID, userID, string(provider), dayStr)
	if err != nil {
		return fmt.Errorf("mysql: recompute daily rollup: %w", err)
	}
	return nil
}

func nullableTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
