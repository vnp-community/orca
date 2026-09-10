# TASK-TG-003-04: `RevokeGrant` / `ListGrants` public RPCs

**From Solution:** BE-SOL-003
**Priority:** P2
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/usecase/revoke_grant.go` (new), `backend-go/services/task-service/internal/usecase/list_grants.go` (new), `backend-go/services/task-service/internal/usecase/ports.go` (extend `GrantRepository`), `backend-go/services/task-service/internal/adapter/postgres/grants.go` (new methods), `backend-go/proto/orca/task/v1/task.proto` (`RevokeGrant`/`ListGrants` RPCs), `backend-go/services/task-service/internal/adapter/grpc/server.go` (handlers)
**Depends on:** TASK-TG-003-03 (this task's `ListGrants` response should carry `expires_at` — land after that field exists, not before, to avoid a second wire-message revision)
**Status:** `[x]` DONE

---

## Context

These are **public-facing** RPCs, distinct from the internal
`GrantRepository.ListGrantsForAncestors` (`internal/adapter/postgres/grants.go:41-74`,
confirmed by direct read) that `ResolvePermission` already uses for its BFS
walk — `ListGrantsForAncestors` returns a map keyed by task ID across a
whole ancestor chain; `ListGrants` (this task) returns every grant on ONE
task, for the frontend's Access tab (per BE-SOL-003, consumed by
[FE-SOL-001](../../../../../frontend/crs/v4/task-graph/solutions/FE-SOL-001-task-crud-board-grant-ui.md)).

Neither RPC nor usecase exists today — confirmed, `grep -rn
"RevokeGrant\|ListGrants\b" internal/` (excluding
`ListGrantsForAncestors`) finds nothing.

`RevokeGrant` deletes by the `(task_id, subject_id, level)` composite key —
matching how a grant is currently created/matched (`Grant`'s own
`INSERT INTO task.task_grants (tenant_id, task_id, subject_id, level,
apply_tree, expires_at)`, `grants.go:31-34`/TASK-TG-003-03) — no surrogate
`grant_id` exists today (confirmed, `migrations/0001_init.up.sql:67-76`'s
`task.task_grants` table has an auto `id UUID PRIMARY KEY` column but no
usecase/RPC ever surfaces it — adding one is a larger schema-surfacing
change this solution avoids per BE-SOL-003's own note, unless the
composite-key delete proves ambiguous in practice, e.g. two identical
`(task_id, subject_id, level)` rows from a double-grant bug; if that's
possible today, flag it rather than silently deleting both).

## Changes to make

**1. `internal/usecase/ports.go`** — extend `GrantRepository`:

```go
type GrantRepository interface {
	Grant(ctx context.Context, tenantID string, grant domain.Grant) error
	ListGrantsForAncestors(ctx context.Context, tenantID string, taskIDs []string) (map[string][]domain.Grant, error)
	// NEW:
	Revoke(ctx context.Context, tenantID, taskID, subjectID string, level domain.GrantLevel) error
	ListByTask(ctx context.Context, tenantID, taskID string) ([]domain.Grant, error)
}
```

**2. `internal/adapter/postgres/grants.go`** — append:

```go
func (r *Repository) Revoke(ctx context.Context, tenantID, taskID, subjectID string, level domain.GrantLevel) error {
	levelStr, ok := grantLevelToString[level]
	if !ok {
		return fmt.Errorf("postgres: unrecognized grant level %v", level)
	}
	_, err := r.db.Exec(ctx, `
		DELETE FROM task.task_grants WHERE tenant_id = $1 AND task_id = $2 AND subject_id = $3 AND level = $4
	`, tenantID, taskID, subjectID, levelStr)
	if err != nil {
		return fmt.Errorf("postgres: revoke task grant: %w", err)
	}
	return nil // idempotent by design — DELETE affecting 0 rows is not an error
}

func (r *Repository) ListByTask(ctx context.Context, tenantID, taskID string) ([]domain.Grant, error) {
	rows, err := r.db.Query(ctx, `
		SELECT task_id, subject_id, level, apply_tree, expires_at FROM task.task_grants
		WHERE tenant_id = $1 AND task_id = $2
	`, tenantID, taskID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query grants for task: %w", err)
	}
	defer rows.Close()
	var out []domain.Grant
	for rows.Next() {
		var g domain.Grant
		var level string
		if err := rows.Scan(&g.TaskID, &g.SubjectID, &level, &g.ApplyTree, &g.ExpiresAt); err != nil {
			return nil, fmt.Errorf("postgres: scan grant row: %w", err)
		}
		g.Level = stringToGrantLevel[level]
		out = append(out, g)
	}
	return out, rows.Err()
}
```

**3. `internal/usecase/revoke_grant.go`** / `list_grants.go`** (new,
following `usecase.Grant`'s existing shape, `internal/usecase/grant.go:1-50`):

```go
type RevokeGrantInput struct {
	TaskID, SubjectID string
	Level             domain.GrantLevel
}

