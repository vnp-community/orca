package mysql

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

// Append inserts an audit entry. No Update/Delete method exists on this
// repository — the table is append-only by design, matching
// postgres/audit_repository.go's identical comment.
func (r *Repository) Append(ctx context.Context, entry domain.AuditEntry) error {
	return r.insertAudit(ctx, entry, false)
}

// AppendIdempotent is Append that ignores a duplicate id (event redelivery).
func (r *Repository) AppendIdempotent(ctx context.Context, entry domain.AuditEntry) error {
	return r.insertAudit(ctx, entry, true)
}

func (r *Repository) insertAudit(ctx context.Context, entry domain.AuditEntry, ignoreDuplicate bool) error {
	metadataJSON, err := json.Marshal(entry.Metadata)
	if err != nil {
		return fmt.Errorf("mysql: marshal audit metadata: %w", err)
	}
	var actorID, ip any
	if entry.ActorID != "" {
		actorID = entry.ActorID
	}
	if entry.IPAddress != "" {
		ip = entry.IPAddress
	}
	outcome := entry.Outcome
	if outcome == "" {
		outcome = domain.OutcomeAllowed // matches domain.NewAuditEntry's own backward-compatible default
	}
	onConflict := ""
	if ignoreDuplicate {
		onConflict = " ON DUPLICATE KEY UPDATE id = id"
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO audit_log (id, tenant_id, actor_id, action, target, target_type, target_id, metadata, outcome, ip_address, occurred_at, actor_type)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`+onConflict,
		entry.ID, entry.TenantID, actorID, entry.Action, entry.Target, entry.TargetType, entry.TargetID, metadataJSON, string(outcome), ip, entry.OccurredAt, string(entry.EffectiveActorType()))
	if err != nil {
		return fmt.Errorf("mysql: insert audit entry: %w", err)
	}
	return nil
}

// Query builds a WHERE clause incrementally, same shape as
// postgres/audit_repository.go — MySQL's `?` placeholders are positional
// by occurrence order (not numbered like Postgres's $N), so the clause
// list and the args slice below are built in lockstep instead of tracking
// an explicit placeholder index.
func (r *Repository) Query(ctx context.Context, filter usecase.AuditQueryFilter, pageToken string, pageSize int32) ([]domain.AuditEntry, string, error) {
	clauses := []string{"tenant_id = ?", "occurred_at >= ?"}
	args := []any{filter.TenantID, filter.Since}
	if filter.NewestFirst {
		if pageToken != "" {
			at, id, err := domain.DecodeAuditKeyset(pageToken)
			if err != nil {
				return nil, "", err
			}
			clauses = append(clauses, "(occurred_at, id) < (?, ?)")
			args = append(args, at, id)
		}
	} else {
		clauses = append(clauses, "id > ?")
		args = append(args, pageToken)
	}
	if filter.ActorType != "" {
		clauses = append(clauses, "actor_type = ?")
		args = append(args, string(filter.ActorType))
	}
	if filter.TargetID != "" {
		clauses = append(clauses, "target_id = ?")
		args = append(args, filter.TargetID)
	}
	// Keys come from a fixed allow-list; only the value is a parameter.
	for _, k := range domain.SortedKeys(filter.MetadataEquals) {
		if !usecase.AllowedAuditMetadataKeys[k] {
			return nil, "", fmt.Errorf("mysql: audit metadata key %q is not filterable", k)
		}
		clauses = append(clauses, "JSON_UNQUOTE(JSON_EXTRACT(metadata, '$."+k+"')) = ?")
		args = append(args, filter.MetadataEquals[k])
	}

	if !filter.To.IsZero() {
		clauses = append(clauses, "occurred_at <= ?")
		args = append(args, filter.To)
	}
	if filter.Action != "" {
		clauses = append(clauses, "action = ?")
		args = append(args, filter.Action)
	}
	if filter.ActorID != "" {
		clauses = append(clauses, "actor_id = ?")
		args = append(args, filter.ActorID)
	}
	if filter.Outcome != "" {
		clauses = append(clauses, "outcome = ?")
		args = append(args, string(filter.Outcome))
	}
	args = append(args, pageSize)

	// ip_address is a plain VARCHAR here (see migrations/mysql/0004's
	// comment) — read directly, no host()-style unwrap needed.
	query := fmt.Sprintf(`
		SELECT id, tenant_id, COALESCE(actor_id, ''), action, target,
		       COALESCE(target_type, ''), COALESCE(target_id, ''), metadata,
		       outcome, COALESCE(ip_address, ''), occurred_at, actor_type
		FROM audit_log
		WHERE %s
		ORDER BY %s
		LIMIT ?
	`, strings.Join(clauses, " AND "), domain.AuditOrderBy(filter.NewestFirst))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("mysql: query audit log: %w", err)
	}
	defer rows.Close()

	var out []domain.AuditEntry
	for rows.Next() {
		var e domain.AuditEntry
		var metadataJSON []byte
		var outcome, actorType string
		if err := rows.Scan(&e.ID, &e.TenantID, &e.ActorID, &e.Action, &e.Target,
			&e.TargetType, &e.TargetID, &metadataJSON, &outcome, &e.IPAddress, &e.OccurredAt, &actorType); err != nil {
			return nil, "", fmt.Errorf("mysql: scan audit log row: %w", err)
		}
		if err := json.Unmarshal(metadataJSON, &e.Metadata); err != nil {
			return nil, "", fmt.Errorf("mysql: unmarshal audit metadata: %w", err)
		}
		e.Outcome = domain.Outcome(outcome)
		e.ActorType = domain.ActorType(actorType)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("mysql: iterate audit log rows: %w", err)
	}

	next := ""
	if int32(len(out)) == pageSize && len(out) > 0 {
		next = out[len(out)-1].ID
		if filter.NewestFirst {
			next = domain.EncodeAuditKeyset(out[len(out)-1])
		}
	}
	return out, next, nil
}
