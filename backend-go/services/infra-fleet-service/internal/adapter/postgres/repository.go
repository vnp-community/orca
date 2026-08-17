// Package postgres implements infra-fleet-service's DevServerRepository,
// SshTargetRepository, ConnectionResolver, and FleetHealthPort ports
// (defined in internal/usecase) against this service's own PostgreSQL
// database — see specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule: this is the ONLY package in infra-fleet-service
// that knows SQL exists.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// Repository implements usecase.DevServerRepository, usecase.SshTargetRepository,
// usecase.ConnectionResolver, and usecase.FleetHealthPort — one Postgres
// connection pool, several narrow ports, matching usage-service's single
// Repository shape (see that service's internal/adapter/postgres for the
// reference pattern).
type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Register inserts a new dev server and returns the persisted row.
func (r *Repository) Register(ctx context.Context, ds domain.DevServer) (domain.DevServer, error) {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO infra.dev_servers (id, tenant_id, host, connection_mode)
		VALUES ($1, $2, $3, $4)
	`, ds.ID, ds.TenantID, ds.Host, string(ds.Mode))
	if err != nil {
		return domain.DevServer{}, fmt.Errorf("postgres: insert dev server: %w", err)
	}
	return ds, nil
}

// Get fetches a dev server scoped to tenantID — a mismatched tenant_id
// (even for a correctly-guessed UUID) must never return a row, per
// specs/backend-go/services/infra-fleet-service.md §9.
func (r *Repository) Get(ctx context.Context, tenantID, id string) (domain.DevServer, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, host, connection_mode
		FROM infra.dev_servers
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)

	var ds domain.DevServer
	var mode string
	err := row.Scan(&ds.ID, &ds.TenantID, &ds.Host, &mode)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DevServer{}, fmt.Errorf("postgres: dev server %q not found for tenant: %w", id, err)
	}
	if err != nil {
		return domain.DevServer{}, fmt.Errorf("postgres: query dev server: %w", err)
	}
	ds.Mode = domain.ConnectionMode(mode)
	return ds, nil
}

// List returns every dev server registered for tenantID.
func (r *Repository) List(ctx context.Context, tenantID string) ([]domain.DevServer, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, host, connection_mode
		FROM infra.dev_servers
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query dev servers: %w", err)
	}
	defer rows.Close()

	var out []domain.DevServer
	for rows.Next() {
		var ds domain.DevServer
		var mode string
		if err := rows.Scan(&ds.ID, &ds.TenantID, &ds.Host, &mode); err != nil {
			return nil, fmt.Errorf("postgres: scan dev server row: %w", err)
		}
		ds.Mode = domain.ConnectionMode(mode)
		out = append(out, ds)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate dev server rows: %w", err)
	}
	return out, nil
}

// Create inserts a new SSH target and returns the persisted row.
func (r *Repository) Create(ctx context.Context, target domain.SshTarget) (domain.SshTarget, error) {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO infra.ssh_targets (id, tenant_id, host, user_name, vault_ssh_role)
		VALUES ($1, $2, $3, $4, $5)
	`, target.ID, target.TenantID, target.Host, target.UserName, target.VaultSSHRole)
	if err != nil {
		return domain.SshTarget{}, fmt.Errorf("postgres: insert ssh target: %w", err)
	}
	return target, nil
}

// ResolveConnection is the storage-backed half of THE core coordination
// primitive — see usecase.ConnectionResolver's doc comment.
//
// Simplification vs. the full design doc (specs/backend-go/services/infra-fleet-service.md
// §5): this scaffold has no separate `connections` table recording live
// transport/session state — only `dev_servers`. connectionID is resolved
// directly against dev_servers.id within tenantID's scope, i.e.
// connectionId == dev_server_id for now. See this service's README "Known
// gaps" for the follow-up (connections/provider_registry_entries/
// terminal_sessions tables and the in-process connection pool described in
// §8).
func (r *Repository) ResolveConnection(ctx context.Context, tenantID, connectionID string) (bool, domain.DevServer, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, host, connection_mode
		FROM infra.dev_servers
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, connectionID)

	var ds domain.DevServer
	var mode string
	err := row.Scan(&ds.ID, &ds.TenantID, &ds.Host, &mode)
	if errors.Is(err, pgx.ErrNoRows) {
		// No dev server owns this connectionId within this tenant — not an
		// error, the caller's cue to execute locally.
		return false, domain.DevServer{}, nil
	}
	if err != nil {
		return false, domain.DevServer{}, fmt.Errorf("postgres: resolve connection: %w", err)
	}
	ds.Mode = domain.ConnectionMode(mode)
	return true, ds, nil
}

// GetFleetHealth returns the latest fleet-health sample per dev server for
// tenantID, joined through dev_servers since fleet_health has no tenant_id
// column of its own (see migrations/0001_init.up.sql's comment on that
// design choice, matching specs/backend-go/services/infra-fleet-service.md
// §5's note on inheriting tenant scope transitively via FK + join).
func (r *Repository) GetFleetHealth(ctx context.Context, tenantID string) ([]domain.DevServerHealth, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT fh.dev_server_id, fh.reachable, fh.cpu_percent, fh.ram_percent, fh.disk_percent, fh.latency_ms
		FROM infra.fleet_health fh
		JOIN infra.dev_servers ds ON ds.id = fh.dev_server_id
		WHERE ds.tenant_id = $1
		ORDER BY fh.checked_at DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query fleet health: %w", err)
	}
	defer rows.Close()

	var out []domain.DevServerHealth
	for rows.Next() {
		var h domain.DevServerHealth
		if err := rows.Scan(&h.DevServerID, &h.Reachable, &h.CPUPercent, &h.RAMPercent, &h.DiskPercent, &h.LatencyMS); err != nil {
			return nil, fmt.Errorf("postgres: scan fleet health row: %w", err)
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate fleet health rows: %w", err)
	}
	return out, nil
}