func (uc *RevokeGrant) Execute(ctx context.Context, in RevokeGrantInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil { return apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err) }
	if err := uc.grants.Revoke(ctx, tenantID, in.TaskID, in.SubjectID, in.Level); err != nil {
		return apperrors.New(apperrors.KindInternal, "TASK_REVOKE_GRANT_FAILED", "failed to revoke grant", err)
	}
	return nil
}
```

```go
func (uc *ListGrants) Execute(ctx context.Context, taskID string) ([]domain.Grant, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil { return nil, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err) }
	grants, err := uc.grants.ListByTask(ctx, tenantID, taskID)
	if err != nil { return nil, apperrors.New(apperrors.KindInternal, "TASK_LIST_GRANTS_FAILED", "failed to list grants for task", err) }
	return grants, nil
}
```

**4. `task.proto`** — after `GrantResponse` (`task.proto:110`):

```protobuf
rpc RevokeGrant(RevokeGrantRequest) returns (google.protobuf.Empty);
rpc ListGrants(ListGrantsRequest) returns (ListGrantsResponse);

message RevokeGrantRequest {
  string task_id = 1;
  string subject_id = 2;
  GrantLevel level = 3;
}

message ListGrantsRequest {
  string task_id = 1;
}
message ListGrantsResponse {
  repeated GrantView grants = 1;
}
message GrantView {
  string subject_id = 1;
  GrantLevel level = 2;
  bool apply_tree = 3;
  google.protobuf.Timestamp expires_at = 4; // unset if never expires
}
```

**5. `server.go`** handlers, following the `Grant` handler pattern
(`internal/adapter/grpc/server.go:104-116`).

## Test plan

- `RevokeGrant` idempotent (revoke twice, second call no-ops, no error) —
  per the DELETE-affecting-0-rows-is-fine design above.
- `ListGrants` on a task with 3 grants (owner/admin/team) returns all 3,
  `expires_at` populated correctly for one with an expiry set.
- `RevokeGrant` on a grant that another ancestor's `ApplyTree=true` grant
  still covers → caller with only the revoked grant loses access, but a
  caller matching the ancestor's grant is unaffected (regression test that
  revoke only removes the ONE row, not the whole resolution chain).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/usecase/... -run "TestRevokeGrant|TestListGrants" -v
go test ./services/task-service/internal/adapter/postgres/... -run TestRepository -v
```

Expected: clean build; idempotent-revoke test passes; `ListGrants` result
matches the seeded grant rows exactly, including `expires_at`.

## Execution notes (2026-09-09)

Implemented exactly per the task's code samples: `GrantRepository.Revoke`/
`ListByTask` added to `ports.go`; `postgres/grants.go` gained both methods
verbatim (composite-key `DELETE`, idempotent by design); `usecase.RevokeGrant`/
`usecase.ListGrants` (2 new files) match the task's sketches; `task.proto`
gained `RevokeGrant`/`ListGrants` RPCs + `RevokeGrantRequest`/
`ListGrantsRequest`/`ListGrantsResponse`/`GrantView` messages (regenerated
via `buf generate`) — `GrantView` is a dedicated projection (no `task_id`,
matching the task's own implied shape since the caller already has it from
`ListGrantsRequest`). Server handlers follow the `Grant` handler's exact
pattern; wired into `main.go`.

No surrogate `grant_id` was added — confirmed via direct read of
`migrations/0001_init.up.sql:67-76` that `task.task_grants` has an unused
auto `id UUID PRIMARY KEY` column, matching the task's own note; the
composite-key delete is unambiguous for every code path this task's own
usecases create grants through (`Grant`/`CreateTask`'s owner-grant insert),
so no double-grant collision risk was found — not flagged further.

Fixed test fallout: added `Revoke`/`ListByTask` to both `fakeGrantRepository`
(usecase package) and `fakeTaskRepository` (grpc package, which stands in
for `GrantRepository` too); updated both `server_test.go` server-construction
call sites for the 2 new constructor params. Added exactly the 3 cases the
task's own Test plan names, all passing: `TestRevokeGrant_Idempotent_SecondCallNoOps`;
`TestListGrants_ReturnsAllGrantsOnTheTask` (3 grants, one with `expires_at`
set, a 4th grant on a DIFFERENT task correctly excluded); and
`TestRevokeGrant_OnlyRemovesTheOneRow` (the regression test that revoking
one caller's direct grant doesn't disturb a different caller's still-valid
inherited ancestor grant — real `ResolvePermission`/`domain.ResolveGrant`
BFS walk, not a special-cased assertion). Also added a new integration
file, `internal/adapter/postgres/grants_integration_test.go`
(`TestRepository_Revoke_And_ListByTask`), against a real database.

Verify: `go build`/`go vet ./services/task-service/...` both clean
(including `-tags=integration`); `go test .../usecase/... -run
"TestRevokeGrant|TestListGrants"` — all 7 cases pass; full `go test
./services/task-service/...` passes with no regressions; `go test
-tags=integration .../postgres/... -run TestRepository_Revoke_And_ListByTask`
passes cleanly (4.61s, no flake hit this run).
