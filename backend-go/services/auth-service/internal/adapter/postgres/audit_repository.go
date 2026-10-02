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
	return r.insertAudit(ctx, entry, false)
}

// AppendIdempotent is Append that ignores a duplicate id (event redelivery).
func (r *Repository) AppendIdempotent(ctx context.Context, entry domain.AuditEntry) error {
	return r.insertAudit(ctx, entry, true)
}

func (r *Repository) insertAudit(ctx context.Context, entry domain.AuditEntry, ignoreDuplicate bool) error {
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
	outcome := entry.Outcome
	if outcome == "" {
		outcome = domain.OutcomeAllowed // matches domain.NewAuditEntry's own backward-compatible default
	}
	onConflict := ""
	if ignoreDuplicate {
		onConflict = " ON CONFLICT (id) DO NOTHING"
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO auth.audit_log (id, tenant_id, actor_id, action, target, target_type, target_id, metadata, outcome, ip_address, occurred_at, actor_type)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`+onConflict,
		entry.ID, entry.TenantID, actorID, entry.Action, entry.Target, entry.TargetType, entry.TargetID, metadataJSON, string(outcome), ip, entry.OccurredAt, string(entry.EffectiveActorType()))
	if err != nil {
		return fmt.Errorf("postgres: insert audit entry: %w", err)
	}
	return nil
}

// Query builds a WHERE clause incrementally — tenant_id + occurred_at >=
// since always present; to/action/actor_id/outcome added only when
// non-empty/non-zero (TASK-BE-015/016; empty-means-no-filter, matching this
// codebase's established convention — see e.g. ListAnnotations' filePath
// parameter).
func (r *Repository) Query(ctx context.Context, filter usecase.AuditQueryFilter, pageToken string, pageSize int32) ([]domain.AuditEntry, string, error) {
	clauses := []string{"tenant_id = $1", "occurred_at >= $2"}
	args := []any{filter.TenantID, filter.Since}
	if filter.NewestFirst {
		if pageToken != "" {
			at, id, err := domain.DecodeAuditKeyset(pageToken)
			if err != nil {
				return nil, "", err
			}
			args = append(args, at, id)
			clauses = append(clauses, fmt.Sprintf("(occurred_at, id::text) < ($%d, $%d)", len(args)-1, len(args)))
		}
	} else {
		args = append(args, pageToken)
		clauses = append(clauses, "id::text > $"+strconv.Itoa(len(args)))
	}
	if filter.ActorType != "" {
		args = append(args, string(filter.ActorType))
		clauses = append(clauses, "actor_type = $"+strconv.Itoa(len(args)))
	}
	if filter.TargetID != "" {
		args = append(args, filter.TargetID)
		clauses = append(clauses, "target_id = $"+strconv.Itoa(len(args)))
	}
	// Keys come from a fixed allow-list (usecase.AllowedAuditMetadataKeys);
	// even so, only the value is a parameter and the key must pass the check
	// again here before it is placed in the SQL text.
	for _, k := range domain.SortedKeys(filter.MetadataEquals) {
		if !usecase.AllowedAuditMetadataKeys[k] {
			return nil, "", fmt.Errorf("postgres: audit metadata key %q is not filterable", k)
		}
		args = append(args, filter.MetadataEquals[k])
		clauses = append(clauses, "metadata->>'"+k+"' = $"+strconv.Itoa(len(args)))
	}

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
	if filter.Outcome != "" {
		args = append(args, string(filter.Outcome))
		clauses = append(clauses, "outcome = $"+strconv.Itoa(len(args)))
	}
	args = append(args, pageSize)
	limitPos := len(args)

	// host(ip_address), not ip_address::text — casting inet to text keeps
	// the /32 netmask suffix (e.g. "203.0.113.7/32"); host() strips it.
	// Same fix as session_repository.go's ip column scans.
	query := fmt.Sprintf(`
		SELECT id, tenant_id, COALESCE(actor_id::text, ''), action, target,
		       COALESCE(target_type, ''), COALESCE(target_id, ''), metadata,
		       outcome, COALESCE(host(ip_address), ''), occurred_at, actor_type
		FROM auth.audit_log
		WHERE %s
		ORDER BY %s
		LIMIT $%d
	`, strings.Join(clauses, " AND "), domain.AuditOrderBy(filter.NewestFirst), limitPos)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: query audit log: %w", err)
	}
	defer rows.Close()

	var out []domain.AuditEntry
	for rows.Next() {
		var e domain.AuditEntry
		var metadataJSON []byte
		var outcome, actorType string
		if err := rows.Scan(&e.ID, &e.TenantID, &e.ActorID, &e.Action, &e.Target,
			&e.TargetType, &e.TargetID, &metadataJSON, &outcome, &e.IPAddress, &e.OccurredAt, &actorType); err != nil {
			return nil, "", fmt.Errorf("postgres: scan audit log row: %w", err)
		}
		if err := json.Unmarshal(metadataJSON, &e.Metadata); err != nil {
			return nil, "", fmt.Errorf("postgres: unmarshal audit metadata: %w", err)
		}
		e.Outcome = domain.Outcome(outcome)
		e.ActorType = domain.ActorType(actorType)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("postgres: iterate audit log rows: %w", err)
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
