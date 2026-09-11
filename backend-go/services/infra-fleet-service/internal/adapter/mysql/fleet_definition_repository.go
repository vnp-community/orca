package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// FleetDefinitionStore implements usecase.FleetDefinitionRepository against
// fleet_definitions (migrations/mysql/0018).
type FleetDefinitionStore struct {
	db *sql.DB
}

func NewFleetDefinitionStore(db *sql.DB) *FleetDefinitionStore {
	return &FleetDefinitionStore{db: db}
}

// Create inserts a new fleet definition and returns the persisted row. No
// RETURNING on MySQL — INSERT (letting created_at/updated_at use their
// CURRENT_TIMESTAMP(6) DEFAULT), then SELECT the two timestamps back.
func (s *FleetDefinitionStore) Create(ctx context.Context, def domain.FleetDefinition) (domain.FleetDefinition, error) {
	serversJSON, err := json.Marshal(def.Servers)
	if err != nil {
		return domain.FleetDefinition{}, fmt.Errorf("mysql: marshal fleet definition servers: %w", err)
	}
	var provisionJSON []byte
	if def.Provision != nil {
		provisionJSON, err = json.Marshal(def.Provision)
		if err != nil {
			return domain.FleetDefinition{}, fmt.Errorf("mysql: marshal fleet definition provision: %w", err)
		}
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO fleet_definitions (id, tenant_id, name, version, servers, provision, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, def.ID, def.TenantID, def.Name, def.Version, serversJSON, provisionJSON, def.CreatedBy)
	if err != nil {
		return domain.FleetDefinition{}, fmt.Errorf("mysql: insert fleet definition: %w", err)
	}
	row := s.db.QueryRowContext(ctx, `SELECT created_at, updated_at FROM fleet_definitions WHERE id = ?`, def.ID)
	if err := row.Scan(&def.CreatedAt, &def.UpdatedAt); err != nil {
		return domain.FleetDefinition{}, fmt.Errorf("mysql: reading back created fleet definition: %w", err)
	}
	return def, nil
}

// Update replaces servers/provision and persists def.Version (the NEW
// version, already incremented by the caller) — optimistic locking: the
// WHERE clause matches the OLD version (def.Version-1). No RETURNING on
// MySQL. Unlike the general RowsAffected pitfall (BE-DB-SOL-005 §3.1), the
// `version` column is ALWAYS set to a value distinct from the WHERE's
// oldVersion on every call, so the row can never be a true no-op UPDATE —
// RowsAffected()==0 unambiguously means no row currently has
// id/tenant/oldVersion (a real version conflict or not-found), so it's
// safe to use directly instead of an independent follow-up SELECT (which
// would wrongly match a row some OTHER, earlier Update already advanced to
// this same version — the bug an initial translation attempt had).
func (s *FleetDefinitionStore) Update(ctx context.Context, def domain.FleetDefinition) (domain.FleetDefinition, error) {
	serversJSON, err := json.Marshal(def.Servers)
	if err != nil {
		return domain.FleetDefinition{}, fmt.Errorf("mysql: marshal fleet definition servers: %w", err)
	}
	var provisionJSON []byte
	if def.Provision != nil {
		provisionJSON, err = json.Marshal(def.Provision)
		if err != nil {
			return domain.FleetDefinition{}, fmt.Errorf("mysql: marshal fleet definition provision: %w", err)
		}
	}
	oldVersion := def.Version - 1
	res, err := s.db.ExecContext(ctx, `
		UPDATE fleet_definitions
		SET servers = ?, provision = ?, version = ?, updated_at = CURRENT_TIMESTAMP(6)
		WHERE id = ? AND tenant_id = ? AND version = ?
	`, serversJSON, provisionJSON, def.Version, def.ID, def.TenantID, oldVersion)
	if err != nil {
		return domain.FleetDefinition{}, fmt.Errorf("mysql: update fleet definition: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return domain.FleetDefinition{}, fmt.Errorf("mysql: reading rows affected: %w", err)
	}
	if affected == 0 {
		return domain.FleetDefinition{}, domain.ErrFleetDefinitionVersionConflict
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT updated_at FROM fleet_definitions WHERE id = ? AND tenant_id = ? AND version = ?
	`, def.ID, def.TenantID, def.Version)
	if err := row.Scan(&def.UpdatedAt); err != nil {
		return domain.FleetDefinition{}, fmt.Errorf("mysql: reading back updated fleet definition: %w", err)
	}
	return def, nil
}

// Get fetches a fleet definition scoped to tenantID.
func (s *FleetDefinitionStore) Get(ctx context.Context, tenantID, id string) (domain.FleetDefinition, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, name, version, servers, provision, created_by, created_at, updated_at
		FROM fleet_definitions
		WHERE tenant_id = ? AND id = ?
	`, tenantID, id)
	def, err := scanFleetDefinition(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.FleetDefinition{}, domain.ErrFleetDefinitionNotFound
	}
	if err != nil {
		return domain.FleetDefinition{}, fmt.Errorf("mysql: get fleet definition: %w", err)
	}
	return def, nil
}

// List returns every fleet definition registered for tenantID.
func (s *FleetDefinitionStore) List(ctx context.Context, tenantID string) ([]domain.FleetDefinition, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, name, version, servers, provision, created_by, created_at, updated_at
		FROM fleet_definitions
		WHERE tenant_id = ?
		ORDER BY name
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query fleet definitions: %w", err)
	}
	defer rows.Close()

	var out []domain.FleetDefinition
	for rows.Next() {
		def, err := scanFleetDefinition(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan fleet definition row: %w", err)
		}
		out = append(out, def)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate fleet definition rows: %w", err)
	}
	return out, nil
}

func scanFleetDefinition(row rowScanner) (domain.FleetDefinition, error) {
	var def domain.FleetDefinition
	var serversJSON []byte
	var provisionJSON []byte
	if err := row.Scan(
		&def.ID, &def.TenantID, &def.Name, &def.Version, &serversJSON, &provisionJSON,
		&def.CreatedBy, &def.CreatedAt, &def.UpdatedAt,
	); err != nil {
		return domain.FleetDefinition{}, err
	}
	if err := json.Unmarshal(serversJSON, &def.Servers); err != nil {
		return domain.FleetDefinition{}, fmt.Errorf("decode servers json: %w", err)
	}
	if len(provisionJSON) > 0 {
		var provision domain.ProvisionConfig
		if err := json.Unmarshal(provisionJSON, &provision); err != nil {
			return domain.FleetDefinition{}, fmt.Errorf("decode provision json: %w", err)
		}
		def.Provision = &provision
	}
	return def, nil
}
