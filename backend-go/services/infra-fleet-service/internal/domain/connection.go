package domain

import (
	"errors"
	"time"
)

var (
	// ErrEmptyConnectionTenant mirrors ErrEmptyDevServerTenant — a connection
	// with no owning tenant is never a valid domain state.
	ErrEmptyConnectionTenant = errors.New("domain: tenant_id is required")
	// ErrEmptyConnectionDevServer guards against a connection that doesn't
	// point at any dev server — the whole point of the entity.
	ErrEmptyConnectionDevServer = errors.New("domain: dev_server_id is required")
)

// Connection binds a worktree/repo path to the DevServer that owns it — the
// entity ResolveConnection resolves against (see migrations/0002_connections),
// replacing the scaffold's original connectionId==dev_server.id equation.
// See specs/backend-go/services/infra-fleet-service.md §5.
type Connection struct {
	ID          string
	TenantID    string
	DevServerID string
	RepoPath    string
	WorktreeID  string
	// Status/LastActivityAt are set by EstablishConnection (ssh.connect,
	// SOL-024/TASK-164) — empty/nil for connections predating this field
	// (worktree-bound connections created via CreateConnection, not
	// EstablishConnection). See migrations/0004_connection_status.
	Status         string // "established" | "degraded" | "closed"
	LastActivityAt *time.Time
}

// NewConnection constructs a Connection, enforcing the invariants a record
// must satisfy to be meaningful — RepoPath/WorktreeID are intentionally not
// required here: a connection can be registered before either is known and
// filled in later (mirrors DevServer's registration-then-detail pattern).
func NewConnection(id, tenantID, devServerID, repoPath, worktreeID string) (Connection, error) {
	if tenantID == "" {
		return Connection{}, ErrEmptyConnectionTenant
	}
	if devServerID == "" {
		return Connection{}, ErrEmptyConnectionDevServer
	}
	return Connection{ID: id, TenantID: tenantID, DevServerID: devServerID, RepoPath: repoPath, WorktreeID: worktreeID}, nil
}

// IsZero reports whether c is the zero-value Connection.
func (c Connection) IsZero() bool {
	return c == Connection{}
}
