package postgres

import (
	"context"
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

func (r *RetentionRepository) ListTenants(ctx context.Context) ([]string, error) {
	var out []string
	err := r.withRelayTx(ctx, func(ctx context.Context, db dbExecer) error {
		rows, err := db.Query(ctx, `SELECT tenant_id::text FROM request.request_counters ORDER BY tenant_id`)
		if err != nil {
			return fmt.Errorf("postgres: list tenants: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			out = append(out, id)
		}
		return rows.Err()
	})
	return out, err
}

func (r *RetentionRepository) Settings(ctx context.Context) (domain.RetentionSettings, error) {
	s := domain.DefaultRetention
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.Query(ctx, `
			SELECT request_retention_days, ai_trace_retention_days, ledger_retention_days, redact_pii_in_prompts
			FROM request.tenant_security_settings WHERE tenant_id = $1`, tenantID)
		if err != nil {
			return fmt.Errorf("postgres: retention settings: %w", err)
		}
		defer rows.Close()
		if rows.Next() {
			return rows.Scan(&s.RequestDays, &s.AITraceDays, &s.LedgerDays, &s.RedactPII)
		}
		return rows.Err()
	})
	return s, err
}

func (r *RetentionRepository) AnonymizeExpired(ctx context.Context, cutoff time.Time, limit int, pseudonym func(string) string, at time.Time) (int, error) {
	n := 0
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.Query(ctx, `
			SELECT q.id::text FROM request.requests q
			WHERE q.tenant_id = $1 AND q.status IN ('completed','cancelled') AND q.updated_at < $2
			  AND NOT EXISTS (SELECT 1 FROM request.request_security_flags f
			                  WHERE f.tenant_id = q.tenant_id AND f.request_id = q.id AND f.erased_at IS NOT NULL)
			ORDER BY q.updated_at, q.id
			LIMIT $3
			FOR UPDATE OF q SKIP LOCKED`, tenantID, cutoff, limit)
		if err != nil {
			return fmt.Errorf("postgres: claim expired requests: %w", err)
		}
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		rows.Close()
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
		var erased bool
		err := db.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM request.request_security_flags WHERE tenant_id = $1 AND request_id = $2::uuid AND erased_at IS NOT NULL)`,
			tenantID, requestID).Scan(&erased)
		if err != nil {
			return fmt.Errorf("postgres: read erase flag: %w", err)
		}
		if erased {
			return nil
		}
		var one int
		if err := db.QueryRow(ctx, `SELECT 1 FROM request.requests WHERE tenant_id = $1 AND id = $2::uuid FOR UPDATE`, tenantID, requestID).Scan(&one); err != nil {
			return domain.ErrRequestNotFound(requestID)
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
		return `'[]'`
	case domain.EraseClearJSONObject:
		return `'{}'`
	}
	return `NULL`
}

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
	for _, table := range order {
		cols := byTable[table]
		sets := make([]string, 0, len(cols))
		for _, c := range cols {
			sets = append(sets, c.Column+" = "+eraseValue(c.Mode))
		}
		match := cols[0].KeyColumn + ` = ANY($2::uuid[])`
		if parent := cols[0].Parent; parent != "" {
			match = fmt.Sprintf(`%s IN (SELECT id FROM request.%s WHERE tenant_id = $1 AND request_id = ANY($2::uuid[]))`, cols[0].KeyColumn, parent)
		}
		q := fmt.Sprintf(`UPDATE request.%s SET %s WHERE tenant_id = $1 AND %s`, table, strings.Join(sets, ", "), match)
		if _, err := db.Exec(ctx, q, tenantID, ids); err != nil {
			return fmt.Errorf("postgres: erase %s: %w", table, err)
		}
	}
	rows, err := db.Query(ctx, `SELECT id::text, reporter_id::text FROM request.requests WHERE tenant_id = $1 AND id = ANY($2::uuid[])`, tenantID, ids)
	if err != nil {
		return fmt.Errorf("postgres: read reporters: %w", err)
	}
	reporters := map[string]string{}
	for rows.Next() {
		var id, rep string
		if err := rows.Scan(&id, &rep); err != nil {
			rows.Close()
			return err
		}
		reporters[id] = rep
	}
	rows.Close()
	for id, rep := range reporters {
		if _, err := db.Exec(ctx, `UPDATE request.requests SET reporter_id = $3::uuid, version = version + 1 WHERE tenant_id = $1 AND id = $2::uuid`,
			tenantID, id, pseudonym(rep)); err != nil {
			return fmt.Errorf("postgres: pseudonymize reporter: %w", err)
		}
	}
	var by any
	if erasedBy != "" {
		by = erasedBy
	}
	for _, id := range ids {
		if _, err := db.Exec(ctx, `
			INSERT INTO request.request_security_flags (tenant_id, request_id, erased_at, erased_by)
			VALUES ($1, $2::uuid, $3, $4::uuid)
			ON CONFLICT (tenant_id, request_id) DO UPDATE SET erased_at = EXCLUDED.erased_at, erased_by = EXCLUDED.erased_by`,
			tenantID, id, at, by); err != nil {
			return fmt.Errorf("postgres: stamp erase marker: %w", err)
		}
	}
	return nil
}

func (r *RetentionRepository) withoutErased(ctx context.Context, db dbExecer, tenantID string, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return ids, nil
	}
	rows, err := db.Query(ctx, `SELECT request_id::text FROM request.request_security_flags WHERE tenant_id = $1 AND request_id = ANY($2::uuid[]) AND erased_at IS NOT NULL`, tenantID, ids)
	if err != nil {
		return nil, fmt.Errorf("postgres: recheck erase flags: %w", err)
	}
	done := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		done[id] = true
	}
	rows.Close()
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
