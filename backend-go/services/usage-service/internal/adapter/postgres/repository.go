// Package postgres implements usage-service's Repository port (defined in
// internal/usecase) against this service's own PostgreSQL database — see
// specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule: this is the ONLY package in usage-service that
// knows SQL exists.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/usage-service/internal/domain"
)

// Repository implements usecase.Repository against Postgres via pgx —
// hand-written SQL (see architecture/04-tech-stack.md: sqlc codegen is the
// eventual target, this scaffold hand-writes the equivalent queries
// directly to avoid a build-time dependency on the sqlc binary).
type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// SaveSession inserts a session, updates its day's rollup, and enqueues
// event as an outbox row — all in ONE transaction (Epic G's
// transactional-outbox pattern, docs/execution-plan.md; see
// specs/backend-go/architecture/05-data-architecture.md). Idempotent on
// (tenant_id, request_id) via ON CONFLICT DO NOTHING: a retried write with
// the same RequestID is a no-op — no double-counted rollup AND no
// duplicate outbox row, since both happen after the same early-return.
func (r *Repository) SaveSession(ctx context.Context, s domain.UsageSession, event domain.OutboxEvent) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		INSERT INTO usage.sessions (
			id, tenant_id, user_id, provider, worktree_id,
			input_tokens, output_tokens, cache_read_tokens, cache_write_tokens,
			cost_usd, started_at, ended_at, request_id
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (tenant_id, request_id) DO NOTHING
	`,
		s.ID, s.TenantID, s.UserID, string(s.Provider), s.WorktreeID,
		s.InputTokens, s.OutputTokens, s.CacheReadTokens, s.CacheWriteTokens,
		s.CostUSD, nullableTime(s.StartedAt), nullableTime(s.EndedAt), s.RequestID,
	)
	if err != nil {
		return fmt.Errorf("postgres: insert session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Idempotent replay of an already-recorded request_id — nothing
		// more to do, the rollup and outbox row were already written on
		// the first attempt.
		return tx.Commit(ctx)
	}

	day := domain.DayKey(s.StartedAt)
	_, err = tx.Exec(ctx, `
		INSERT INTO usage.daily_rollups (
			tenant_id, user_id, provider, day,
			total_input_tokens, total_output_tokens, total_cost_usd, session_count
		) VALUES ($1,$2,$3,$4,$5,$6,$7,1)
		ON CONFLICT (tenant_id, user_id, provider, day) DO UPDATE SET
			total_input_tokens = usage.daily_rollups.total_input_tokens + EXCLUDED.total_input_tokens,
			total_output_tokens = usage.daily_rollups.total_output_tokens + EXCLUDED.total_output_tokens,
			total_cost_usd = usage.daily_rollups.total_cost_usd + EXCLUDED.total_cost_usd,
			session_count = usage.daily_rollups.session_count + 1
	`,
		s.TenantID, s.UserID, string(s.Provider), day,
		s.InputTokens, s.OutputTokens, s.CostUSD,
	)
	if err != nil {
		return fmt.Errorf("postgres: upsert daily rollup: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO usage.outbox_events (id, tenant_id, subject, occurred_at, version, payload)
		VALUES ($1, $2, $3, $4, 1, $5)
	`, event.ID, s.TenantID, event.Subject, event.OccurredAt, event.PayloadJSON)
	if err != nil {
		return fmt.Errorf("postgres: insert outbox event: %w", err)
	}

	return tx.Commit(ctx)
}

// FetchUnpublished and MarkPublished implement common/outbox.Store — see
// cmd/server/main.go for where the relay is wired. Kept on the same
// Repository as SaveSession since both operate on this service's own
// database; no domain reason to split them into a separate type.
func (r *Repository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, subject, occurred_at, version, payload
		FROM usage.outbox_events
		WHERE published_at IS NULL
		ORDER BY created_at
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: query unpublished outbox events: %w", err)
	}
	defer rows.Close()

	var out []outbox.Record
	for rows.Next() {
		var rec outbox.Record
		if err := rows.Scan(&rec.ID, &rec.Event.TenantID, &rec.Subject, &rec.Event.OccurredAt, &rec.Event.Version, &rec.Event.Payload); err != nil {
			return nil, fmt.Errorf("postgres: scan outbox event row: %w", err)
		}
		rec.Event.ID = rec.ID
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate outbox event rows: %w", err)
	}
	return out, nil
}

