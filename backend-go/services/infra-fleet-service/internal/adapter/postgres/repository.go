// Package postgres implements infra-fleet-service's DevServerRepository,
// SshTargetRepository, SshTargetResolver, ConnectionRepository,
// ConnectionResolver, and FleetHealthPort ports (defined in internal/usecase)
// against this service's own PostgreSQL database — see
// specs/backend-go/architecture/05-data-architecture.md's
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

// Repository implements usecase.DevServerRepository, usecase.ConnectionRepository,
// usecase.ConnectionResolver, and usecase.FleetHealthPort — one Postgres
// connection pool, several narrow ports, matching usage-service's single
// Repository shape (see that service's internal/adapter/postgres for the
// reference pattern). SSH-target persistence lives on the separate
// SshTargetStore type below (same pool) rather than here — Go's method sets
// allow only one method per name per receiver, and
// DevServerRepository.Get(...) domain.DevServer and
// SshTargetRepository.Get(...) domain.SshTarget both need the name "Get".
type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Register inserts a new dev server and returns the persisted row.
// SSHTargetID is stored as NULL when empty (non-relay-ssh modes) — NULLIF
// against an empty string keeps the FK to infra.ssh_targets satisfiable
// without a sentinel row. The explicit ::uuid cast matters: NULLIF's result
// is text-typed on its own, and Postgres won't implicitly assign text (even
// a NULL text) into a uuid column.
func (r *Repository) Register(ctx context.Context, ds domain.DevServer) (domain.DevServer, error) {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO infra.dev_servers (id, tenant_id, host, connection_mode, ssh_target_id)
		VALUES ($1, $2, $3, $4, NULLIF($5, '')::uuid)
	`, ds.ID, ds.TenantID, ds.Host, string(ds.Mode), ds.SSHTargetID)
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
		SELECT id, tenant_id, host, connection_mode, ssh_target_id
		FROM infra.dev_servers
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)

	var ds domain.DevServer
	var mode string
	var sshTargetID *string
	err := row.Scan(&ds.ID, &ds.TenantID, &ds.Host, &mode, &sshTargetID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DevServer{}, fmt.Errorf("postgres: dev server %q not found for tenant: %w", id, err)
	}
	if err != nil {
		return domain.DevServer{}, fmt.Errorf("postgres: query dev server: %w", err)
	}
	ds.Mode = domain.ConnectionMode(mode)
	if sshTargetID != nil {
		ds.SSHTargetID = *sshTargetID
	}
	return ds, nil
}

// List returns every dev server registered for tenantID.
func (r *Repository) List(ctx context.Context, tenantID string) ([]domain.DevServer, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, host, connection_mode, ssh_target_id
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
		var sshTargetID *string
		if err := rows.Scan(&ds.ID, &ds.TenantID, &ds.Host, &mode, &sshTargetID); err != nil {
			return nil, fmt.Errorf("postgres: scan dev server row: %w", err)
		}
		ds.Mode = domain.ConnectionMode(mode)
		if sshTargetID != nil {
			ds.SSHTargetID = *sshTargetID
		}
		out = append(out, ds)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate dev server rows: %w", err)
	}
	return out, nil
}

// SshTargetStore implements usecase.SshTargetRepository and
// usecase.SshTargetResolver against the same infra.ssh_targets table
// Repository's ResolveConnection/GetFleetHealth etc. read from — split into
// its own type rather than a method on Repository, see Repository's doc
// comment for why (the Get method-name collision with
// DevServerRepository.Get).
type SshTargetStore struct {
	pool *pgxpool.Pool
}

// NewSshTargetStore builds an SshTargetStore over the same pool Repository
// uses — both are thin wrappers over one PostgreSQL connection pool, per
// architecture/05-data-architecture.md's database-per-service rule; this
// just isn't the SAME Go value as Repository (see Repository's doc comment).
func NewSshTargetStore(pool *pgxpool.Pool) *SshTargetStore {
	return &SshTargetStore{pool: pool}
}

// Create inserts a new SSH target and returns the persisted row.
func (s *SshTargetStore) Create(ctx context.Context, target domain.SshTarget) (domain.SshTarget, error) {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO infra.ssh_targets (id, tenant_id, host, user_name, vault_ssh_role)
		VALUES ($1, $2, $3, $4, $5)
	`, target.ID, target.TenantID, target.Host, target.UserName, target.VaultSSHRole)
	if err != nil {
		return domain.SshTarget{}, fmt.Errorf("postgres: insert ssh target: %w", err)
	}
	return target, nil
}

// Get fetches an SSH target scoped to tenantID — implements both
// usecase.SshTargetRepository and usecase.SshTargetResolver (the latter
// consumed by adapter/devserveragent.Client to resolve DevServer.SSHTargetID
// before dialing via sshconn.Connector for relay-ssh mode).
func (s *SshTargetStore) Get(ctx context.Context, tenantID, id string) (domain.SshTarget, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, host, user_name, vault_ssh_role
		FROM infra.ssh_targets
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)

	var target domain.SshTarget
	err := row.Scan(&target.ID, &target.TenantID, &target.Host, &target.UserName, &target.VaultSSHRole)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SshTarget{}, fmt.Errorf("postgres: ssh target %q not found for tenant: %w", id, err)
	}
	if err != nil {
		return domain.SshTarget{}, fmt.Errorf("postgres: query ssh target: %w", err)
	}
	return target, nil
}

// ResolveConnection is the storage-backed half of THE core coordination
// primitive — see usecase.ConnectionResolver's doc comment.
//
// Joins infra.connections -> infra.dev_servers within tenantID's scope
// (migrations/0002_connections) — the real routing model that replaced this
// scaffold's original connectionId==dev_server.id equation. See this
// service's README "Known gaps" for what still isn't wired (port_forwards,
// provider_registry_entries, terminal_sessions).
func (r *Repository) ResolveConnection(ctx context.Context, tenantID, connectionID string) (bool, domain.DevServer, domain.Connection, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT c.id, c.tenant_id, c.dev_server_id, c.repo_path, c.worktree_id,
		       ds.id, ds.tenant_id, ds.host, ds.connection_mode
		FROM infra.connections c
		JOIN infra.dev_servers ds ON ds.id = c.dev_server_id
		WHERE c.tenant_id = $1 AND c.id = $2
	`, tenantID, connectionID)

	var conn domain.Connection
	var ds domain.DevServer
	var mode string
	err := row.Scan(
		&conn.ID, &conn.TenantID, &conn.DevServerID, &conn.RepoPath, &conn.WorktreeID,
		&ds.ID, &ds.TenantID, &ds.Host, &mode,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		// No dev server owns this connectionId within this tenant — not an
		// error, the caller's cue to execute locally.
		return false, domain.DevServer{}, domain.Connection{}, nil
	}
	if err != nil {
		return false, domain.DevServer{}, domain.Connection{}, fmt.Errorf("postgres: resolve connection: %w", err)
	}
	ds.Mode = domain.ConnectionMode(mode)
	return true, ds, conn, nil
}

// CreateConnection inserts a new connection binding and returns the
// persisted row — the write side ResolveConnection's join reads from.
func (r *Repository) CreateConnection(ctx context.Context, conn domain.Connection) (domain.Connection, error) {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO infra.connections (id, tenant_id, dev_server_id, repo_path, worktree_id)
		VALUES ($1, $2, $3, $4, $5)
	`, conn.ID, conn.TenantID, conn.DevServerID, conn.RepoPath, conn.WorktreeID)
	if err != nil {
		return domain.Connection{}, fmt.Errorf("postgres: insert connection: %w", err)
	}
	return conn, nil
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
