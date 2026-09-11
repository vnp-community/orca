// Package mysql implements infra-fleet-service's DevServerRepository,
// SshTargetRepository, ConnectionRepository, ConnectionResolver, and
// FleetHealthPort ports (defined in internal/usecase) against MySQL/TiDB —
// the dialect counterpart to internal/adapter/postgres, per
// specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-017.
package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

// Repository implements usecase.DevServerRepository, usecase.ConnectionRepository,
// usecase.ConnectionResolver, usecase.FleetHealthPort/FleetHealthWriter,
// usecase.PollLockPort, usecase.FleetConnectivityRepository,
// usecase.FleetHealthPollerRepository, and usecase.OutboxWriter — mirrors
// internal/adapter/postgres.Repository's shape 1:1 over *sql.DB instead of
// *pgxpool.Pool.
type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// marshalTags encodes a []string as a JSON array, defaulting a nil/empty
// slice to "[]" (never SQL NULL or the literal string "null") — the MySQL
// counterpart to Postgres's COALESCE($n, '{}'::text[]), since tags is
// stored as JSON (migrations/mysql/0023), not a native array type.
func marshalTags(tags []string) ([]byte, error) {
	if tags == nil {
		tags = []string{}
	}
	return json.Marshal(tags)
}

func unmarshalTags(raw []byte, out *[]string) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

// Register inserts a new dev server and returns the persisted row.
// ssh_target_id/group_id are stored as NULL when empty (non-relay-ssh
// modes / no group) — no ::uuid cast needed, CHAR(36) already accepts a
// plain string.
func (r *Repository) Register(ctx context.Context, ds domain.DevServer) (domain.DevServer, error) {
	tagsJSON, err := marshalTags(ds.Tags)
	if err != nil {
		return domain.DevServer{}, fmt.Errorf("mysql: marshal dev server tags: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO dev_servers (id, tenant_id, host, connection_mode, ssh_target_id, tags, approval_status, group_id, kind)
		VALUES (?, ?, ?, ?, NULLIF(?, ''), ?, ?, NULLIF(?, ''), ?)
	`, ds.ID, ds.TenantID, ds.Host, string(ds.Mode), ds.SSHTargetID, tagsJSON, string(ds.Status), ds.GroupID, string(ds.Kind))
	if err != nil {
		return domain.DevServer{}, fmt.Errorf("mysql: insert dev server: %w", err)
	}
	return ds, nil
}

// Get fetches a dev server scoped to tenantID — a mismatched tenant_id
// (even for a correctly-guessed id) must never return a row, per
// specs/backend-go/services/infra-fleet-service.md §9.
func (r *Repository) Get(ctx context.Context, tenantID, id string) (domain.DevServer, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, host, connection_mode, ssh_target_id, tags, approval_status, group_id, kind, status
		FROM dev_servers
		WHERE tenant_id = ? AND id = ?
	`, tenantID, id)

	var ds domain.DevServer
	var mode, status, kind, healthStatus string
	var sshTargetID, groupID sql.NullString
	var tagsJSON []byte
	err := row.Scan(&ds.ID, &ds.TenantID, &ds.Host, &mode, &sshTargetID, &tagsJSON, &status, &groupID, &kind, &healthStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DevServer{}, fmt.Errorf("mysql: dev server %q not found for tenant: %w", id, err)
	}
	if err != nil {
		return domain.DevServer{}, fmt.Errorf("mysql: query dev server: %w", err)
	}
	if err := unmarshalTags(tagsJSON, &ds.Tags); err != nil {
		return domain.DevServer{}, fmt.Errorf("mysql: decode dev server tags: %w", err)
	}
	ds.Mode = domain.ConnectionMode(mode)
	ds.Status = domain.DevServerStatus(status)
	ds.Kind = domain.AgentKind(kind)
	ds.HealthStatus = domain.DevServerHealthStatus(healthStatus)
	if sshTargetID.Valid {
		ds.SSHTargetID = sshTargetID.String
	}
	if groupID.Valid {
		ds.GroupID = groupID.String
	}
	return ds, nil
}

// List returns every dev server registered for tenantID.
func (r *Repository) List(ctx context.Context, tenantID string) ([]domain.DevServer, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, tenant_id, host, connection_mode, ssh_target_id, tags, approval_status, group_id, kind, status
		FROM dev_servers
		WHERE tenant_id = ?
		ORDER BY created_at DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query dev servers: %w", err)
	}
	defer rows.Close()

	var out []domain.DevServer
	for rows.Next() {
		var ds domain.DevServer
		var mode, status, kind, healthStatus string
		var sshTargetID, groupID sql.NullString
		var tagsJSON []byte
		if err := rows.Scan(&ds.ID, &ds.TenantID, &ds.Host, &mode, &sshTargetID, &tagsJSON, &status, &groupID, &kind, &healthStatus); err != nil {
			return nil, fmt.Errorf("mysql: scan dev server row: %w", err)
		}
		if err := unmarshalTags(tagsJSON, &ds.Tags); err != nil {
			return nil, fmt.Errorf("mysql: decode dev server tags: %w", err)
		}
		ds.Mode = domain.ConnectionMode(mode)
		ds.Status = domain.DevServerStatus(status)
		ds.Kind = domain.AgentKind(kind)
		ds.HealthStatus = domain.DevServerHealthStatus(healthStatus)
		if sshTargetID.Valid {
			ds.SSHTargetID = sshTargetID.String
		}
		if groupID.Valid {
			ds.GroupID = groupID.String
		}
		out = append(out, ds)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate dev server rows: %w", err)
	}
	return out, nil
}

