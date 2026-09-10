package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// DevServerGroupStore implements usecase.DevServerGroupRepository against
// infra.dev_server_groups (migrations/0007_dev_server_status_and_groups) —
// split into its own type for the same reason SshTargetStore is: Go's
// method sets allow only one method per name per receiver, and this store's
// List(ctx, tenantID) would collide with Repository.List's DevServer
// signature otherwise. See docs/crs/v2/dev-server/
// CR-DS-006-dev-server-approval-and-grouping.md §3.2.
type DevServerGroupStore struct {
	pool *pgxpool.Pool
}

// NewDevServerGroupStore builds a DevServerGroupStore over the same pool
// Repository/SshTargetStore use.
func NewDevServerGroupStore(pool *pgxpool.Pool) *DevServerGroupStore {
	return &DevServerGroupStore{pool: pool}
}

// Create inserts a new dev-server group and returns the persisted row.
// parent_group_id is stored as NULL when empty (root of the tree) — same
// NULLIF pattern Repository.Register uses for ssh_target_id.
func (s *DevServerGroupStore) Create(ctx context.Context, group domain.DevServerGroup) (domain.DevServerGroup, error) {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO infra.dev_server_groups (id, tenant_id, name, parent_group_id)
		VALUES ($1, $2, $3, NULLIF($4, '')::uuid)
	`, group.ID, group.TenantID, group.Name, group.ParentGroupID)
	if err != nil {
		return domain.DevServerGroup{}, fmt.Errorf("postgres: insert dev server group: %w", err)
	}
	return group, nil
}

// List returns every dev-server group registered for tenantID.
func (s *DevServerGroupStore) List(ctx context.Context, tenantID string) ([]domain.DevServerGroup, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, name, parent_group_id
		FROM infra.dev_server_groups
		WHERE tenant_id = $1
		ORDER BY name
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query dev server groups: %w", err)
	}
	defer rows.Close()

	var out []domain.DevServerGroup
	for rows.Next() {
		var g domain.DevServerGroup
		var parentGroupID *string
		if err := rows.Scan(&g.ID, &g.TenantID, &g.Name, &parentGroupID); err != nil {
			return nil, fmt.Errorf("postgres: scan dev server group row: %w", err)
		}
		if parentGroupID != nil {
			g.ParentGroupID = *parentGroupID
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate dev server group rows: %w", err)
	}
	return out, nil
}