func (r *Repository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx, `UPDATE usage.outbox_events SET published_at = now() WHERE id = ANY($1)`, ids)
	if err != nil {
		return fmt.Errorf("postgres: mark outbox events published: %w", err)
	}
	return nil
}

func (r *Repository) GetDailyRollup(ctx context.Context, tenantID, userID string, provider domain.Provider, day time.Time) (domain.DailyUsageRollup, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT total_input_tokens, total_output_tokens, total_cost_usd, session_count
		FROM usage.daily_rollups
		WHERE tenant_id = $1 AND user_id = $2 AND provider = $3 AND day = $4
	`, tenantID, userID, string(provider), day)

	rollup := domain.DailyUsageRollup{TenantID: tenantID, UserID: userID, Provider: provider, Day: day}
	err := row.Scan(&rollup.TotalInputTokens, &rollup.TotalOutputTokens, &rollup.TotalCostUSD, &rollup.SessionCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return rollup, nil // no usage yet today is not an error, just zeros
	}
	if err != nil {
		return domain.DailyUsageRollup{}, fmt.Errorf("postgres: query daily rollup: %w", err)
	}
	return rollup, nil
}

func (r *Repository) ListSessions(ctx context.Context, tenantID, userID, pageToken string, pageSize int32) ([]domain.UsageSession, string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, user_id, provider, worktree_id,
		       input_tokens, output_tokens, cache_read_tokens, cache_write_tokens,
		       cost_usd, started_at, ended_at, request_id
		FROM usage.sessions
		WHERE tenant_id = $1 AND ($2 = '' OR user_id = $2) AND id > $3
		ORDER BY id
		LIMIT $4
	`, tenantID, userID, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: query sessions: %w", err)
	}
	defer rows.Close()

	var out []domain.UsageSession
	for rows.Next() {
		var s domain.UsageSession
		var provider string
		var started, ended time.Time
		if err := rows.Scan(&s.ID, &s.TenantID, &s.UserID, &provider, &s.WorktreeID,
			&s.InputTokens, &s.OutputTokens, &s.CacheReadTokens, &s.CacheWriteTokens,
			&s.CostUSD, &started, &ended, &s.RequestID); err != nil {
			return nil, "", fmt.Errorf("postgres: scan session row: %w", err)
		}
		s.Provider = domain.Provider(provider)
		s.StartedAt = started
		s.EndedAt = ended
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("postgres: iterate session rows: %w", err)
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
	_, err := r.pool.Exec(ctx, `
		INSERT INTO usage.daily_rollups (tenant_id, user_id, provider, day, total_input_tokens, total_output_tokens, total_cost_usd, session_count)
		SELECT tenant_id, user_id, provider, $4::date,
		       COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0),
		       COALESCE(SUM(cost_usd), 0), COUNT(*)
		FROM usage.sessions
		WHERE tenant_id = $1 AND user_id = $2 AND provider = $3 AND started_at::date = $4::date
		GROUP BY tenant_id, user_id, provider
		ON CONFLICT (tenant_id, user_id, provider, day) DO UPDATE SET
			total_input_tokens = EXCLUDED.total_input_tokens,
			total_output_tokens = EXCLUDED.total_output_tokens,
			total_cost_usd = EXCLUDED.total_cost_usd,
			session_count = EXCLUDED.session_count
	`, tenantID, userID, string(provider), day)
	if err != nil {
		return fmt.Errorf("postgres: recompute daily rollup: %w", err)
	}
	return nil
}

func nullableTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