// ListByTag returns tenantID's dev servers carrying tag exactly — backs
// usecase.ListDevServersByTag / workflow-service's "fleet:tag:<tag>"
// dispatch-target shape (TASK-WF-02-02). JSON_CONTAINS is the MySQL/TiDB
// equivalent of Postgres's `$2 = ANY(tags)` array-membership test — tags is
// a JSON array of strings (migrations/mysql/0023), not a native array
// type, so this is a JSON containment check, not an index-backed GIN
// lookup (see that migration's comment on the accepted cost).
func (r *Repository) ListByTag(ctx context.Context, tenantID, tag string) ([]domain.DevServer, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, tenant_id, host, connection_mode, ssh_target_id, tags
		FROM dev_servers
		WHERE tenant_id = ? AND JSON_CONTAINS(tags, JSON_QUOTE(?))
		ORDER BY created_at DESC
	`, tenantID, tag)
	if err != nil {
		return nil, fmt.Errorf("mysql: query dev servers by tag: %w", err)
	}
	defer rows.Close()

	var out []domain.DevServer
	for rows.Next() {
		var ds domain.DevServer
		var mode string
		var sshTargetID sql.NullString
		var tagsJSON []byte
		if err := rows.Scan(&ds.ID, &ds.TenantID, &ds.Host, &mode, &sshTargetID, &tagsJSON); err != nil {
			return nil, fmt.Errorf("mysql: scan dev server row: %w", err)
		}
		if err := unmarshalTags(tagsJSON, &ds.Tags); err != nil {
			return nil, fmt.Errorf("mysql: decode dev server tags: %w", err)
		}
		ds.Mode = domain.ConnectionMode(mode)
		if sshTargetID.Valid {
			ds.SSHTargetID = sshTargetID.String
		}
		out = append(out, ds)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate dev server rows: %w", err)
	}
	return out, nil
}

// ListAllForPolling returns every dev server across every tenant — the
// health poller (usecase.PollFleetHealth) is not answering one tenant's
// request, unlike every other DevServerRepository method's tenantID
// parameter.
func (r *Repository) ListAllForPolling(ctx context.Context) ([]domain.DevServer, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, tenant_id, host, connection_mode, ssh_target_id, status
		FROM dev_servers
	`)
	if err != nil {
		return nil, fmt.Errorf("mysql: query dev servers for polling: %w", err)
	}
	defer rows.Close()

	var out []domain.DevServer
	for rows.Next() {
		var ds domain.DevServer
		var mode, status string
		var sshTargetID sql.NullString
		if err := rows.Scan(&ds.ID, &ds.TenantID, &ds.Host, &mode, &sshTargetID, &status); err != nil {
			return nil, fmt.Errorf("mysql: scan dev server row for polling: %w", err)
		}
		ds.Mode = domain.ConnectionMode(mode)
		// The dev_servers.status column is health/connectivity, NOT
		// CR-DS-006 admin approval — see domain.DevServerHealthStatus's doc
		// comment for why these are deliberately separate Go types.
		ds.HealthStatus = domain.DevServerHealthStatus(status)
		if sshTargetID.Valid {
			ds.SSHTargetID = sshTargetID.String
		}
		out = append(out, ds)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate dev server rows for polling: %w", err)
	}
	return out, nil
}

// UpdateApprovalStatus sets a dev server's approval_status, scoped to
// tenantID — CR-DS-006 Phase 2. No RETURNING on MySQL — UPDATE then SELECT
// back, per BE-DB-SOL-005 §3's translation.
func (r *Repository) UpdateApprovalStatus(ctx context.Context, tenantID, devServerID string, status domain.DevServerStatus) (domain.DevServer, error) {
	_, err := r.db.ExecContext(ctx, `
		UPDATE dev_servers
		SET approval_status = ?
		WHERE tenant_id = ? AND id = ?
	`, string(status), tenantID, devServerID)
	if err != nil {
		return domain.DevServer{}, fmt.Errorf("mysql: update dev server approval status: %w", err)
	}
	return r.selectDevServerRow(ctx, tenantID, devServerID)
}

// AssignGroup sets (or, when groupID == "", clears) a dev server's
// group_id, scoped to tenantID — CR-DS-006 Phase 2. NULLIF(groupID, ”)
// makes an empty string clear the FK to NULL, same pattern Register uses.
func (r *Repository) AssignGroup(ctx context.Context, tenantID, devServerID, groupID string) (domain.DevServer, error) {
	_, err := r.db.ExecContext(ctx, `
		UPDATE dev_servers
		SET group_id = NULLIF(?, '')
		WHERE tenant_id = ? AND id = ?
	`, groupID, tenantID, devServerID)
	if err != nil {
		return domain.DevServer{}, fmt.Errorf("mysql: assign dev server group: %w", err)
	}
	return r.selectDevServerRow(ctx, tenantID, devServerID)
}

