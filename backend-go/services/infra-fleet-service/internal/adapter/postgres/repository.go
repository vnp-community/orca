// Package postgres implements infra-fleet-service's DevServerRepository,
// SshTargetRepository, ConnectionRepository, ConnectionResolver, and
// FleetHealthPort ports (defined in internal/usecase), plus
// adapter/sshrelay.SshTargetResolver (a structurally-identical interface
// that package declares for itself, per this codebase's Dependency
// Inversion convention), against this service's own PostgreSQL database —
// see specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule: this is the ONLY package in infra-fleet-service
// that knows SQL exists.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
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
		INSERT INTO infra.dev_servers (id, tenant_id, host, connection_mode, ssh_target_id, tags)
		VALUES ($1, $2, $3, $4, NULLIF($5, '')::uuid, COALESCE($6, '{}'::text[]))
	`, ds.ID, ds.TenantID, ds.Host, string(ds.Mode), ds.SSHTargetID, ds.Tags)
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
		SELECT id, tenant_id, host, connection_mode, ssh_target_id, tags
		FROM infra.dev_servers
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)

	var ds domain.DevServer
	var mode string
	var sshTargetID *string
	err := row.Scan(&ds.ID, &ds.TenantID, &ds.Host, &mode, &sshTargetID, &ds.Tags)
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
		SELECT id, tenant_id, host, connection_mode, ssh_target_id, tags
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
		if err := rows.Scan(&ds.ID, &ds.TenantID, &ds.Host, &mode, &sshTargetID, &ds.Tags); err != nil {
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

// ListByTag returns tenantID's dev servers carrying tag exactly — backs
// usecase.ListDevServersByTag / workflow-service's "fleet:tag:<tag>"
// dispatch-target shape (TASK-WF-02-02).
func (r *Repository) ListByTag(ctx context.Context, tenantID, tag string) ([]domain.DevServer, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, host, connection_mode, ssh_target_id, tags
		FROM infra.dev_servers
		WHERE tenant_id = $1 AND $2 = ANY(tags)
		ORDER BY created_at DESC
	`, tenantID, tag)
	if err != nil {
		return nil, fmt.Errorf("postgres: query dev servers by tag: %w", err)
	}
	defer rows.Close()

	var out []domain.DevServer
	for rows.Next() {
		var ds domain.DevServer
		var mode string
		var sshTargetID *string
		if err := rows.Scan(&ds.ID, &ds.TenantID, &ds.Host, &mode, &sshTargetID, &ds.Tags); err != nil {
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

// ListAllForPolling returns every dev server across every tenant — the
// health poller (usecase.PollFleetHealth) is not answering one tenant's
// request, unlike every other DevServerRepository method's tenantID
// parameter.
func (r *Repository) ListAllForPolling(ctx context.Context) ([]domain.DevServer, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, host, connection_mode, ssh_target_id, status
		FROM infra.dev_servers
	`)
	if err != nil {
		return nil, fmt.Errorf("postgres: query dev servers for polling: %w", err)
	}
	defer rows.Close()

	var out []domain.DevServer
	for rows.Next() {
		var ds domain.DevServer
		var mode, status string
		var sshTargetID *string
		if err := rows.Scan(&ds.ID, &ds.TenantID, &ds.Host, &mode, &sshTargetID, &status); err != nil {
			return nil, fmt.Errorf("postgres: scan dev server row for polling: %w", err)
		}
		ds.Mode = domain.ConnectionMode(mode)
		ds.Status = domain.DevServerStatus(status)
		if sshTargetID != nil {
			ds.SSHTargetID = *sshTargetID
		}
		out = append(out, ds)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate dev server rows for polling: %w", err)
	}
	return out, nil
}

// SshTargetStore implements usecase.SshTargetRepository and
// adapter/sshrelay.SshTargetResolver against the same infra.ssh_targets
// table Repository's ResolveConnection/GetFleetHealth etc. read from — split
// into its own type rather than a method on Repository, see Repository's
// doc comment for why (the Get method-name collision with
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
	var jumpHostID *string
	if target.JumpHostTargetID != "" {
		jumpHostID = &target.JumpHostTargetID
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO infra.ssh_targets (id, tenant_id, host, port, user_name, vault_ssh_role, known_hosts_fingerprint, jump_host_target_id, project, tags)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, target.ID, target.TenantID, target.Host, target.Port, target.UserName, target.VaultSSHRole, target.KnownHostsFingerprint, jumpHostID, target.Project, target.Tags)
	if err != nil {
		return domain.SshTarget{}, fmt.Errorf("postgres: insert ssh target: %w", err)
	}
	return target, nil
}

// List returns every SSH target registered for tenantID.
func (s *SshTargetStore) List(ctx context.Context, tenantID string) ([]domain.SshTarget, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, host, port, user_name, vault_ssh_role, known_hosts_fingerprint, jump_host_target_id, project, tags
		FROM infra.ssh_targets
		WHERE tenant_id = $1
		ORDER BY host
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query ssh targets: %w", err)
	}
	defer rows.Close()

	var out []domain.SshTarget
	for rows.Next() {
		var t domain.SshTarget
		var jumpHostID *string
		if err := rows.Scan(&t.ID, &t.TenantID, &t.Host, &t.Port, &t.UserName, &t.VaultSSHRole, &t.KnownHostsFingerprint, &jumpHostID, &t.Project, &t.Tags); err != nil {
			return nil, fmt.Errorf("postgres: scan ssh target row: %w", err)
		}
		if jumpHostID != nil {
			t.JumpHostTargetID = *jumpHostID
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate ssh target rows: %w", err)
	}
	return out, nil
}

// Get fetches an SSH target scoped to tenantID — implements both
// usecase.SshTargetRepository and adapter/sshrelay.SshTargetResolver (the
// latter consumed by sshrelay.Provisioner to resolve DevServer.SSHTargetID
// before dialing via sshconn.Connector for relay-ssh mode).
func (s *SshTargetStore) Get(ctx context.Context, tenantID, id string) (domain.SshTarget, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, host, port, user_name, vault_ssh_role, known_hosts_fingerprint, jump_host_target_id, project, tags
		FROM infra.ssh_targets
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)

	var target domain.SshTarget
	var jumpHostID *string
	err := row.Scan(&target.ID, &target.TenantID, &target.Host, &target.Port, &target.UserName, &target.VaultSSHRole, &target.KnownHostsFingerprint, &jumpHostID, &target.Project, &target.Tags)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SshTarget{}, fmt.Errorf("postgres: ssh target %q not found for tenant: %w", id, err)
	}
	if err != nil {
		return domain.SshTarget{}, fmt.Errorf("postgres: query ssh target: %w", err)
	}
	if jumpHostID != nil {
		target.JumpHostTargetID = *jumpHostID
	}
	return target, nil
}

// Upsert inserts or updates by (tenant_id, host, user_name) — the conflict
// target migrations/0007_ssh_target_project_tags's unique index establishes.
// The `xmax != 0` trick is the standard Postgres idiom for "insert vs.
// update" in one round trip, avoiding a separate SELECT ... FOR UPDATE per
// row on a bulk-import fan-in. Port/known-hosts/jump-host (SOL-SSH-01) are
// included so a re-import doesn't silently reset them to their column
// defaults on every upsert of an existing row.
func (s *SshTargetStore) Upsert(ctx context.Context, target domain.SshTarget) (domain.SshTarget, bool, error) {
	var jumpHostID *string
	if target.JumpHostTargetID != "" {
		jumpHostID = &target.JumpHostTargetID
	}
	const query = `
		INSERT INTO infra.ssh_targets (id, tenant_id, host, port, user_name, vault_ssh_role, known_hosts_fingerprint, jump_host_target_id, project, tags)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (tenant_id, host, user_name) DO UPDATE SET
		  vault_ssh_role = EXCLUDED.vault_ssh_role,
		  port = EXCLUDED.port,
		  known_hosts_fingerprint = EXCLUDED.known_hosts_fingerprint,
		  jump_host_target_id = EXCLUDED.jump_host_target_id,
		  project = EXCLUDED.project,
		  tags = EXCLUDED.tags
		RETURNING id, (xmax != 0) AS updated`
	var id string
	var updated bool
	if err := s.pool.QueryRow(ctx, query, target.ID, target.TenantID, target.Host, target.Port, target.UserName, target.VaultSSHRole, target.KnownHostsFingerprint, jumpHostID, target.Project, target.Tags).Scan(&id, &updated); err != nil {
		return domain.SshTarget{}, false, fmt.Errorf("postgres: upsert ssh target: %w", err)
	}
	target.ID = id
	return target, updated, nil
}

// GetByHostUser is a narrow existence-probe used only by the dry-run import
// path (usecase.ImportFleetInventory) — it does not commit anything.
func (s *SshTargetStore) GetByHostUser(ctx context.Context, tenantID, host, userName string) (domain.SshTarget, bool, error) {
	const query = `SELECT id, tenant_id, host, port, user_name, vault_ssh_role, known_hosts_fingerprint, jump_host_target_id, project, tags
		FROM infra.ssh_targets WHERE tenant_id = $1 AND host = $2 AND user_name = $3`
	var t domain.SshTarget
	var jumpHostID *string
	err := s.pool.QueryRow(ctx, query, tenantID, host, userName).Scan(&t.ID, &t.TenantID, &t.Host, &t.Port, &t.UserName, &t.VaultSSHRole, &t.KnownHostsFingerprint, &jumpHostID, &t.Project, &t.Tags)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SshTarget{}, false, nil
	}
	if err != nil {
		return domain.SshTarget{}, false, fmt.Errorf("postgres: query ssh target by host/user: %w", err)
	}
	if jumpHostID != nil {
		t.JumpHostTargetID = *jumpHostID
	}
	return t, true, nil
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
	const q = `
		SELECT c.id, c.tenant_id, c.dev_server_id, c.repo_path, c.worktree_id,
		       ds.id, ds.tenant_id, ds.host, ds.connection_mode
		FROM infra.connections c
		JOIN infra.dev_servers ds ON ds.id = c.dev_server_id
		WHERE c.tenant_id = $1 AND c.id = $2`
	return r.scanConnectionRow(ctx, q, tenantID, connectionID)
}

// ResolveConnectionByDevServer is ResolveConnection's reverse-lookup
// counterpart — see usecase.ConnectionResolver's doc comment (TASK-025).
// A dev server can have had multiple connection rows over time — resolves
// the most recently created one.
func (r *Repository) ResolveConnectionByDevServer(ctx context.Context, tenantID, devServerID string) (bool, domain.DevServer, domain.Connection, error) {
	const q = `
		SELECT c.id, c.tenant_id, c.dev_server_id, c.repo_path, c.worktree_id,
		       ds.id, ds.tenant_id, ds.host, ds.connection_mode
		FROM infra.connections c
		JOIN infra.dev_servers ds ON ds.id = c.dev_server_id
		WHERE c.tenant_id = $1 AND c.dev_server_id = $2
		ORDER BY c.created_at DESC
		LIMIT 1`
	return r.scanConnectionRow(ctx, q, tenantID, devServerID)
}

// ResolveConnectionByWorktree is ResolveConnection's worktree-keyed
// counterpart — see usecase.ConnectionResolver's doc comment (TASK-025).
func (r *Repository) ResolveConnectionByWorktree(ctx context.Context, tenantID, worktreeID string) (bool, domain.DevServer, domain.Connection, error) {
	const q = `
		SELECT c.id, c.tenant_id, c.dev_server_id, c.repo_path, c.worktree_id,
		       ds.id, ds.tenant_id, ds.host, ds.connection_mode
		FROM infra.connections c
		JOIN infra.dev_servers ds ON ds.id = c.dev_server_id
		WHERE c.tenant_id = $1 AND c.worktree_id = $2
		ORDER BY c.created_at DESC
		LIMIT 1`
	return r.scanConnectionRow(ctx, q, tenantID, worktreeID)
}

// scanConnectionRow factors out ResolveConnection/ResolveConnectionByDevServer/
// ResolveConnectionByWorktree's shared row-scan + "no rows means
// connected=false, not an error" handling (TASK-025) — same column order,
// same pgx.ErrNoRows branch, so all three callers share it instead of
// duplicating the scan 3 ways.
func (r *Repository) scanConnectionRow(ctx context.Context, query string, args ...any) (bool, domain.DevServer, domain.Connection, error) {
	row := r.pool.QueryRow(ctx, query, args...)

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
		INSERT INTO infra.connections (id, tenant_id, dev_server_id, repo_path, worktree_id, status, last_activity_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, conn.ID, conn.TenantID, conn.DevServerID, conn.RepoPath, conn.WorktreeID, conn.Status, conn.LastActivityAt)
	if err != nil {
		return domain.Connection{}, fmt.Errorf("postgres: insert connection: %w", err)
	}
	return conn, nil
}

// CreateConnectionWithOutbox inserts a new connection binding and enqueues
// event as an infra.outbox_events row — both in ONE transaction (Epic G's
// transactional-outbox pattern; see domain.OutboxEvent's doc comment).
// common/outbox.Relay (wired in cmd/server/main.go) is the only thing that
// reads infra.outbox_events afterward — this method's job ends at durably
// committing the row, not publishing it.
func (r *Repository) CreateConnectionWithOutbox(ctx context.Context, conn domain.Connection, event domain.OutboxEvent) (domain.Connection, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Connection{}, fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		INSERT INTO infra.connections (id, tenant_id, dev_server_id, repo_path, worktree_id, status, last_activity_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, conn.ID, conn.TenantID, conn.DevServerID, conn.RepoPath, conn.WorktreeID, conn.Status, conn.LastActivityAt); err != nil {
		return domain.Connection{}, fmt.Errorf("postgres: insert connection: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO infra.outbox_events (id, tenant_id, subject, occurred_at, version, payload)
		VALUES ($1, $2, $3, $4, 1, $5)
	`, event.ID, conn.TenantID, event.Subject, event.OccurredAt, event.PayloadJSON); err != nil {
		return domain.Connection{}, fmt.Errorf("postgres: insert outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Connection{}, fmt.Errorf("postgres: commit tx: %w", err)
	}
	return conn, nil
}

// FetchUnpublished and MarkPublished (both implementing common/outbox.Store)
// live in outbox.go alongside EnqueueOutboxEvent, not here — CreateConnectionWithOutbox
// above and adapter/eventbus.HealthPublisher both enqueue into the same
// infra.outbox_events table this Repository's pool already owns.

// GetActiveByDevServer returns the most recent connection for devServerID
// whose status is not "closed" — see Connection.Status's doc comment
// (migrations/0004_connection_status).
func (r *Repository) GetActiveByDevServer(ctx context.Context, tenantID, devServerID string) (domain.Connection, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, dev_server_id, repo_path, worktree_id, status, last_activity_at
		FROM infra.connections
		WHERE tenant_id = $1 AND dev_server_id = $2 AND status <> 'closed'
		ORDER BY created_at DESC
		LIMIT 1
	`, tenantID, devServerID)

	var conn domain.Connection
	err := row.Scan(&conn.ID, &conn.TenantID, &conn.DevServerID, &conn.RepoPath, &conn.WorktreeID, &conn.Status, &conn.LastActivityAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Connection{}, false, nil
	}
	if err != nil {
		return domain.Connection{}, false, fmt.Errorf("postgres: get active connection by dev server: %w", err)
	}
	return conn, true, nil
}

// UpdateStatus sets connectionID's status column — TeardownConnection's
// "mark closed" step (BR-SSH-13).
func (r *Repository) UpdateStatus(ctx context.Context, tenantID, connectionID, status string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE infra.connections
		SET status = $1
		WHERE tenant_id = $2 AND id = $3
	`, status, tenantID, connectionID)
	if err != nil {
		return fmt.Errorf("postgres: update connection status: %w", err)
	}
	return nil
}

// GetDevServerByConnection resolves connectionID's owning DevServer —
// mirrors ResolveConnection's join shape but returns only the DevServer
// half, since TeardownConnection only needs devServer.ID for
// CancelReconnect.
func (r *Repository) GetDevServerByConnection(ctx context.Context, tenantID, connectionID string) (domain.DevServer, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT ds.id, ds.tenant_id, ds.host, ds.connection_mode, ds.ssh_target_id
		FROM infra.connections c
		JOIN infra.dev_servers ds ON ds.id = c.dev_server_id
		WHERE c.tenant_id = $1 AND c.id = $2
	`, tenantID, connectionID)

	var devServer domain.DevServer
	var mode string
	var sshTargetID *string
	err := row.Scan(&devServer.ID, &devServer.TenantID, &devServer.Host, &mode, &sshTargetID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DevServer{}, false, nil
	}
	if err != nil {
		return domain.DevServer{}, false, fmt.Errorf("postgres: get dev server by connection: %w", err)
	}
	devServer.Mode = domain.ConnectionMode(mode)
	if sshTargetID != nil {
		devServer.SSHTargetID = *sshTargetID
	}
	return devServer, true, nil
}

// FindBySshTarget returns the DevServer bound to sshTargetID, if any.
func (r *Repository) FindBySshTarget(ctx context.Context, tenantID, sshTargetID string) (domain.DevServer, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, host, connection_mode, ssh_target_id
		FROM infra.dev_servers
		WHERE tenant_id = $1 AND ssh_target_id = $2
		LIMIT 1
	`, tenantID, sshTargetID)

	var ds domain.DevServer
	var mode string
	err := row.Scan(&ds.ID, &ds.TenantID, &ds.Host, &mode, &ds.SSHTargetID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DevServer{}, false, nil
	}
	if err != nil {
		return domain.DevServer{}, false, fmt.Errorf("postgres: find dev server by ssh target: %w", err)
	}
	ds.Mode = domain.ConnectionMode(mode)
	return ds, true, nil
}

