package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// DevServerGroupGrantStore implements usecase.DevServerGroupGrantRepository
// against infra.dev_server_group_grants
// (migrations/0009_dev_server_group_grants_and_access_requests) — see
// docs/crs/v2/dev-server/CR-DS-007-department-based-access-control.md.
type DevServerGroupGrantStore struct {
	pool *pgxpool.Pool
}

func NewDevServerGroupGrantStore(pool *pgxpool.Pool) *DevServerGroupGrantStore {
	return &DevServerGroupGrantStore{pool: pool}
}

func (s *DevServerGroupGrantStore) Create(ctx context.Context, grant domain.DevServerGroupGrant) (domain.DevServerGroupGrant, error) {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO infra.dev_server_group_grants (id, tenant_id, dev_server_group_id, grantee_kind, grantee_id)
		VALUES ($1, $2, $3, $4, $5)
	`, grant.ID, grant.TenantID, grant.DevServerGroupID, string(grant.GranteeKind), grant.GranteeID)
	if err != nil {
		return domain.DevServerGroupGrant{}, fmt.Errorf("postgres: insert dev server group grant: %w", err)
	}
	return grant, nil
}

func (s *DevServerGroupGrantStore) Delete(ctx context.Context, tenantID, grantID string) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM infra.dev_server_group_grants
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, grantID)
	if err != nil {
		return fmt.Errorf("postgres: delete dev server group grant: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: grant %q not found for tenant", grantID)
	}
	return nil
}

func (s *DevServerGroupGrantStore) ListByGroup(ctx context.Context, tenantID, groupID string) ([]domain.DevServerGroupGrant, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, dev_server_group_id, grantee_kind, grantee_id
		FROM infra.dev_server_group_grants
		WHERE tenant_id = $1 AND dev_server_group_id = $2
		ORDER BY created_at
	`, tenantID, groupID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query dev server group grants: %w", err)
	}
	defer rows.Close()
	return scanGrantRows(rows)
}

func (s *DevServerGroupGrantStore) ListAll(ctx context.Context, tenantID string) ([]domain.DevServerGroupGrant, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, dev_server_group_id, grantee_kind, grantee_id
		FROM infra.dev_server_group_grants
		WHERE tenant_id = $1
		ORDER BY created_at
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query dev server group grants: %w", err)
	}
	defer rows.Close()
	return scanGrantRows(rows)
}

func scanGrantRows(rows pgx.Rows) ([]domain.DevServerGroupGrant, error) {
	var out []domain.DevServerGroupGrant
	for rows.Next() {
		var g domain.DevServerGroupGrant
		var kind string
		if err := rows.Scan(&g.ID, &g.TenantID, &g.DevServerGroupID, &kind, &g.GranteeID); err != nil {
			return nil, fmt.Errorf("postgres: scan dev server group grant row: %w", err)
		}
		g.GranteeKind = domain.GranteeKind(kind)
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate dev server group grant rows: %w", err)
	}
	return out, nil
}