// selectDevServerRow re-reads the 9-column dev_servers projection
// UpdateApprovalStatus/AssignGroup's Postgres RETURNING clause used to
// return — shared here the same way scanDevServerRow was shared there.
func (r *Repository) selectDevServerRow(ctx context.Context, tenantID, devServerID string) (domain.DevServer, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, host, connection_mode, ssh_target_id, approval_status, group_id, kind, status
		FROM dev_servers WHERE tenant_id = ? AND id = ?
	`, tenantID, devServerID)
	var ds domain.DevServer
	var mode, status, kind, healthStatus string
	var sshTargetID, groupID sql.NullString
	err := row.Scan(&ds.ID, &ds.TenantID, &ds.Host, &mode, &sshTargetID, &status, &groupID, &kind, &healthStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DevServer{}, fmt.Errorf("mysql: dev server not found for tenant: %w", err)
	}
	if err != nil {
		return domain.DevServer{}, fmt.Errorf("mysql: reading back updated dev server: %w", err)
	}
	ds.Mode = domain.ConnectionMode(mode)
	ds.Status = domain.DevServerStatus(status)
	ds.Kind = domain.AgentKind(kind)
	ds.HealthStatus = domain.DevServerHealthStatus(healthStatus)
	if sshTargetID.Valid {
		ds.SSHTargetID = sshTargetID.String
	}
	if groupID.Valid {
		ds.GroupID = groupID.String
	}
	return ds, nil
}

// SshTargetStore implements usecase.SshTargetRepository and
// adapter/sshrelay.SshTargetResolver against the same ssh_targets table
// Repository reads from — split into its own type, mirroring
// internal/adapter/postgres.SshTargetStore's identical Get-method-name-
// collision rationale.
type SshTargetStore struct {
	db *sql.DB
}

func NewSshTargetStore(db *sql.DB) *SshTargetStore {
	return &SshTargetStore{db: db}
}

// Create inserts a new SSH target and returns the persisted row.
func (s *SshTargetStore) Create(ctx context.Context, target domain.SshTarget) (domain.SshTarget, error) {
	tagsJSON, err := marshalTags(target.Tags)
	if err != nil {
		return domain.SshTarget{}, fmt.Errorf("mysql: marshal ssh target tags: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO ssh_targets (id, tenant_id, host, port, user_name, vault_ssh_role, known_hosts_fingerprint, jump_host_target_id, project, tags)
		VALUES (?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?)
	`, target.ID, target.TenantID, target.Host, target.Port, target.UserName, target.VaultSSHRole, target.KnownHostsFingerprint, target.JumpHostTargetID, target.Project, tagsJSON)
	if err != nil {
		return domain.SshTarget{}, fmt.Errorf("mysql: insert ssh target: %w", err)
	}
	return target, nil
}

func scanSshTarget(row rowScanner) (domain.SshTarget, error) {
	var t domain.SshTarget
	var jumpHostID sql.NullString
	var tagsJSON []byte
	if err := row.Scan(&t.ID, &t.TenantID, &t.Host, &t.Port, &t.UserName, &t.VaultSSHRole, &t.KnownHostsFingerprint, &jumpHostID, &t.Project, &tagsJSON); err != nil {
		return domain.SshTarget{}, err
	}
	if err := unmarshalTags(tagsJSON, &t.Tags); err != nil {
		return domain.SshTarget{}, fmt.Errorf("decode ssh target tags: %w", err)
	}
	if jumpHostID.Valid {
		t.JumpHostTargetID = jumpHostID.String
	}
	return t, nil
}

// List returns every SSH target registered for tenantID.
func (s *SshTargetStore) List(ctx context.Context, tenantID string) ([]domain.SshTarget, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, host, port, user_name, vault_ssh_role, known_hosts_fingerprint, jump_host_target_id, project, tags
		FROM ssh_targets
		WHERE tenant_id = ?
		ORDER BY host
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query ssh targets: %w", err)
	}
	defer rows.Close()

	var out []domain.SshTarget
	for rows.Next() {
		t, err := scanSshTarget(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan ssh target row: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate ssh target rows: %w", err)
	}
	return out, nil
}

