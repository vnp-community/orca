package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// Append inserts an audit entry. No Update/Delete method exists on this
// repository — the table is append-only by design (domain.AuditEntry's doc
// comment) and, in production, at the database-permission level too (see
// migrations/0001_init.up.sql's comment on auth.audit_log).
func (r *Repository) Append(ctx context.Context, entry domain.AuditEntry) error {
	var actorID any
	if entry.ActorID != "" {
		actorID = entry.ActorID
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO auth.audit_log (id, tenant_id, actor_id, action, target, occurred_at)
		VALUES ($1,$2,$3,$4,$5,$6)
	`, entry.ID, entry.TenantID, actorID, entry.Action, entry.Target, entry.OccurredAt)
	if err != nil {
		return fmt.Errorf("postgres: insert audit entry: %w", err)
	}
	return nil
}

func (r *Repository) Query(ctx context.Context, tenantID string, since time.Time, pageToken string, pageSize int32) ([]domain.AuditEntry, string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, COALESCE(actor_id::text, ''), action, target, occurred_at
		FROM auth.audit_log
		WHERE tenant_id = $1 AND occurred_at >= $2 AND id::text > $3
		ORDER BY id
		LIMIT $4
	`, tenantID, since, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: query audit log: %w", err)
	}
	defer rows.Close()

	var out []domain.AuditEntry
	for rows.Next() {
		var e domain.AuditEntry
		if err := rows.Scan(&e.ID, &e.TenantID, &e.ActorID, &e.Action, &e.Target, &e.OccurredAt); err != nil {
			return nil, "", fmt.Errorf("postgres: scan audit log row: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("postgres: iterate audit log rows: %w", err)
	}

	next := ""
	if int32(len(out)) == pageSize && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}
