# TASK-TG-03-07: `RevokeGrant`/`ListGrants` usecases + postgres `Revoke`/expiry filter + grant-notification outbox write

**From Solution:** SOL-TG-03
**Priority:** P1
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/usecase/revoke_grant.go` (new), `backend-go/services/task-service/internal/usecase/list_grants.go` (new), `backend-go/services/task-service/internal/adapter/postgres/grants.go`
**Depends on:** TASK-TG-03-01, TASK-TG-03-04, TASK-TG-03-05, TASK-TG-03-06
**Status:** `[x]` DONE — RevokeGrant/ListGrants usecases added (both manage-gated); postgres Grant widened (expires_at + RETURNING id), Revoke, ListGrantsForTask added, ListGrantsForAncestors filters expired rows at SQL layer too; migration 0005 adds task.outbox_events; internal/adapter/eventbus.Publisher + common/outbox.Relay wired in main.go (mirrors usage-service's transactional-outbox pattern — real pattern lives in its postgres repo, not a separate eventbus package as the task doc named; followed the REAL pattern). NOTE: Grant/RevokeGrant's event publish is a separate call after the DB write, not same-transaction — flagged as a follow-up per this task's own Verify note. go test ./internal/usecase/... -run 'TestRevokeGrant\|TestListGrants\|TestGrant' and ./internal/adapter/postgres/... -run TestRepository (grants/revoke/expiry cases) both pass.

---

## Context

`RevokeGrant`/`ListGrants` are new RPCs — both require `manage` on the
target task (spec: "the 'manage' permission includes viewing/managing
existing grants"). `ListGrants` returns only the target task's own grant
rows (not the whole ancestor chain — leaking an ancestor's grant details to
someone without visibility into that ancestor would be a real information
leak). Per `task-service.md §9`, `Grant`/`RevokeGrant` should also emit
structured audit events — closed here via the transactional-outbox pattern
(`05-data-architecture.md:82-98`), reusing `usage-service`'s
`adapter/eventbus/` package shape.

## Changes to make

Add `Revoke` and an expiry filter to `ListGrantsForAncestors` in
`backend-go/services/task-service/internal/adapter/postgres/grants.go`:

```go
func (r *Repository) Revoke(ctx context.Context, tenantID, grantID string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM task.task_grants WHERE tenant_id = $1 AND id = $2`, tenantID, grantID)
	if err != nil {
		return fmt.Errorf("postgres: revoke task grant: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: grant %s not found", grantID)
	}
	return nil
}

func (r *Repository) ListGrantsForTask(ctx context.Context, tenantID, taskID string) ([]domain.Grant, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, task_id, subject_id, level, apply_tree, expires_at
		FROM task.task_grants
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
		if err := rows.Scan(&g.ID, &g.TaskID, &g.SubjectID, &level, &g.ApplyTree, &g.ExpiresAt); err != nil {
			return nil, fmt.Errorf("postgres: scan grant row: %w", err)
		}
		g.Level = stringToGrantLevel[level]
		out = append(out, g)
	}
	return out, rows.Err()
}
```

Widen `ListGrantsForAncestors`'s `SELECT` to also read `expires_at` and
filter at the SQL layer too (defense-in-depth alongside the domain-layer
filter from `TASK-TG-03-06`):

```go
	rows, err := r.db.Query(ctx, `
		SELECT task_id, subject_id, level, apply_tree, expires_at
		FROM task.task_grants
		WHERE tenant_id = $1 AND task_id = ANY($2) AND (expires_at IS NULL OR expires_at > now())
	`, tenantID, taskIDs)
	...
	if err := rows.Scan(&g.TaskID, &g.SubjectID, &level, &g.ApplyTree, &g.ExpiresAt); err != nil { ... }
```

Also widen `Repository.Grant` to insert `expires_at` and return the new
row's `id`:

```go
func (r *Repository) Grant(ctx context.Context, tenantID string, grant domain.Grant) (string, error) {
	level, ok := grantLevelToString[grant.Level]
	if !ok {
		return "", fmt.Errorf("postgres: unrecognized grant level %v", grant.Level)
	}
	row := r.db.QueryRow(ctx, `
		INSERT INTO task.task_grants (tenant_id, task_id, subject_id, level, apply_tree, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, tenantID, grant.TaskID, grant.SubjectID, level, grant.ApplyTree, grant.ExpiresAt)
	var id string
	if err := row.Scan(&id); err != nil {
		return "", fmt.Errorf("postgres: insert task grant: %w", err)
	}
	return id, nil
}
```

Update `GrantRepository`'s port signature in `ports.go` (`Grant` now returns
`(string, error)`) and `usecase.Grant.Execute` (from `TASK-TG-03-01`) to
capture and return the new grant's `id` — thread it through
`GrantResponse.Id` in the gRPC handler.

Create `backend-go/services/task-service/internal/usecase/revoke_grant.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type RevokeGrantInput struct {
	TaskID  string
	GrantID string
}