// Get fetches an SSH target scoped to tenantID — implements both
// usecase.SshTargetRepository and adapter/sshrelay.SshTargetResolver.
func (s *SshTargetStore) Get(ctx context.Context, tenantID, id string) (domain.SshTarget, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, host, port, user_name, vault_ssh_role, known_hosts_fingerprint, jump_host_target_id, project, tags
		FROM ssh_targets
		WHERE tenant_id = ? AND id = ?
	`, tenantID, id)
	target, err := scanSshTarget(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.SshTarget{}, fmt.Errorf("mysql: ssh target %q not found for tenant: %w", id, err)
	}
	if err != nil {
		return domain.SshTarget{}, fmt.Errorf("mysql: query ssh target: %w", err)
	}
	return target, nil
}

// Upsert inserts or updates by (tenant_id, host, user_name) — the conflict
// target migrations/mysql/0021's unique index establishes.
// ON DUPLICATE KEY UPDATE is MySQL's ON CONFLICT DO UPDATE equivalent, but
// has no RETURNING/xmax trick to report insert-vs-update — MySQL documents
// INSERT ... ON DUPLICATE KEY UPDATE's own affected-rows convention
// instead: 1 row affected means a fresh INSERT, 2 means an existing row was
// updated, 0 means an existing row matched but every value was already
// identical (still "hit an existing row", just no column literally
// changed) — so `updated := affected != 1` reproduces `xmax != 0`'s exact
// insert-vs-update signal. The row's real id is read back afterward rather
// than trusted from target.ID, since a conflict means an EXISTING row (with
// its own, possibly different, id) was the one actually touched.
func (s *SshTargetStore) Upsert(ctx context.Context, target domain.SshTarget) (domain.SshTarget, bool, error) {
	tagsJSON, err := marshalTags(target.Tags)
	if err != nil {
		return domain.SshTarget{}, false, fmt.Errorf("mysql: marshal ssh target tags: %w", err)
	}
	const query = `
		INSERT INTO ssh_targets (id, tenant_id, host, port, user_name, vault_ssh_role, known_hosts_fingerprint, jump_host_target_id, project, tags)
		VALUES (?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?)
		ON DUPLICATE KEY UPDATE
		  vault_ssh_role = VALUES(vault_ssh_role),
		  port = VALUES(port),
		  known_hosts_fingerprint = VALUES(known_hosts_fingerprint),
		  jump_host_target_id = VALUES(jump_host_target_id),
		  project = VALUES(project),
		  tags = VALUES(tags)`
	res, err := s.db.ExecContext(ctx, query, target.ID, target.TenantID, target.Host, target.Port, target.UserName, target.VaultSSHRole, target.KnownHostsFingerprint, target.JumpHostTargetID, target.Project, tagsJSON)
	if err != nil {
		return domain.SshTarget{}, false, fmt.Errorf("mysql: upsert ssh target: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return domain.SshTarget{}, false, fmt.Errorf("mysql: rows affected: %w", err)
	}
	updated := affected != 1

	var id string
	row := s.db.QueryRowContext(ctx, `SELECT id FROM ssh_targets WHERE tenant_id = ? AND host = ? AND user_name = ?`, target.TenantID, target.Host, target.UserName)
	if err := row.Scan(&id); err != nil {
		return domain.SshTarget{}, false, fmt.Errorf("mysql: reading back upserted ssh target id: %w", err)
	}
	target.ID = id
	return target, updated, nil
}

// GetByHostUser is a narrow existence-probe used only by the dry-run import
// path (usecase.ImportFleetInventory) — it does not commit anything.
func (s *SshTargetStore) GetByHostUser(ctx context.Context, tenantID, host, userName string) (domain.SshTarget, bool, error) {
	const query = `SELECT id, tenant_id, host, port, user_name, vault_ssh_role, known_hosts_fingerprint, jump_host_target_id, project, tags
		FROM ssh_targets WHERE tenant_id = ? AND host = ? AND user_name = ?`
	t, err := scanSshTarget(s.db.QueryRowContext(ctx, query, tenantID, host, userName))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.SshTarget{}, false, nil
	}
	if err != nil {
		return domain.SshTarget{}, false, fmt.Errorf("mysql: query ssh target by host/user: %w", err)
	}
	return t, true, nil
}

// Delete removes an SSH target scoped to tenantID — used by
// DeleteSshTarget's compensating-rollback path in BulkProvisionFleet
// (CR-FLEET-001) when RegisterDevServer fails after CreateSshTarget
// already succeeded.
func (s *SshTargetStore) Delete(ctx context.Context, tenantID, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM ssh_targets WHERE id = ? AND tenant_id = ?`, id, tenantID)
	if err != nil {
		return fmt.Errorf("mysql: delete ssh target: %w", err)
	}
	return nil
}

// ResolveConnection is the storage-backed half of THE core coordination
// primitive — see usecase.ConnectionResolver's doc comment.
func (r *Repository) ResolveConnection(ctx context.Context, tenantID, connectionID string) (bool, domain.DevServer, domain.Connection, error) {
	const q = `
		SELECT c.id, c.tenant_id, c.dev_server_id, c.repo_path, c.worktree_id,
		       ds.id, ds.tenant_id, ds.host, ds.connection_mode
		FROM connections c
		JOIN dev_servers ds ON ds.id = c.dev_server_id
		WHERE c.tenant_id = ? AND c.id = ?`
	return r.scanConnectionRow(ctx, q, tenantID, connectionID)
}

// ResolveConnectionByDevServer is ResolveConnection's reverse-lookup
// counterpart — a dev server can have had multiple connection rows over
// time, resolves the most recently created one.
func (r *Repository) ResolveConnectionByDevServer(ctx context.Context, tenantID, devServerID string) (bool, domain.DevServer, domain.Connection, error) {
	const q = `
		SELECT c.id, c.tenant_id, c.dev_server_id, c.repo_path, c.worktree_id,
		       ds.id, ds.tenant_id, ds.host, ds.connection_mode
		FROM connections c
		JOIN dev_servers ds ON ds.id = c.dev_server_id
		WHERE c.tenant_id = ? AND c.dev_server_id = ?
		ORDER BY c.created_at DESC
		LIMIT 1`
	return r.scanConnectionRow(ctx, q, tenantID, devServerID)
}

