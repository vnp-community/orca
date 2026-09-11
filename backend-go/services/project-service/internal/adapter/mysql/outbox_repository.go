package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/common/outbox"
)

// OutboxRepository implements common/outbox.Store against `outbox_events` —
// mirrors postgres.OutboxRepository. As in the Postgres package, the write
// side (the INSERT sharing a transaction with the worktrees write) lives in
// WorktreeRepository.CreateWorktreeWithEvent/RemoveWorktreeWithEvent, not
// here — see that file's insertOutboxEvent helper.
type OutboxRepository struct {
	db *sql.DB
}

func NewOutboxRepository(db *sql.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

func (r *OutboxRepository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	return fetchUnpublishedOutbox(ctx, r.db, limit)
}

func (r *OutboxRepository) MarkPublished(ctx context.Context, ids []string) error {
	return markOutboxPublished(ctx, r.db, ids)
}

// fetchUnpublishedOutbox/markOutboxPublished are shared by OutboxRepository
// and WorktreeRepository (both query the same `outbox_events` table, same
// duplication as the Postgres package's OutboxRepository/WorktreeRepository
// pair — see postgres/outbox_repository.go's doc comment for why enqueue
// isn't unified with these reads).
func fetchUnpublishedOutbox(ctx context.Context, db *sql.DB, limit int) ([]outbox.Record, error) {
	rows, err := db.QueryContext(ctx, `
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
		var payload []byte
		if err := rows.Scan(&rec.ID, &rec.Event.TenantID, &rec.Subject, &rec.Event.OccurredAt, &rec.Event.Version, &payload); err != nil {
			return nil, fmt.Errorf("mysql: scan outbox event row: %w", err)
		}
		rec.Event.Payload = payload
		rec.Event.ID = rec.ID
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate outbox event rows: %w", err)
	}
	return out, nil
}

// markOutboxPublished builds a dynamic `IN (?,...)` — MySQL/database/sql
// has no array bind parameter equivalent to Postgres's `id = ANY($1)`.
func markOutboxPublished(ctx context.Context, db *sql.DB, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := `UPDATE outbox_events SET published_at = NOW(6) WHERE id IN (` + strings.Join(placeholders, ",") + `)`
	if _, err := db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("mysql: mark outbox events published: %w", err)
	}
	return nil
}
