# TASK-163: Extend `infra-fleet-service`'s ports and Postgres adapters for `ssh.*`

**From Solution:** SOL-024
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `services/infra-fleet-service/internal/usecase/ports.go`, `services/infra-fleet-service/internal/domain/{connection,dev_server}.go`, `services/infra-fleet-service/internal/adapter/postgres/repository.go`, `services/infra-fleet-service/migrations/0004_connection_status.up.sql` (new)
**Depends on:** TASK-162 (grounds this task's message shapes)
**Status:** `[ ]` TODO

---

## Context

`SshTargetRepository`/`ConnectionRepository`/`DevServerRepository`
(`ports.go:20-42`) are read/write-lopsided today — this task adds the
missing read methods TASK-164's usecases need. `domain.Connection`
currently has no `Status`/`LastActivityAt` field
(`internal/domain/connection.go`) and `infra.connections` has no matching
column (`migrations/0002_connections.up.sql`) — both need adding.
`infra.dev_servers.ssh_target_id` already exists (`migrations/0003_dev_server_ssh_target.up.sql`),
so `FindBySshTarget` is a straightforward query; `FindOrCreateForSshTarget`
is new write logic.

## Changes to make

### `internal/domain/connection.go` — add `Status`/`LastActivityAt`

```go
// Connection binds a worktree/repo path to the DevServer that owns it...
type Connection struct {
	ID             string
	TenantID       string
	DevServerID    string
	RepoPath       string
	WorktreeID     string
	Status         string     // "established" | "degraded" | "closed" — empty for connections predating this field (worktree-bound connections created via CreateConnection, not EstablishConnection)
	LastActivityAt *time.Time // nil when never recorded
}
```

Add `"time"` to this file's imports. `NewConnection`'s existing
constructor signature/invariants are unchanged — `Status`/`LastActivityAt`
are set directly by callers that need them (`EstablishConnection`,
TASK-164), not validated as required fields, since `CreateConnection`'s
existing worktree-binding callers don't set them.

### New migration `migrations/0004_connection_status.up.sql`

```sql
-- Adds the columns EstablishConnection (ssh.connect, TASK-164) needs to
-- record a connection's live handshake state — infra.connections
-- previously only tracked the static worktree/dev-server binding written
-- by CreateConnection. See specs/backend-go/bugs/missing-v1/BUG-024.
ALTER TABLE infra.connections
    ADD COLUMN status TEXT NOT NULL DEFAULT '',
    ADD COLUMN last_activity_at TIMESTAMPTZ;
```

And the matching `migrations/0004_connection_status.down.sql`:

```sql
ALTER TABLE infra.connections
    DROP COLUMN status,
    DROP COLUMN last_activity_at;
```

### `internal/usecase/ports.go` — extend the three ports

```go
type SshTargetRepository interface {
	Create(ctx context.Context, target domain.SshTarget) (domain.SshTarget, error)
	Get(ctx context.Context, tenantID, id string) (domain.SshTarget, error)
	// List returns every SSH target registered for tenantID — backs
	// ssh.listTargets/ssh.getUserAccount (TASK-164).
	List(ctx context.Context, tenantID string) ([]domain.SshTarget, error)
}

type ConnectionRepository interface {
	CreateConnection(ctx context.Context, conn domain.Connection) (domain.Connection, error)
	// GetActiveByDevServer returns the most recent non-closed connection
	// bound to devServerID, if any — backs ssh.getState's local read
	// (TASK-164). found=false, err=nil means "no active connection", not
	// an error.
	GetActiveByDevServer(ctx context.Context, tenantID, devServerID string) (conn domain.Connection, found bool, err error)
}

type DevServerRepository interface {
	Register(ctx context.Context, devServer domain.DevServer) (domain.DevServer, error)
	Get(ctx context.Context, tenantID, id string) (domain.DevServer, error)
	List(ctx context.Context, tenantID string) ([]domain.DevServer, error)
	// FindBySshTarget returns the DevServer whose ssh_target_id matches
	// sshTargetID, if one has been registered yet. found=false, err=nil
	// means "no dev server bound to this SSH target yet" — the caller
	// (TASK-164's EstablishConnection) is responsible for constructing and
	// Register()-ing a new relay-ssh-mode DevServer in that case,
	// generating its ID with uuid.NewString() the same way
	// register_dev_server.go's usecase already does — ID generation stays
	// in the usecase layer, not this adapter, matching every other `New*`
	// call site in this service.
	FindBySshTarget(ctx context.Context, tenantID, sshTargetID string) (ds domain.DevServer, found bool, err error)
}
```

### `internal/adapter/postgres/repository.go` / the `SshTargetStore` type — implement the new methods

```go
// List returns every SSH target registered for tenantID.
func (s *SshTargetStore) List(ctx context.Context, tenantID string) ([]domain.SshTarget, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, host, user_name, vault_ssh_role
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
		if err := rows.Scan(&t.ID, &t.TenantID, &t.Host, &t.UserName, &t.VaultSSHRole); err != nil {
			return nil, fmt.Errorf("postgres: scan ssh target row: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate ssh target rows: %w", err)
	}
	return out, nil
}
```

```go
// GetActiveByDevServer returns the most recent connection for devServerID
// whose status is not "closed" — see Connection.Status's doc comment
// (TASK-163's migration).
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
```

```go
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
```

No `FindOrCreateForSshTarget` adapter method — the find-or-create
decision and new-`DevServer` construction (with `uuid.NewString()`) live
in TASK-164's `EstablishConnection` usecase, calling `FindBySshTarget`
then `Register` directly, matching how `register_dev_server.go`'s usecase
already generates IDs at the usecase layer, not in `postgres/`.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/infra-fleet-service
go build ./... && go vet ./...
```

Run the new migration against a local dev database per this service's
existing migration-apply instructions (check `README.md` for the exact
`migrate`/`goose`/`atlas` invocation this repo uses) before TASK-164's
tests depend on the new columns.