// ResolveConnectionByWorktree is ResolveConnection's worktree-keyed
// counterpart.
func (r *Repository) ResolveConnectionByWorktree(ctx context.Context, tenantID, worktreeID string) (bool, domain.DevServer, domain.Connection, error) {
	const q = `
		SELECT c.id, c.tenant_id, c.dev_server_id, c.repo_path, c.worktree_id,
		       ds.id, ds.tenant_id, ds.host, ds.connection_mode
		FROM connections c
		JOIN dev_servers ds ON ds.id = c.dev_server_id
		WHERE c.tenant_id = ? AND c.worktree_id = ?
		ORDER BY c.created_at DESC
		LIMIT 1`
	return r.scanConnectionRow(ctx, q, tenantID, worktreeID)
}

// scanConnectionRow factors out ResolveConnection/ResolveConnectionByDevServer/
// ResolveConnectionByWorktree's shared row-scan + "no rows means
// connected=false, not an error" handling.
func (r *Repository) scanConnectionRow(ctx context.Context, query string, args ...any) (bool, domain.DevServer, domain.Connection, error) {
	row := r.db.QueryRowContext(ctx, query, args...)

	var conn domain.Connection
	var ds domain.DevServer
	var mode string
	err := row.Scan(
		&conn.ID, &conn.TenantID, &conn.DevServerID, &conn.RepoPath, &conn.WorktreeID,
		&ds.ID, &ds.TenantID, &ds.Host, &mode,
	)
	if errors.Is(err, sql.ErrNoRows) {
		// No dev server owns this connectionId within this tenant — not an
		// error, the caller's cue to execute locally.
		return false, domain.DevServer{}, domain.Connection{}, nil
	}
	if err != nil {
		return false, domain.DevServer{}, domain.Connection{}, fmt.Errorf("mysql: resolve connection: %w", err)
	}
	ds.Mode = domain.ConnectionMode(mode)
	return true, ds, conn, nil
}

// CreateConnection inserts a new connection binding and returns the
// persisted row — the write side ResolveConnection's join reads from.
func (r *Repository) CreateConnection(ctx context.Context, conn domain.Connection) (domain.Connection, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO connections (id, tenant_id, dev_server_id, repo_path, worktree_id, status, last_activity_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, conn.ID, conn.TenantID, conn.DevServerID, conn.RepoPath, conn.WorktreeID, conn.Status, conn.LastActivityAt)
	if err != nil {
		return domain.Connection{}, fmt.Errorf("mysql: insert connection: %w", err)
	}
	return conn, nil
}

