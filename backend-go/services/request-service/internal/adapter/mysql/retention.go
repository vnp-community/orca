package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type RetentionRepository struct{ *Repository }

func NewRetentionRepository(r *Repository) *RetentionRepository {
	return &RetentionRepository{Repository: r}
}

var _ usecase.RetentionStore = (*RetentionRepository)(nil)

// ListTenants is the one cross-tenant read (every tenant that ever created a Request).
func (r *RetentionRepository) ListTenants(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT tenant_id FROM request_counters ORDER BY tenant_id`)
	if err != nil {
		return nil, fmt.Errorf("mysql: list tenants: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *RetentionRepository) Settings(ctx context.Context) (domain.RetentionSettings, error) {
	s := domain.DefaultRetention
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		err := db.QueryRowContext(ctx, `
			SELECT request_retention_days, ai_trace_retention_days, ledger_retention_days, redact_pii_in_prompts
			FROM tenant_security_settings WHERE tenant_id = ?`, tenantID).Scan(&s.RequestDays, &s.AITraceDays, &s.LedgerDays, &s.RedactPII)
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	})
	return s, err
}

func (r *RetentionRepository) AnonymizeExpired(ctx context.Context, cutoff time.Time, limit int, pseudonym func(string) string, at time.Time) (int, error) {
	n := 0
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.QueryContext(ctx, `
			SELECT q.id FROM requests q
			WHERE q.tenant_id = ? AND q.status IN ('completed','cancelled') AND q.updated_at < ?
			  AND NOT EXISTS (SELECT 1 FROM request_security_flags f
			                  WHERE f.tenant_id = q.tenant_id AND f.request_id = q.id AND f.erased_at IS NOT NULL)
			ORDER BY q.updated_at, q.id
			LIMIT ?
			FOR UPDATE SKIP LOCKED`, tenantID, cutoff.UTC(), limit)
		if err != nil {
			return fmt.Errorf("mysql: claim expired requests: %w", err)
		}
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		_ = rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		// The erase flag is read from another table, so a replica that committed after our snapshot is only
		// visible to a fresh statement; the row locks we now hold keep it from changing again.
		ids, err = r.withoutErased(ctx, db, tenantID, ids)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		if err := r.anonymizeIDs(ctx, db, tenantID, ids, pseudonym, at, ""); err != nil {
			return err
		}
		n = len(ids)
		return nil
	})
	return n, err
}

func (r *RetentionRepository) Anonymize(ctx context.Context, requestID string, pseudonym func(string) string, at time.Time, erasedBy string) (bool, error) {
	changed := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		var one int
		if err := db.QueryRowContext(ctx, `SELECT 1 FROM requests WHERE tenant_id = ? AND id = ? FOR UPDATE`, tenantID, requestID).Scan(&one); err != nil {
			return domain.ErrRequestNotFound(requestID)
		}
		var erased bool
		if err := db.QueryRowContext(ctx, `
			SELECT EXISTS (SELECT 1 FROM request_security_flags WHERE tenant_id = ? AND request_id = ? AND erased_at IS NOT NULL)`,
			tenantID, requestID).Scan(&erased); err != nil {
			return fmt.Errorf("mysql: read erase flag: %w", err)
		}
		if erased {
			return nil
		}
		if err := r.anonymizeIDs(ctx, db, tenantID, []string{requestID}, pseudonym, at, erasedBy); err != nil {
			return err
		}
		changed = true
		return nil
	})
	return changed, err
}

func eraseValue(m domain.EraseMode) string {
	switch m {
	case domain.EraseClearText:
		return `''`
	case domain.EraseMarker:
		return `'[erased]'`
	case domain.EraseClearJSONArray:
		return `CAST('[]' AS JSON)`
	case domain.EraseClearJSONObject:
		return `CAST('{}' AS JSON)`
	}
	return `NULL`
}

func inList(n int) string { return strings.TrimSuffix(strings.Repeat("?,", n), ",") }

// anonymizeIDs clears every ErasableColumns table for the Requests, swaps the reporter for its pseudonym
// and stamps the erase marker, all in the caller's transaction.
func (r *RetentionRepository) anonymizeIDs(ctx context.Context, db dbExecer, tenantID string, ids []string, pseudonym func(string) string, at time.Time, erasedBy string) error {
	byTable := map[string][]domain.ErasableColumn{}
	var order []string
	for _, c := range domain.ErasableColumns {
		if _, ok := byTable[c.Table]; !ok {
			order = append(order, c.Table)
		}
		byTable[c.Table] = append(byTable[c.Table], c)
	}
	args := make([]any, 0, len(ids)+1)
	args = append(args, tenantID)
	for _, id := range ids {
		args = append(args, id)
	}
	for _, table := range order {
		cols := byTable[table]
		sets := make([]string, 0, len(cols))
		for _, c := range cols {
			sets = append(sets, c.Column+" = "+eraseValue(c.Mode))
		}
		q := fmt.Sprintf(`UPDATE %s SET %s WHERE tenant_id = ? AND %s IN (%s)`, table, strings.Join(sets, ", "), cols[0].KeyColumn, inList(len(ids)))
		qargs := args
		if parent := cols[0].Parent; parent != "" {
			q = fmt.Sprintf(`UPDATE %s SET %s WHERE tenant_id = ? AND %s IN (SELECT id FROM %s WHERE tenant_id = ? AND request_id IN (%s))`,
				table, strings.Join(sets, ", "), cols[0].KeyColumn, parent, inList(len(ids)))
			qargs = append([]any{tenantID}, args...)
		}
		if _, err := db.ExecContext(ctx, q, qargs...); err != nil {
			return fmt.Errorf("mysql: erase %s: %w", table, err)
		}
	}
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`SELECT id, reporter_id FROM requests WHERE tenant_id = ? AND id IN (%s)`, inList(len(ids))), args...)
	if err != nil {
		return fmt.Errorf("mysql: read reporters: %w", err)
	}
	reporters := map[string]string{}
	for rows.Next() {
		var id, rep string
		if err := rows.Scan(&id, &rep); err != nil {
			_ = rows.Close()
			return err
		}
		reporters[id] = rep
	}
	_ = rows.Close()
	for id, rep := range reporters {
		if _, err := db.ExecContext(ctx, `UPDATE requests SET reporter_id = ?, version = version + 1 WHERE tenant_id = ? AND id = ?`, pseudonym(rep), tenantID, id); err != nil {
			return fmt.Errorf("mysql: pseudonymize reporter: %w", err)
		}
	}
	var by any
	if erasedBy != "" {
		by = erasedBy
	}
	for _, id := range ids {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO request_security_flags (tenant_id, request_id, erased_at, erased_by)
			VALUES (?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE erased_at = VALUES(erased_at), erased_by = VALUES(erased_by)`, tenantID, id, at.UTC(), by); err != nil {
			return fmt.Errorf("mysql: stamp erase marker: %w", err)
		}
	}
	return nil
}

func (r *RetentionRepository) withoutErased(ctx context.Context, db dbExecer, tenantID string, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return ids, nil
	}
	args := append([]any{tenantID}, anySlice(ids)...)
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`SELECT request_id FROM request_security_flags WHERE tenant_id = ? AND erased_at IS NOT NULL AND request_id IN (%s)`, inList(len(ids))), args...)
	if err != nil {
		return nil, fmt.Errorf("mysql: recheck erase flags: %w", err)
	}
	done := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		done[id] = true
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	kept := ids[:0:0]
	for _, id := range ids {
		if !done[id] {
			kept = append(kept, id)
		}
	}
	return kept, nil
}

func anySlice(ids []string) []any {
	out := make([]any, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}
