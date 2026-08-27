# TASK-TM-03-07: Add Postgres adapter + gRPC server handlers for scrollback RPCs

**From Solution:** SOL-TM-03
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/postgres/terminal_scrollback_snapshot_repository.go`
**Depends on:** TASK-TM-03-01 (schema), TASK-TM-03-03 (proto), TASK-TM-03-04 (port), TASK-TM-03-05 (Save usecase), TASK-TM-03-06 (Get/Delete/Expire usecases)
**Status:** `[ ]` TODO

---

## Context

Wires the usecases from TASK-TM-03-05/06 to a real Postgres-backed
implementation of `TerminalScrollbackSnapshotRepository`, and exposes the
three new RPCs (TASK-TM-03-03) on `infra-fleet-service`'s gRPC server —
same pattern as `TerminalSessionStore`/`Server.SpawnTerminalSession`
already use for `infra.terminal_sessions`.

## Changes to make

### 1. Postgres adapter

Create `backend-go/services/infra-fleet-service/internal/adapter/postgres/terminal_scrollback_snapshot_repository.go`:

```go
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

// TerminalScrollbackSnapshotStore implements usecase.TerminalScrollbackSnapshotRepository
// against infra.terminal_scrollback_snapshots (migrations/0009).
type TerminalScrollbackSnapshotStore struct {
	pool *pgxpool.Pool
}

func NewTerminalScrollbackSnapshotStore(pool *pgxpool.Pool) *TerminalScrollbackSnapshotStore {
	return &TerminalScrollbackSnapshotStore{pool: pool}
}

func (s *TerminalScrollbackSnapshotStore) Upsert(ctx context.Context, snap domain.TerminalScrollbackSnapshot) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO infra.terminal_scrollback_snapshots
			(tenant_id, worktree_id, pane_key, cols, rows, data_gzip, uncompressed_bytes, last_title, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (tenant_id, worktree_id, pane_key) DO UPDATE SET
			cols = EXCLUDED.cols, rows = EXCLUDED.rows, data_gzip = EXCLUDED.data_gzip,
			uncompressed_bytes = EXCLUDED.uncompressed_bytes, last_title = EXCLUDED.last_title,
			updated_at = EXCLUDED.updated_at
	`, snap.TenantID, snap.WorktreeID, snap.PaneKey, snap.Cols, snap.Rows,
		snap.DataGzip, snap.UncompressedBytes, snap.LastTitle, snap.UpdatedAt)
	if err != nil {
		return fmt.Errorf("postgres: upsert terminal scrollback snapshot: %w", err)
	}
	return nil
}

func (s *TerminalScrollbackSnapshotStore) Get(ctx context.Context, tenantID, worktreeID, paneKey string) (bool, domain.TerminalScrollbackSnapshot, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT tenant_id, worktree_id, pane_key, cols, rows, data_gzip, uncompressed_bytes, last_title, updated_at
		FROM infra.terminal_scrollback_snapshots
		WHERE tenant_id = $1 AND worktree_id = $2 AND pane_key = $3
	`, tenantID, worktreeID, paneKey)

	var snap domain.TerminalScrollbackSnapshot
	err := row.Scan(&snap.TenantID, &snap.WorktreeID, &snap.PaneKey, &snap.Cols, &snap.Rows,
		&snap.DataGzip, &snap.UncompressedBytes, &snap.LastTitle, &snap.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, domain.TerminalScrollbackSnapshot{}, nil
	}
	if err != nil {
		return false, domain.TerminalScrollbackSnapshot{}, fmt.Errorf("postgres: query terminal scrollback snapshot: %w", err)
	}
	return true, snap, nil
}

// SumUncompressedBytes excludes excludePaneKey (the row Upsert is about to
// replace) so two saves to the same pane never double-count toward BR-TM-10's cap.
func (s *TerminalScrollbackSnapshotStore) SumUncompressedBytes(ctx context.Context, tenantID, worktreeID, excludePaneKey string) (int64, error) {
	var total int64
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(uncompressed_bytes), 0) FROM infra.terminal_scrollback_snapshots
		WHERE tenant_id = $1 AND worktree_id = $2 AND pane_key != $3
	`, tenantID, worktreeID, excludePaneKey).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("postgres: sum terminal scrollback snapshot bytes: %w", err)
	}
	return total, nil
}

func (s *TerminalScrollbackSnapshotStore) DeleteByWorktree(ctx context.Context, tenantID, worktreeID string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM infra.terminal_scrollback_snapshots WHERE tenant_id = $1 AND worktree_id = $2
	`, tenantID, worktreeID)
	if err != nil {
		return fmt.Errorf("postgres: delete terminal scrollback snapshots by worktree: %w", err)
	}
	return nil
}