// CreateConnectionWithOutbox inserts a new connection binding and enqueues
// event as an outbox_events row — both in ONE transaction (Epic G's
// transactional-outbox pattern), using database/sql's BeginTx/Commit/
// Rollback the way credential-broker-service's RunInTx translation
// (BE-DB-SOL-006 §3) established for this rollout, since pgx.Pool.Begin
// has no database/sql equivalent method name.
func (r *Repository) CreateConnectionWithOutbox(ctx context.Context, conn domain.Connection, event domain.OutboxEvent) (domain.Connection, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Connection{}, fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO connections (id, tenant_id, dev_server_id, repo_path, worktree_id, status, last_activity_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, conn.ID, conn.TenantID, conn.DevServerID, conn.RepoPath, conn.WorktreeID, conn.Status, conn.LastActivityAt); err != nil {
		return domain.Connection{}, fmt.Errorf("mysql: insert connection: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO outbox_events (id, tenant_id, subject, occurred_at, version, payload)
		VALUES (?, ?, ?, ?, 1, ?)
	`, event.ID, conn.TenantID, event.Subject, event.OccurredAt, event.PayloadJSON); err != nil {
		return domain.Connection{}, fmt.Errorf("mysql: insert outbox event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return domain.Connection{}, fmt.Errorf("mysql: commit tx: %w", err)
	}
	return conn, nil
}

// GetActiveByDevServer returns the most recent connection for devServerID
// whose status is not "closed".
func (r *Repository) GetActiveByDevServer(ctx context.Context, tenantID, devServerID string) (domain.Connection, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, dev_server_id, repo_path, worktree_id, status, last_activity_at
		FROM connections
		WHERE tenant_id = ? AND dev_server_id = ? AND status <> 'closed'
		ORDER BY created_at DESC
		LIMIT 1
	`, tenantID, devServerID)

	var conn domain.Connection
	err := row.Scan(&conn.ID, &conn.TenantID, &conn.DevServerID, &conn.RepoPath, &conn.WorktreeID, &conn.Status, &conn.LastActivityAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Connection{}, false, nil
	}
	if err != nil {
		return domain.Connection{}, false, fmt.Errorf("mysql: get active connection by dev server: %w", err)
	}
	return conn, true, nil
}

// UpdateStatus persists a connection's Status/DegradedSince after a domain
// state machine transition (BE-SOL-STORAGE-003 §2, TASK-BE-STORAGE-009).
// Unlike AgentTokenStore.Revoke/TerminalSessionStore.Touch, a retried
// transition CAN legitimately resubmit identical Status/DegradedSince
// values (BE-DB-SOL-005 §3.1's pitfall) — so not-found is decided by a
// follow-up existence SELECT, never by RowsAffected().
func (r *Repository) UpdateStatus(ctx context.Context, tenantID string, conn domain.Connection) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE connections
		SET status = ?, degraded_since = ?
		WHERE id = ? AND tenant_id = ?
	`, conn.Status, conn.DegradedSince, conn.ID, tenantID)
	if err != nil {
		return fmt.Errorf("mysql: update connection status: %w", err)
	}
	var exists int
	err = r.db.QueryRowContext(ctx, `SELECT 1 FROM connections WHERE id = ? AND tenant_id = ?`, conn.ID, tenantID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("mysql: update connection status: no connection %s for tenant %s", conn.ID, tenantID)
	}
	if err != nil {
		return fmt.Errorf("mysql: confirming connection status update: %w", err)
	}
	return nil
}

// ListConnectivitySummary returns every connection scoped to tenantID,
// joined through dev_servers as defense-in-depth tenant scoping — backs
// GetFleetConnectivitySummary (TASK-BE-STORAGE-006, CR-STORAGE-007).
func (r *Repository) ListConnectivitySummary(ctx context.Context, tenantID string) ([]domain.Connection, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.tenant_id, c.dev_server_id, c.repo_path, c.worktree_id, c.status,
		       c.last_activity_at, c.degraded_since, c.grace_period_seconds
		FROM connections c
		JOIN dev_servers d ON d.id = c.dev_server_id AND d.tenant_id = c.tenant_id
		WHERE c.tenant_id = ?
		ORDER BY c.created_at DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("mysql: list connectivity summary: %w", err)
	}
	defer rows.Close()

	var out []domain.Connection
	for rows.Next() {
		var conn domain.Connection
		if err := rows.Scan(&conn.ID, &conn.TenantID, &conn.DevServerID, &conn.RepoPath, &conn.WorktreeID,
			&conn.Status, &conn.LastActivityAt, &conn.DegradedSince, &conn.GracePeriodSeconds); err != nil {
			return nil, fmt.Errorf("mysql: scan connectivity summary row: %w", err)
		}
		out = append(out, conn)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate connectivity summary rows: %w", err)
	}
	return out, nil
}

// FindBySshTarget returns the DevServer bound to sshTargetID, if any.
func (r *Repository) FindBySshTarget(ctx context.Context, tenantID, sshTargetID string) (domain.DevServer, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, host, connection_mode, ssh_target_id, approval_status, group_id, kind
		FROM dev_servers
		WHERE tenant_id = ? AND ssh_target_id = ?
		LIMIT 1
	`, tenantID, sshTargetID)

	var ds domain.DevServer
	var mode, status, kind string
	var groupID sql.NullString
	err := row.Scan(&ds.ID, &ds.TenantID, &ds.Host, &mode, &ds.SSHTargetID, &status, &groupID, &kind)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DevServer{}, false, nil
	}
	if err != nil {
		return domain.DevServer{}, false, fmt.Errorf("mysql: find dev server by ssh target: %w", err)
	}
	ds.Mode = domain.ConnectionMode(mode)
	ds.Status = domain.DevServerStatus(status)
	ds.Kind = domain.AgentKind(kind)
	if groupID.Valid {
		ds.GroupID = groupID.String
	}
	return ds, true, nil
}

// FindByHostAndMode resolves a dev server by (tenant, host, mode) — see
// internal/adapter/postgres.Repository.FindByHostAndMode's doc comment for
// the live nullable-scan bug this shape guards against (fixed the same way
// here from the start: ssh_target_id is always scanned through a nullable
// local var, never directly into ds.SSHTargetID).
func (r *Repository) FindByHostAndMode(ctx context.Context, tenantID, host string, mode domain.ConnectionMode) (domain.DevServer, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, host, connection_mode, ssh_target_id, approval_status, group_id, kind
		FROM dev_servers
		WHERE tenant_id = ? AND host = ? AND connection_mode = ?
		LIMIT 1
	`, tenantID, host, string(mode))

	var ds domain.DevServer
	var modeStr, status, kind string
	var sshTargetID, groupID sql.NullString
	err := row.Scan(&ds.ID, &ds.TenantID, &ds.Host, &modeStr, &sshTargetID, &status, &groupID, &kind)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DevServer{}, false, nil
	}
	if err != nil {
		return domain.DevServer{}, false, fmt.Errorf("mysql: find dev server by host and mode: %w", err)
	}
	if sshTargetID.Valid {
		ds.SSHTargetID = sshTargetID.String
	}
	ds.Mode = domain.ConnectionMode(modeStr)
	ds.Status = domain.DevServerStatus(status)
	ds.Kind = domain.AgentKind(kind)
	if groupID.Valid {
		ds.GroupID = groupID.String
	}
	return ds, true, nil
}

// UpdateProvisionResult persists the outcome of one provisioning attempt —
// see usecase.DevServerRepository.UpdateProvisionResult's doc comment.
func (r *Repository) UpdateProvisionResult(ctx context.Context, tenantID, id string, status domain.DevServerHealthStatus, info usecase.HandshakeInfo, provisionedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE dev_servers
		SET status = ?, platform = ?, arch = ?, node_version = ?, agent_version = ?, last_provisioned_at = ?
		WHERE tenant_id = ? AND id = ?
	`, string(status), info.Platform, info.Arch, info.NodeVersion, info.AgentVersion, provisionedAt, tenantID, id)
	if err != nil {
		return fmt.Errorf("mysql: update dev server provision result: %w", err)
	}
	return nil
}