// UpdateProvisionResult persists the outcome of one provisioning attempt —
// see usecase.DevServerRepository.UpdateProvisionResult's doc comment.
func (r *Repository) UpdateProvisionResult(ctx context.Context, tenantID, id string, status domain.DevServerStatus, info usecase.HandshakeInfo, provisionedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE infra.dev_servers
		SET status = $3, platform = $4, arch = $5, node_version = $6, agent_version = $7, last_provisioned_at = $8
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id, string(status), info.Platform, info.Arch, info.NodeVersion, info.AgentVersion, provisionedAt)
	if err != nil {
		return fmt.Errorf("postgres: update dev server provision result: %w", err)
	}
	return nil
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

// UpsertFleetHealth implements usecase.FleetHealthWriter.UpsertFleetHealth —
// dev_server_id is fleet_health's primary key (migrations/0001_init), so
// this is a plain upsert-by-PK, one row per dev server, latest sample wins.
func (r *Repository) UpsertFleetHealth(ctx context.Context, sample domain.DevServerHealth) error {
	const query = `
		INSERT INTO infra.fleet_health (dev_server_id, reachable, cpu_percent, ram_percent, disk_percent, latency_ms, status, checked_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, now())
		ON CONFLICT (dev_server_id) DO UPDATE SET
		  reachable = EXCLUDED.reachable, cpu_percent = EXCLUDED.cpu_percent,
		  ram_percent = EXCLUDED.ram_percent, disk_percent = EXCLUDED.disk_percent,
		  latency_ms = EXCLUDED.latency_ms, status = EXCLUDED.status, checked_at = now()`
	_, err := r.pool.Exec(ctx, query, sample.DevServerID, sample.Reachable, sample.CPUPercent, sample.RAMPercent, sample.DiskPercent, sample.LatencyMS, string(sample.Status))
	if err != nil {
		return fmt.Errorf("postgres: upsert fleet health: %w", err)
	}
	return nil
}