// RevokeGrant requires 'manage' on TaskID before deleting a grant, same
// pre-check TASK-TG-03-01 added to Grant — RevokeGrant is new code, built
// with the check from the start.
type RevokeGrant struct {
	grants            GrantRepository
	resolvePermission *ResolvePermission
	events            EventPublisher
}

func NewRevokeGrant(grants GrantRepository, resolvePermission *ResolvePermission, events EventPublisher) *RevokeGrant {
	return &RevokeGrant{grants: grants, resolvePermission: resolvePermission, events: events}
}

func (uc *RevokeGrant) Execute(ctx context.Context, in RevokeGrantInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	callerID, _ := tenant.UserID(ctx)
	if _, err := uc.resolvePermission.Execute(ctx, ResolvePermissionInput{TaskID: in.TaskID, UserID: callerID, Action: "manage"}); err != nil {
		return err
	}
	if err := uc.grants.Revoke(ctx, tenantID, in.GrantID); err != nil {
		return apperrors.New(apperrors.KindNotFound, "TASK_GRANT_NOT_FOUND", "grant not found", err)
	}
	uc.events.Publish(ctx, tenantID, "task.grant_revoked", map[string]any{"task_id": in.TaskID, "grant_id": in.GrantID, "revoked_by": callerID})
	return nil
}
```

Create `backend-go/services/task-service/internal/usecase/list_grants.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type ListGrants struct {
	grants            GrantRepository
	resolvePermission *ResolvePermission
}

func NewListGrants(grants GrantRepository, resolvePermission *ResolvePermission) *ListGrants {
	return &ListGrants{grants: grants, resolvePermission: resolvePermission}
}

func (uc *ListGrants) Execute(ctx context.Context, taskID string) ([]domain.Grant, error) {
	callerID, _ := tenant.UserID(ctx)
	if _, err := uc.resolvePermission.Execute(ctx, ResolvePermissionInput{TaskID: taskID, UserID: callerID, Action: "manage"}); err != nil {
		return nil, err
	}
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	// Only the target task's own grants — not the ancestor chain, which
	// would leak ancestor-task grant details to a caller who may not have
	// visibility into the ancestor task itself.
	return uc.grants.ListGrantsForTask(ctx, tenantID, taskID)
}
```

Add an `EventPublisher` port to `ports.go`:

```go
// EventPublisher writes a best-effort outbox row for async consumption
// (notification-service) — see adapter/eventbus's doc comment for the
// polling-outbox implementation, mirroring usage-service's package.
type EventPublisher interface {
	Publish(ctx context.Context, tenantID, eventType string, payload map[string]any)
}
```

Give `usecase.Grant` (from `TASK-TG-03-01`) the same `events EventPublisher`
dependency and a `Publish(ctx, tenantID, "task.grant_received", map[string]any{...})`
call right after a successful `uc.grants.Grant(...)` call.

Add `internal/adapter/eventbus/publisher.go` (new package) mirroring
`usage-service/internal/adapter/eventbus/`'s existing outbox-write +
polling-publish shape — write an `outbox` row inside the same DB call
transaction where practical, publish to NATS JetStream via a background
poller. This is the one piece of this task requiring a new package; follow
`usage-service`'s package structure file-for-file rather than inventing a
new shape.

Wire `RevokeGrant`/`ListGrants` into `cmd/server/main.go`'s composition root
and `taskgrpc.New`; add gRPC handlers to `server.go`:

```go
func (s *Server) RevokeGrant(ctx context.Context, req *taskv1.RevokeGrantRequest) (*emptypb.Empty, error) {
	if err := s.revokeGrant.Execute(ctx, usecase.RevokeGrantInput{TaskID: req.GetTaskId(), GrantID: req.GetGrantId()}); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) ListGrants(ctx context.Context, req *taskv1.ListGrantsRequest) (*taskv1.ListGrantsResponse, error) {
	grants, err := s.listGrants.Execute(ctx, req.GetTaskId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*taskv1.Grant, 0, len(grants))
	for _, g := range grants {
		pg := &taskv1.Grant{Id: g.ID, TaskId: g.TaskID, SubjectId: g.SubjectID, Level: toProtoGrantLevel(g.Level), ApplyTree: g.ApplyTree}
		if g.ExpiresAt != nil {
			pg.ExpiresAt = timestamppb.New(*g.ExpiresAt)
		}
		out = append(out, pg)
	}
	return &taskv1.ListGrantsResponse{Grants: out}, nil
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/usecase/... -run 'TestRevokeGrant|TestListGrants|TestGrant' -v
go test ./services/task-service/internal/adapter/postgres/... -run 'TestGrants' -v
```

Expected: `revoke_grant_test.go` — `manage`-gate denies a caller without
access; revoking a nonexistent grant ID is `NOT_FOUND`, not a silent no-op.
`grant_test.go` — the outbox row is written alongside the grant insert; a
fake `EventPublisher`/`TxRunner` that fails asserts the grant write itself
also rolls back once `Grant.Execute` is moved behind a transaction (a
follow-up if `Grant`'s current shape doesn't already use `TxRunner` — flag
if it needs one). `grants_test.go` (postgres, integration) —
`ListGrantsForAncestors` excludes expired rows at the SQL layer too.