func (s *TerminalScrollbackSnapshotStore) DeleteExpired(ctx context.Context, olderThan time.Time) (int, error) {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM infra.terminal_scrollback_snapshots WHERE updated_at < $1
	`, olderThan)
	if err != nil {
		return 0, fmt.Errorf("postgres: delete expired terminal scrollback snapshots: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
```

### 2. gRPC server handlers

In `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go`:
add three fields to `Server` (`saveTerminalScrollbackSnapshot *usecase.SaveTerminalScrollbackSnapshot`,
`getTerminalScrollbackSnapshot *usecase.GetTerminalScrollbackSnapshot`,
`deleteTerminalScrollbackSnapshots *usecase.DeleteTerminalScrollbackSnapshots`),
add matching params to `New(...)` and its return-struct assignment (follow
the existing positional-param pattern this constructor already uses for
every other usecase), and add the three handler methods:

```go
func (s *Server) SaveTerminalScrollbackSnapshot(ctx context.Context, req *infrafleetv1.SaveTerminalScrollbackSnapshotRequest) (*emptypb.Empty, error) {
	err := s.saveTerminalScrollbackSnapshot.Execute(ctx, usecase.SaveTerminalScrollbackSnapshotInput{
		WorktreeID: req.GetWorktreeId(), PaneKey: req.GetPaneKey(),
		Cols: req.GetCols(), Rows: req.GetRows(), Data: req.GetData(), LastTitle: req.GetLastTitle(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) GetTerminalScrollbackSnapshot(ctx context.Context, req *infrafleetv1.GetTerminalScrollbackSnapshotRequest) (*infrafleetv1.GetTerminalScrollbackSnapshotResponse, error) {
	result, err := s.getTerminalScrollbackSnapshot.Execute(ctx, req.GetWorktreeId(), req.GetPaneKey())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.GetTerminalScrollbackSnapshotResponse{
		Found: result.Found, Cols: result.Cols, Rows: result.Rows, Data: result.Data,
		LastTitle: result.LastTitle, UpdatedAtUnixMs: result.UpdatedAt.UnixMilli(),
	}, nil
}

func (s *Server) DeleteTerminalScrollbackSnapshots(ctx context.Context, req *infrafleetv1.DeleteTerminalScrollbackSnapshotsRequest) (*emptypb.Empty, error) {
	if err := s.deleteTerminalScrollbackSnapshots.Execute(ctx, req.GetWorktreeId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}
```

### 3. Wire construction in `cmd/server/main.go`

Add, near the other usecase constructions:

```go
scrollbackStore := postgres.NewTerminalScrollbackSnapshotStore(pool)
saveTerminalScrollbackSnapshotUC := usecase.NewSaveTerminalScrollbackSnapshot(scrollbackStore, usecase.RealClock{})
getTerminalScrollbackSnapshotUC := usecase.NewGetTerminalScrollbackSnapshot(scrollbackStore)
deleteTerminalScrollbackSnapshotsUC := usecase.NewDeleteTerminalScrollbackSnapshots(scrollbackStore)
```

and append the three new usecase variables as trailing args to the
`infragrpc.New(...)` call, matching the order added to `Server`'s
constructor in step 2.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
```

Add `internal/adapter/postgres/terminal_scrollback_snapshot_repository_test.go`
using this service's existing `testcontainers-go` Postgres test harness
(see `repository_test.go` for the setup pattern):
- `SumUncompressedBytes` excludes the pane being replaced (two saves to the
  same pane never double-count toward BR-TM-10's cap)
- RLS policy blocks a cross-tenant `Get` (query with a different
  `app.tenant_id` session var returns `found=false`, not the other
  tenant's row)
- `DeleteExpired` respects the `updated_at` cutoff

```bash
go test ./services/infra-fleet-service/internal/adapter/postgres/... -run TestTerminalScrollbackSnapshot -v
go test ./services/infra-fleet-service/internal/adapter/grpc/... -v
```

Expected: clean build, all tests pass (requires Docker for
`testcontainers-go`).
