package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// FleetDefinitionStore implements usecase.FleetDefinitionRepository against
// infra.fleet_definitions (migration 0018, TASK-BE-FLEET-011) — own type
// rather than a method on Repository, mirroring SshTargetStore's precedent
// (this package's convention for a table that isn't Repository's original
// scope).
type FleetDefinitionStore struct {
	pool *pgxpool.Pool
}

func NewFleetDefinitionStore(pool *pgxpool.Pool) *FleetDefinitionStore {
	return &FleetDefinitionStore{pool: pool}
}

// Create inserts a new fleet definition and returns the persisted row.
func (s *FleetDefinitionStore) Create(ctx context.Context, def domain.FleetDefinition) (domain.FleetDefinition, error) {
	serversJSON, err := json.Marshal(def.Servers)
	if err != nil {
		return domain.FleetDefinition{}, fmt.Errorf("postgres: marshal fleet definition servers: %w", err)
	}
	var provisionJSON []byte
	if def.Provision != nil {
		provisionJSON, err = json.Marshal(def.Provision)
		if err != nil {
			return domain.FleetDefinition{}, fmt.Errorf("postgres: marshal fleet definition provision: %w", err)
		}
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO infra.fleet_definitions (id, tenant_id, name, version, servers, provision, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at
	`, def.ID, def.TenantID, def.Name, def.Version, serversJSON, provisionJSON, def.CreatedBy)
	if err := row.Scan(&def.CreatedAt, &def.UpdatedAt); err != nil {
		return domain.FleetDefinition{}, fmt.Errorf("postgres: insert fleet definition: %w", err)
	}
	return def, nil
}

// Update replaces servers/provision and persists def.Version (the NEW
// version, already incremented by the caller — usecase.UpdateFleetDefinition)
// — optimistic locking: the WHERE clause matches the OLD version
// (def.Version-1), so a concurrent writer's stale Update affects 0 rows,
// surfaced as domain.ErrFleetDefinitionVersionConflict.
func (s *FleetDefinitionStore) Update(ctx context.Context, def domain.FleetDefinition) (domain.FleetDefinition, error) {
	serversJSON, err := json.Marshal(def.Servers)
	if err != nil {
		return domain.FleetDefinition{}, fmt.Errorf("postgres: marshal fleet definition servers: %w", err)
	}
	var provisionJSON []byte
	if def.Provision != nil {
		provisionJSON, err = json.Marshal(def.Provision)
		if err != nil {
			return domain.FleetDefinition{}, fmt.Errorf("postgres: marshal fleet definition provision: %w", err)
		}
	}
	oldVersion := def.Version - 1
	row := s.pool.QueryRow(ctx, `
		UPDATE infra.fleet_definitions
		SET servers = $1, provision = $2, version = $3, updated_at = now()
		WHERE id = $4 AND tenant_id = $5 AND version = $6
		RETURNING updated_at
	`, serversJSON, provisionJSON, def.Version, def.ID, def.TenantID, oldVersion)
	if err := row.Scan(&def.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.FleetDefinition{}, domain.ErrFleetDefinitionVersionConflict
		}
		return domain.FleetDefinition{}, fmt.Errorf("postgres: update fleet definition: %w", err)
	}
	return def, nil
}

// Get fetches a fleet definition scoped to tenantID.
func (s *FleetDefinitionStore) Get(ctx context.Context, tenantID, id string) (domain.FleetDefinition, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, version, servers, provision, created_by, created_at, updated_at
		FROM infra.fleet_definitions
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)
	def, err := scanFleetDefinition(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.FleetDefinition{}, domain.ErrFleetDefinitionNotFound
	}
	if err != nil {
		return domain.FleetDefinition{}, fmt.Errorf("postgres: get fleet definition: %w", err)
	}
	return def, nil
}

// List returns every fleet definition registered for tenantID.
func (s *FleetDefinitionStore) List(ctx context.Context, tenantID string) ([]domain.FleetDefinition, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, name, version, servers, provision, created_by, created_at, updated_at
		FROM infra.fleet_definitions
		WHERE tenant_id = $1
		ORDER BY name
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query fleet definitions: %w", err)
	}
	defer rows.Close()

	var out []domain.FleetDefinition
	for rows.Next() {
		def, err := scanFleetDefinition(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan fleet definition row: %w", err)
		}
		out = append(out, def)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate fleet definition rows: %w", err)
	}
	return out, nil
}

// fleetDefinitionRowScanner is satisfied by both pgx.Rows and pgx.Row —
// same pattern as this package's rowScanner in repository.go, but scoped
// here since fleet_definitions' scan shape (servers/provision JSON decode)
// is unique to this file.
type fleetDefinitionRowScanner interface {
	Scan(dest ...any) error
}

func scanFleetDefinition(row fleetDefinitionRowScanner) (domain.FleetDefinition, error) {
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
