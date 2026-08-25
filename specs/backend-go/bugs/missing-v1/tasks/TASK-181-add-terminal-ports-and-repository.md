# TASK-181: Add `TerminalSessionRepository`/`DevServerAgentClient` PTY ports + Postgres repository

**From Solution:** SOL-029
**Priority:** P0 — every terminal usecase depends on these ports
**Service:** `infra-fleet-service`
**File:** `internal/usecase/ports.go`, `internal/adapter/postgres/terminal_session_repository.go` (new)
**Depends on:** TASK-180
**Status:** `[ ]` TODO

---

## Context

This task adds the persistence port for `terminal_sessions` bookkeeping and
extends `DevServerAgentClient` with PTY-specific methods, alongside the
existing `Exec`/`Health` (`internal/usecase/ports.go:89-97`). Per this
package's doc comment, `DevServerAgentClient` is defined in `usecase/`
(consumer-side), implemented in `adapter/devserveragent` (TASK-183) — this
task only adds the port's method signatures, not their implementation.

## Changes to make

### `internal/usecase/ports.go`

Add after the existing `FleetHealthPort` interface (before
`DevServerAgentClient`):

```go
// TerminalSessionRepository is the persistence port for terminal_sessions
// bookkeeping (infra-fleet-service.md §4's TerminalSession entity — "Holds
// no PTY bytes"). Implemented by internal/adapter/postgres.
type TerminalSessionRepository interface {
	Create(ctx context.Context, s domain.TerminalSession) error
	Get(ctx context.Context, tenantID, ptyID string) (domain.TerminalSession, bool, error)
	ListByConnection(ctx context.Context, tenantID, connectionID string) ([]domain.TerminalSession, error)
	Touch(ctx context.Context, ptyID string, at time.Time) error
	MarkClosed(ctx context.Context, ptyID string, at time.Time) error
}

// PtyEvent is what DevServerAgentClient.StreamPty yields — output bytes or
// a terminal exit, tagged by PtyID so one Client can multiplex many
// sessions over one underlying agent connection.
type PtyEvent struct {
	PtyID    string
	Output   []byte // nil when Exited is set
	Exited   bool
	ExitCode int32
}
```

Add `"time"` to this file's import block if not already present.

Extend the existing `DevServerAgentClient` interface — find:

```go
type DevServerAgentClient interface {
	// Exec dispatches one JSON-RPC method call (e.g. "ports.scan",
	// "pty.spawn", "preflight.check") to the agent over devServer's resolved
	// transport mode and returns its decoded result.
	Exec(ctx context.Context, devServer domain.DevServer, method string, params map[string]any) (map[string]any, error)
	// Health performs an agent-level reachability/handshake check, distinct
	// from the SSH-exec-based fleet health poll that GetFleetHealth reads.
	Health(ctx context.Context, devServer domain.DevServer) (bool, error)
}
```

Replace with:

```go
type DevServerAgentClient interface {
	// Exec dispatches one JSON-RPC method call (e.g. "ports.scan",
	// "pty.spawn", "preflight.check") to the agent over devServer's resolved
	// transport mode and returns its decoded result.
	Exec(ctx context.Context, devServer domain.DevServer, method string, params map[string]any) (map[string]any, error)
	// Health performs an agent-level reachability/handshake check, distinct
	// from the SSH-exec-based fleet health poll that GetFleetHealth reads.
	Health(ctx context.Context, devServer domain.DevServer) (bool, error)

	// SpawnPty/WritePty/ResizePty/KillPty are typed wrappers over the same
	// JSON-RPC transport Exec uses (see adapter/devserveragent/methods.go,
	// TASK-183) — not a second protocol.
	SpawnPty(ctx context.Context, devServer domain.DevServer, cwd, shell string, cols, rows int32) (ptyID string, err error)
	WritePty(ctx context.Context, devServer domain.DevServer, ptyID string, data []byte) error
	ResizePty(ctx context.Context, devServer domain.DevServer, ptyID string, cols, rows int32) error
	KillPty(ctx context.Context, devServer domain.DevServer, ptyID string) error
	// StreamPty subscribes to output/exit notifications for one ptyId. The
	// returned channel closes when ctx is cancelled or the underlying agent
	// session drops — the usecase layer, not this port, decides whether
	// that's a client-visible error.
	StreamPty(ctx context.Context, devServer domain.DevServer, ptyID string) (<-chan PtyEvent, error)
	AgentStatus(ctx context.Context, devServer domain.DevServer, ptyID string) (running bool, kind string, ready bool, err error)
	InspectProcess(ctx context.Context, devServer domain.DevServer, ptyID string) (known bool, pid int32, command, cwd string, err error)
}
```