// GetFleetHealth returns the latest fleet-health sample per dev server for
// tenantID, joined through dev_servers since fleet_health has no tenant_id
// column of its own.
func (r *Repository) GetFleetHealth(ctx context.Context, tenantID string) ([]domain.DevServerHealth, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT fh.dev_server_id, fh.reachable, fh.cpu_percent, fh.ram_percent, fh.disk_percent, fh.latency_ms
		FROM fleet_health fh
		JOIN dev_servers ds ON ds.id = fh.dev_server_id
		WHERE ds.tenant_id = ?
		ORDER BY fh.checked_at DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query fleet health: %w", err)
	}
	defer rows.Close()

	var out []domain.DevServerHealth
	for rows.Next() {
		var h domain.DevServerHealth
		if err := rows.Scan(&h.DevServerID, &h.Reachable, &h.CPUPercent, &h.RAMPercent, &h.DiskPercent, &h.LatencyMS); err != nil {
			return nil, fmt.Errorf("mysql: scan fleet health row: %w", err)
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate fleet health rows: %w", err)
	}
	return out, nil
}

// ListAllDevServers returns every registered dev server across every
// tenant — deliberately unscoped, the fleet-health poller's internal
// background process has no request-scoped tenant to join through.
func (r *Repository) ListAllDevServers(ctx context.Context) ([]domain.DevServer, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, tenant_id, host, connection_mode, ssh_target_id, approval_status, group_id, kind
		FROM dev_servers
	`)
	if err != nil {
		return nil, fmt.Errorf("mysql: query all dev servers: %w", err)
	}
	defer rows.Close()

	var out []domain.DevServer
	for rows.Next() {
		var ds domain.DevServer
		var mode, status, kind string
		var sshTargetID, groupID sql.NullString
		if err := rows.Scan(&ds.ID, &ds.TenantID, &ds.Host, &mode, &sshTargetID, &status, &groupID, &kind); err != nil {
			return nil, fmt.Errorf("mysql: scan dev server row: %w", err)
		}
		ds.Mode = domain.ConnectionMode(mode)
		ds.Status = domain.DevServerStatus(status)
		ds.Kind = domain.AgentKind(kind)
		if sshTargetID.Valid {
			ds.SSHTargetID = sshTargetID.String
		}
		if groupID.Valid {
			ds.GroupID = groupID.String
		}
		out = append(out, ds)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate all dev server rows: %w", err)
	}
	return out, nil
}

// UpsertFleetHealth implements usecase.FleetHealthWriter.UpsertFleetHealth —
// dev_server_id is fleet_health's primary key, so this is a plain
// upsert-by-PK, one row per dev server, latest sample wins.
func (r *Repository) UpsertFleetHealth(ctx context.Context, h domain.DevServerHealth) error {
	const query = `
		INSERT INTO fleet_health (dev_server_id, reachable, cpu_percent, ram_percent, disk_percent, latency_ms, status, checked_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP(6))
		ON DUPLICATE KEY UPDATE
		  reachable = VALUES(reachable), cpu_percent = VALUES(cpu_percent),
		  ram_percent = VALUES(ram_percent), disk_percent = VALUES(disk_percent),
		  latency_ms = VALUES(latency_ms), status = VALUES(status), checked_at = CURRENT_TIMESTAMP(6)`
	_, err := r.db.ExecContext(ctx, query, h.DevServerID, h.Reachable, h.CPUPercent, h.RAMPercent, h.DiskPercent, h.LatencyMS, string(h.Status))
	if err != nil {
		return fmt.Errorf("mysql: upsert fleet health: %w", err)
	}
	return nil
}

// PortForwardStore is domain.PortForward's storage — same
// own-Go-value-not-the-same-as-Repository shape as SshTargetStore, sharing
// the same *sql.DB.
type PortForwardStore struct {
	db *sql.DB
}

func NewPortForwardStore(db *sql.DB) *PortForwardStore {
	return &PortForwardStore{db: db}
}

// Create inserts a new port forward and returns the persisted row.
func (s *PortForwardStore) Create(ctx context.Context, pf domain.PortForward) (domain.PortForward, error) {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO port_forwards (id, tenant_id, connection_id, local_port, remote_port, process_name, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, pf.ID, pf.TenantID, pf.ConnectionID, pf.LocalPort, pf.RemotePort, pf.ProcessName, string(pf.Status))
	if err != nil {
		return domain.PortForward{}, fmt.Errorf("mysql: insert port forward: %w", err)
	}
	return pf, nil
}

// UpdateStatus sets id's status column — PollWorkspacePorts' teardown step
// (BR-SSH-18) writes "closed" here rather than deleting the row, keeping a
// history of forwards for the connection's lifetime.
func (s *PortForwardStore) UpdateStatus(ctx context.Context, tenantID, id string, status domain.PortForwardStatus) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE port_forwards
		SET status = ?
		WHERE tenant_id = ? AND id = ?
	`, string(status), tenantID, id)
	if err != nil {
		return fmt.Errorf("mysql: update port forward status: %w", err)
	}
	return nil
}