// PortForwardStore is domain.PortForward's storage — same
// own-Go-value-not-the-same-as-Repository shape as SshTargetStore, sharing
// the same connection pool.
type PortForwardStore struct {
	pool *pgxpool.Pool
}

// NewPortForwardStore builds a PortForwardStore over the same pool
// Repository/SshTargetStore use.
func NewPortForwardStore(pool *pgxpool.Pool) *PortForwardStore {
	return &PortForwardStore{pool: pool}
}

// Create inserts a new port forward and returns the persisted row.
func (s *PortForwardStore) Create(ctx context.Context, pf domain.PortForward) (domain.PortForward, error) {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO infra.port_forwards (id, tenant_id, connection_id, local_port, remote_port, process_name, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, pf.ID, pf.TenantID, pf.ConnectionID, pf.LocalPort, pf.RemotePort, pf.ProcessName, string(pf.Status))
	if err != nil {
		return domain.PortForward{}, fmt.Errorf("postgres: insert port forward: %w", err)
	}
	return pf, nil
}

// UpdateStatus sets id's status column — PollWorkspacePorts' teardown step
// (BR-SSH-18) writes "closed" here rather than deleting the row, keeping a
// history of forwards for the connection's lifetime.
func (s *PortForwardStore) UpdateStatus(ctx context.Context, tenantID, id string, status domain.PortForwardStatus) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE infra.port_forwards
		SET status = $1
		WHERE tenant_id = $2 AND id = $3
	`, string(status), tenantID, id)
	if err != nil {
		return fmt.Errorf("postgres: update port forward status: %w", err)
	}
	return nil
}

// GetPrevious implements usecase.FleetHealthWriter.GetPrevious — the
// last-persisted sample for devServerID, which PollFleetHealth diffs
// against to detect a status_change.
func (r *Repository) GetPrevious(ctx context.Context, devServerID string) (domain.DevServerHealth, bool, error) {
	const query = `SELECT dev_server_id, reachable, cpu_percent, ram_percent, disk_percent, latency_ms, status
		FROM infra.fleet_health WHERE dev_server_id = $1`
	var h domain.DevServerHealth
	var status string
	err := r.pool.QueryRow(ctx, query, devServerID).Scan(&h.DevServerID, &h.Reachable, &h.CPUPercent, &h.RAMPercent, &h.DiskPercent, &h.LatencyMS, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DevServerHealth{}, false, nil
	}
	if err != nil {
		return domain.DevServerHealth{}, false, fmt.Errorf("postgres: get previous fleet health: %w", err)
	}
	h.Status = domain.HealthStatus(status)
	return h, true, nil
}

// TryLock implements usecase.PollLockPort.TryLock via a Postgres
// session-level advisory lock keyed by hashtext(devServerID) — non-blocking
// (pg_try_advisory_lock, not pg_advisory_lock), so a replica that loses the
// race skips this server this tick rather than queueing. Uses a held
// connection acquired from the pool (not a bare pool.Exec/QueryRow), since
// advisory locks are session-scoped: the lock and its later unlock must run
// on literally the same underlying connection, and unlock releases the
// connection back to the pool.
func (r *Repository) TryLock(ctx context.Context, devServerID string) (bool, func(), error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return false, nil, fmt.Errorf("postgres: acquire connection for advisory lock: %w", err)
	}
	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtext($1))`, devServerID).Scan(&locked); err != nil {
		conn.Release()
		return false, nil, fmt.Errorf("postgres: try advisory lock: %w", err)
	}
	if !locked {
		conn.Release()
		return false, nil, nil
	}
	unlock := func() {
		// Why context.Background(): unlock typically runs in a defer after
		// the caller's ctx may already be done (poll tick finished) —
		// releasing the advisory lock must not be skipped just because the
		// tick's own context expired.
		_, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtext($1))`, devServerID)
		conn.Release()
	}
	return true, unlock, nil
}

// ListActiveByConnection returns every non-closed port forward for
// connectionID.
func (s *PortForwardStore) ListActiveByConnection(ctx context.Context, tenantID, connectionID string) ([]domain.PortForward, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, connection_id, local_port, remote_port, process_name, status
		FROM infra.port_forwards
		WHERE tenant_id = $1 AND connection_id = $2 AND status <> 'closed'
		ORDER BY created_at DESC
	`, tenantID, connectionID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query port forwards: %w", err)
	}
	defer rows.Close()

	var out []domain.PortForward
	for rows.Next() {
		var pf domain.PortForward
		var status string
		if err := rows.Scan(&pf.ID, &pf.TenantID, &pf.ConnectionID, &pf.LocalPort, &pf.RemotePort, &pf.ProcessName, &status); err != nil {
			return nil, fmt.Errorf("postgres: scan port forward row: %w", err)
		}
		pf.Status = domain.PortForwardStatus(status)
		out = append(out, pf)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate port forward rows: %w", err)
	}
	return out, nil
}
