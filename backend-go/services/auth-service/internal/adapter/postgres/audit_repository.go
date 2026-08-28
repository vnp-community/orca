package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

// Append inserts an audit entry. No Update/Delete method exists on this
// repository — the table is append-only by design (domain.AuditEntry's doc
// comment) and, in production, at the database-permission level too (see
// migrations/0001_init.up.sql's comment on auth.audit_log).
func (r *Repository) Append(ctx context.Context, entry domain.AuditEntry) error {
	metadataJSON, err := json.Marshal(entry.Metadata)
	if err != nil {
		return fmt.Errorf("postgres: marshal audit metadata: %w", err)
	}
	var actorID, ip any
	if entry.ActorID != "" {
		actorID = entry.ActorID
	}
	if entry.IPAddress != "" {
		ip = entry.IPAddress
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO auth.audit_log (id, tenant_id, actor_id, action, target_type, target_id, metadata, ip_address, occurred_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, entry.ID, entry.TenantID, actorID, entry.Action, entry.TargetType, entry.TargetID, metadataJSON, ip, entry.OccurredAt)
	if err != nil {
		return fmt.Errorf("postgres: insert audit entry: %w", err)
	}
	return nil
}

// Query builds a WHERE clause incrementally — tenant_id + occurred_at >=
// since always present; action/actor_id/to added only when non-empty/
// non-zero.
func (r *Repository) Query(ctx context.Context, filter usecase.AuditQueryFilter, pageToken string, pageSize int32) ([]domain.AuditEntry, string, error) {
	clauses := []string{"tenant_id = $1", "occurred_at >= $2", "id::text > $3"}
	args := []any{filter.TenantID, filter.Since, pageToken}

	if !filter.To.IsZero() {
		args = append(args, filter.To)
		clauses = append(clauses, "occurred_at <= $"+strconv.Itoa(len(args)))
	}
	if filter.Action != "" {
		args = append(args, filter.Action)
		clauses = append(clauses, "action = $"+strconv.Itoa(len(args)))
	}
	if filter.ActorID != "" {
		args = append(args, filter.ActorID)
		clauses = append(clauses, "actor_id = $"+strconv.Itoa(len(args)))
	}
	args = append(args, pageSize)
	limitPos := len(args)

	// host(ip_address), not ip_address::text — casting inet to text keeps
	// the /32 netmask suffix (e.g. "203.0.113.7/32"); host() strips it.
	// Same fix as session_repository.go's ip column scans.
	query := fmt.Sprintf(`
		SELECT id, tenant_id, COALESCE(actor_id::text, ''), action,
		       COALESCE(target_type, ''), COALESCE(target_id, ''), metadata,
		       COALESCE(host(ip_address), ''), occurred_at
		FROM auth.audit_log
		WHERE %s
		ORDER BY id
		LIMIT $%d
	`, strings.Join(clauses, " AND "), limitPos)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: query audit log: %w", err)
	}
	defer rows.Close()

	var out []domain.AuditEntry
	for rows.Next() {
		var e domain.AuditEntry
		var metadataJSON []byte
		if err := rows.Scan(&e.ID, &e.TenantID, &e.ActorID, &e.Action, &e.TargetType, &e.TargetID, &metadataJSON, &e.IPAddress, &e.OccurredAt); err != nil {
			return nil, "", fmt.Errorf("postgres: scan audit log row: %w", err)
		}
		if err := json.Unmarshal(metadataJSON, &e.Metadata); err != nil {
			return nil, "", fmt.Errorf("postgres: unmarshal audit metadata: %w", err)
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