// GetDevServerHealth implements usecase.FleetHealthWriter.GetDevServerHealth
// — devServerID's current (about-to-be-overwritten) sample, PollFleetHealth's
// transition-detection read called BEFORE UpsertFleetHealth. found=false
// (not an error) when no sample exists yet.
func (r *Repository) GetDevServerHealth(ctx context.Context, devServerID string) (domain.DevServerHealth, bool, error) {
	const query = `SELECT dev_server_id, reachable, cpu_percent, ram_percent, disk_percent, latency_ms, status
		FROM fleet_health WHERE dev_server_id = ?`
	var h domain.DevServerHealth
	var status string
	err := r.db.QueryRowContext(ctx, query, devServerID).Scan(&h.DevServerID, &h.Reachable, &h.CPUPercent, &h.RAMPercent, &h.DiskPercent, &h.LatencyMS, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DevServerHealth{}, false, nil
	}
	if err != nil {
		return domain.DevServerHealth{}, false, fmt.Errorf("mysql: get dev server health: %w", err)
	}
	h.Status = domain.HealthStatus(status)
	return h, true, nil
}

// TryLock implements usecase.PollLockPort.TryLock via a MySQL named
// user-level lock (GET_LOCK/RELEASE_LOCK) — the MySQL counterpart to
// Postgres's session-level advisory lock (pg_try_advisory_lock/
// pg_advisory_unlock). Non-blocking: GET_LOCK(name, 0) returns immediately
// (1 = acquired, 0 = held by someone else, NULL = error), so a replica that
// loses the race skips this server this tick rather than queueing — same
// semantics as the Postgres variant. No hashtext() needed: MySQL's
// GET_LOCK takes the name string directly (devServerID, a UUID, is well
// under its 64-character limit). Uses a single reserved connection
// (sql.Conn), not a bare db.QueryRowContext, since these locks are
// session-scoped: acquire and release must run on literally the same
// underlying connection.
func (r *Repository) TryLock(ctx context.Context, devServerID string) (bool, func(), error) {
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return false, nil, fmt.Errorf("mysql: acquire connection for lock: %w", err)
	}
	var locked sql.NullInt64
	if err := conn.QueryRowContext(ctx, `SELECT GET_LOCK(?, 0)`, devServerID).Scan(&locked); err != nil {
		_ = conn.Close()
		return false, nil, fmt.Errorf("mysql: try lock: %w", err)
	}
	if !locked.Valid || locked.Int64 != 1 {
		_ = conn.Close()
		return false, nil, nil
	}
	unlock := func() {
		// Why context.Background(): unlock typically runs in a defer after
		// the caller's ctx may already be done (poll tick finished) —
		// releasing the lock must not be skipped just because the tick's
		// own context expired.
		_, _ = conn.ExecContext(context.Background(), `SELECT RELEASE_LOCK(?)`, devServerID)
		_ = conn.Close()
	}
	return true, unlock, nil
}

// ListActiveByConnection returns every non-closed port forward for
// connectionID.
func (s *PortForwardStore) ListActiveByConnection(ctx context.Context, tenantID, connectionID string) ([]domain.PortForward, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, connection_id, local_port, remote_port, process_name, status
		FROM port_forwards
		WHERE tenant_id = ? AND connection_id = ? AND status <> 'closed'
		ORDER BY created_at DESC
	`, tenantID, connectionID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query port forwards: %w", err)
	}
	defer rows.Close()

	var out []domain.PortForward
	for rows.Next() {
		var pf domain.PortForward
		var status string
		if err := rows.Scan(&pf.ID, &pf.TenantID, &pf.ConnectionID, &pf.LocalPort, &pf.RemotePort, &pf.ProcessName, &status); err != nil {
			return nil, fmt.Errorf("mysql: scan port forward row: %w", err)
		}
		pf.Status = domain.PortForwardStatus(status)
		out = append(out, pf)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate port forward rows: %w", err)
	}
	return out, nil
}

// InsertOutboxEvent enqueues one row for common/outbox.Relay to publish —
// implements usecase.OutboxWriter against outbox_events (migrations/mysql/0025).
func (r *Repository) InsertOutboxEvent(ctx context.Context, event domain.OutboxEvent) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO outbox_events (id, tenant_id, subject, occurred_at, version, payload)
		VALUES (?, ?, ?, ?, 1, ?)
	`, event.ID, event.TenantID, event.Subject, event.OccurredAt, event.PayloadJSON)
	if err != nil {
		return fmt.Errorf("mysql: insert outbox event: %w", err)
	}
	return nil
}
