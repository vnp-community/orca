package mysql

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// Upsert inserts a new sso_group_role_mapping row, or updates role on an
// existing (tenant_id, provider, group_name) row. MySQL's `ON DUPLICATE
// KEY UPDATE` has no RETURNING equivalent (unlike Postgres's `ON CONFLICT
// ... DO UPDATE ... RETURNING`), and — unlike a plain UPDATE — its
// RowsAffected() is 0 for an insert-shaped no-op, 1 for a fresh insert, 2
// for an update that changed a value, and 0 again for an update that
// didn't (MySQL's ON DUPLICATE KEY UPDATE quirk on top of the ordinary
// UPDATE RowsAffected ambiguity) — none of those numbers reliably tell us
// which row/id resulted. So: upsert unconditionally, then re-SELECT by the
// (tenant_id, provider, group_name) unique key to build the return value —
// this also correctly returns the PRE-EXISTING row's id on a conflict
// (m.ID is only used for a brand-new row), matching the Postgres variant's
// RETURNING semantics exactly (EXCLUDED.role only updates role, never id).
func (r *Repository) Upsert(ctx context.Context, m domain.SsoGroupRoleMapping) (domain.SsoGroupRoleMapping, error) {
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO sso_group_role_mapping (id, tenant_id, provider, group_name, role, created_at)
		VALUES (?,?,?,?,?,?)
		ON DUPLICATE KEY UPDATE role = VALUES(role)
	`, m.ID, m.TenantID, string(m.Provider), m.GroupName, string(m.Role), m.CreatedAt); err != nil {
		return domain.SsoGroupRoleMapping{}, fmt.Errorf("mysql: upsert sso group role mapping: %w", err)
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, provider, group_name, role, created_at
		FROM sso_group_role_mapping
		WHERE tenant_id = ? AND provider = ? AND group_name = ?
	`, m.TenantID, string(m.Provider), m.GroupName)

	var out domain.SsoGroupRoleMapping
	var provider, role string
	if err := row.Scan(&out.ID, &out.TenantID, &provider, &out.GroupName, &role, &out.CreatedAt); err != nil {
		return domain.SsoGroupRoleMapping{}, fmt.Errorf("mysql: re-read upserted sso group role mapping: %w", err)
	}
	out.Provider = domain.SsoProvider(provider)
	out.Role = domain.Role(role)
	return out, nil
}

// ListForProvider returns every mapping row for tenantID, optionally
// narrowed to one provider — empty provider means "every provider".
func (r *Repository) ListForProvider(ctx context.Context, tenantID string, provider domain.SsoProvider) ([]domain.SsoGroupRoleMapping, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, tenant_id, provider, group_name, role, created_at
		FROM sso_group_role_mapping
		WHERE tenant_id = ? AND (? = '' OR provider = ?)
		ORDER BY provider, group_name
	`, tenantID, string(provider), string(provider))
	if err != nil {
		return nil, fmt.Errorf("mysql: query sso group role mappings: %w", err)
	}
	defer rows.Close()

	var out []domain.SsoGroupRoleMapping
	for rows.Next() {
		var m domain.SsoGroupRoleMapping
		var providerStr, role string
		if err := rows.Scan(&m.ID, &m.TenantID, &providerStr, &m.GroupName, &role, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("mysql: scan sso group role mapping row: %w", err)
		}
		m.Provider = domain.SsoProvider(providerStr)
		m.Role = domain.Role(role)
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate sso group role mapping rows: %w", err)
	}
	return out, nil
}
