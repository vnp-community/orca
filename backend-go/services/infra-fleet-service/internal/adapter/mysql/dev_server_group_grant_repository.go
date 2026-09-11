package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// DevServerGroupGrantStore implements usecase.DevServerGroupGrantRepository
// against dev_server_group_grants (migrations/mysql/0033).
type DevServerGroupGrantStore struct {
	db *sql.DB
}

func NewDevServerGroupGrantStore(db *sql.DB) *DevServerGroupGrantStore {
	return &DevServerGroupGrantStore{db: db}
}

func (s *DevServerGroupGrantStore) Create(ctx context.Context, grant domain.DevServerGroupGrant) (domain.DevServerGroupGrant, error) {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO dev_server_group_grants (id, tenant_id, dev_server_group_id, grantee_kind, grantee_id)
		VALUES (?, ?, ?, ?, ?)
	`, grant.ID, grant.TenantID, grant.DevServerGroupID, string(grant.GranteeKind), grant.GranteeID)
	if err != nil {
		return domain.DevServerGroupGrant{}, fmt.Errorf("mysql: insert dev server group grant: %w", err)
	}
	return grant, nil
}

func (s *DevServerGroupGrantStore) Delete(ctx context.Context, tenantID, grantID string) error {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM dev_server_group_grants
		WHERE tenant_id = ? AND id = ?
	`, tenantID, grantID)
	if err != nil {
		return fmt.Errorf("mysql: delete dev server group grant: %w", err)
	}
	// DELETE's RowsAffected() counts by WHERE-match on every dialect (no
	// "changed value" ambiguity the way UPDATE has — see
	// BE-DB-SOL-005 §3.1's RowsAffected pitfall, which does not apply here).
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("mysql: grant %q not found for tenant", grantID)
	}
	return nil
}

func (s *DevServerGroupGrantStore) ListByGroup(ctx context.Context, tenantID, groupID string) ([]domain.DevServerGroupGrant, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, dev_server_group_id, grantee_kind, grantee_id
		FROM dev_server_group_grants
		WHERE tenant_id = ? AND dev_server_group_id = ?
		ORDER BY created_at
	`, tenantID, groupID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query dev server group grants: %w", err)
	}
	defer rows.Close()
	return scanGrantRows(rows)
}

func (s *DevServerGroupGrantStore) ListAll(ctx context.Context, tenantID string) ([]domain.DevServerGroupGrant, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, dev_server_group_id, grantee_kind, grantee_id
		FROM dev_server_group_grants
		WHERE tenant_id = ?
		ORDER BY created_at
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query dev server group grants: %w", err)
	}
	defer rows.Close()
	return scanGrantRows(rows)
}

func scanGrantRows(rows *sql.Rows) ([]domain.DevServerGroupGrant, error) {
	var out []domain.DevServerGroupGrant
	for rows.Next() {
		var g domain.DevServerGroupGrant
		var kind string
		if err := rows.Scan(&g.ID, &g.TenantID, &g.DevServerGroupID, &kind, &g.GranteeID); err != nil {
			return nil, fmt.Errorf("mysql: scan dev server group grant row: %w", err)
		}
		g.GranteeKind = domain.GranteeKind(kind)
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate dev server group grant rows: %w", err)
	}
	return out, nil
}