## New file `internal/adapter/postgres/terminal_session_repository.go`

Follows this package's existing `repository.go` style (hand-written SQL via
`pgxpool.Pool`, `pgx.ErrNoRows` → `(zero, false, nil)` for `Get`):

```go
// Package postgres — terminal_session_repository.go implements
// usecase.TerminalSessionRepository against terminal_sessions
// (infra-fleet-service.md §5).
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// TerminalSessionRepository implements usecase.TerminalSessionRepository.
type TerminalSessionRepository struct {
	pool *pgxpool.Pool
}

func NewTerminalSessionRepository(pool *pgxpool.Pool) *TerminalSessionRepository {
	return &TerminalSessionRepository{pool: pool}
}

func (r *TerminalSessionRepository) Create(ctx context.Context, s domain.TerminalSession) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO terminal_sessions (pty_id, connection_id, cwd, created_at, last_active_at)
		VALUES ($1, NULLIF($2, ''), $3, $4, $5)
	`, s.PtyID, s.ConnectionID, s.Cwd, s.CreatedAt, s.LastActiveAt)
	if err != nil {
		return fmt.Errorf("postgres: insert terminal session: %w", err)
	}
	return nil
}

// Get is deliberately NOT joined on a tenant_id column — terminal_sessions
// has no such column (it is keyed by connection_id, which infra-fleet's
// existing ConnectionResolver already tenant-scopes at resolution time).
// tenantID is accepted for interface symmetry with every other
// tenant-scoped Repository port in this service and reserved for a future
// tenant_id column — confirm terminal_sessions' actual migrated schema
// before removing this parameter or wiring a real WHERE tenant_id clause.
func (r *TerminalSessionRepository) Get(ctx context.Context, tenantID, ptyID string) (domain.TerminalSession, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT pty_id, COALESCE(connection_id, ''), cwd, created_at, last_active_at, closed_at
		FROM terminal_sessions WHERE pty_id = $1
	`, ptyID)

	var s domain.TerminalSession
	if err := row.Scan(&s.PtyID, &s.ConnectionID, &s.Cwd, &s.CreatedAt, &s.LastActiveAt, &s.ClosedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.TerminalSession{}, false, nil
		}
		return domain.TerminalSession{}, false, fmt.Errorf("postgres: query terminal session: %w", err)
	}
	return s, true, nil
}

func (r *TerminalSessionRepository) ListByConnection(ctx context.Context, tenantID, connectionID string) ([]domain.TerminalSession, error) {
	query := `SELECT pty_id, COALESCE(connection_id, ''), cwd, created_at, last_active_at, closed_at FROM terminal_sessions WHERE closed_at IS NULL`
	args := []any{}
	if connectionID != "" {
		query += ` AND connection_id = $1`
		args = append(args, connectionID)
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: query terminal sessions: %w", err)
	}
	defer rows.Close()

	var out []domain.TerminalSession
	for rows.Next() {
		var s domain.TerminalSession
		if err := rows.Scan(&s.PtyID, &s.ConnectionID, &s.Cwd, &s.CreatedAt, &s.LastActiveAt, &s.ClosedAt); err != nil {
			return nil, fmt.Errorf("postgres: scan terminal session row: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate terminal session rows: %w", err)
	}
	return out, nil
}

func (r *TerminalSessionRepository) Touch(ctx context.Context, ptyID string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE terminal_sessions SET last_active_at = $1 WHERE pty_id = $2`, at, ptyID)
	if err != nil {
		return fmt.Errorf("postgres: touch terminal session: %w", err)
	}
	return nil
}

func (r *TerminalSessionRepository) MarkClosed(ctx context.Context, ptyID string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE terminal_sessions SET closed_at = $1 WHERE pty_id = $2`, at, ptyID)
	if err != nil {
		return fmt.Errorf("postgres: mark terminal session closed: %w", err)
	}
	return nil
}
```

If `terminal_sessions` turns out to have a real `tenant_id` column once
TASK-180's migration check is resolved, add `AND tenant_id = $N` to `Get`/
`ListByConnection` and update this doc comment — the interface signature
already accepts `tenantID` for exactly this reason.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/infra-fleet-service
go build ./internal/usecase/... ./internal/adapter/postgres/...
```

Expected: `go build` will fail on any package implementing
`DevServerAgentClient` today (there are none yet — a fake would need
updating, but no adapter/devserveragent implementation exists until
TASK-183) — that is expected at this point; TASK-182/183 close it.
