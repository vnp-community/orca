// Package mysql implements infra-fleet-service's repository ports against
// MySQL/TiDB (CR-DB-002/CR-DB-003), mirroring internal/adapter/postgres 1:1
// per specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-002.md's
// established pattern — see
// specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-017-infra-fleet-service-mysql-tidb-adapter.md
// for this service's own translation notes (RLS drop, TEXT[]->JSON,
// partial-index workarounds, pty_id/rows reserved-word fixes).
package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// DevServerGroupStore implements usecase.DevServerGroupRepository against
// dev_server_groups (migrations/mysql/0030) — split into its own type for
// the same reason internal/adapter/postgres.DevServerGroupStore is: Go's
// method sets allow only one method per name per receiver.
type DevServerGroupStore struct {
	db *sql.DB
}

// NewDevServerGroupStore builds a DevServerGroupStore over the same *sql.DB
// every other MySQL store in this package uses.
func NewDevServerGroupStore(db *sql.DB) *DevServerGroupStore {
	return &DevServerGroupStore{db: db}
}

// Create inserts a new dev-server group and returns the persisted row.
// parent_group_id is stored as NULL when empty (root of the tree) — same
// NULLIF pattern the Postgres variant uses, no ::uuid cast needed since
// CHAR(36) already accepts a plain string.
func (s *DevServerGroupStore) Create(ctx context.Context, group domain.DevServerGroup) (domain.DevServerGroup, error) {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO dev_server_groups (id, tenant_id, name, parent_group_id)
		VALUES (?, ?, ?, NULLIF(?, ''))
	`, group.ID, group.TenantID, group.Name, group.ParentGroupID)
	if err != nil {
		return domain.DevServerGroup{}, fmt.Errorf("mysql: insert dev server group: %w", err)
	}
	return group, nil
}

// List returns every dev-server group registered for tenantID.
func (s *DevServerGroupStore) List(ctx context.Context, tenantID string) ([]domain.DevServerGroup, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, name, parent_group_id
		FROM dev_server_groups
		WHERE tenant_id = ?
		ORDER BY name
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query dev server groups: %w", err)
	}
	defer rows.Close()

	var out []domain.DevServerGroup
	for rows.Next() {
		var g domain.DevServerGroup
		var parentGroupID sql.NullString
		if err := rows.Scan(&g.ID, &g.TenantID, &g.Name, &parentGroupID); err != nil {
			return nil, fmt.Errorf("mysql: scan dev server group row: %w", err)
		}
		if parentGroupID.Valid {
			g.ParentGroupID = parentGroupID.String
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate dev server group rows: %w", err)
	}
	return out, nil
}
