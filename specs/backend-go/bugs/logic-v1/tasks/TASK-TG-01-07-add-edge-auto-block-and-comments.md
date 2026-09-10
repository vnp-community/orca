# TASK-TG-01-07: `AddEdge` auto-block + atomic cycle-check, `UpdateTask` un-block, `AddComment`/`ListComments`

**From Solution:** SOL-TG-01
**Priority:** P1 — closes a real known gap (`README.md:199-205`: cycle-check-then-write is not atomic)
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/usecase/add_edge.go`
**Depends on:** TASK-TG-01-03, TASK-TG-01-04, TASK-TG-01-05
**Status:** `[x]` DONE — AddEdge now runs cycle-check+write+auto-block in one TxRunner transaction (ListByKindForUpdate); UpdateTask un-blocks dependents on transition to Done; AddComment/ListComments usecases added; go test ./internal/usecase/... -run TestAddEdge\|TestUpdateTask\|TestAddComment\|TestListComments passes.

---

## Context

Two fixes land in the same usecase since both touch the same
read-then-write shape `add_edge.go:41-54`'s own comment already flags as
non-atomic: (1) the cycle-check-then-write race, closed with a transaction +
`SELECT ... FOR UPDATE`; (2) auto-block — adding a `depends_on` edge to a
not-yet-`Done` task should transition `from` to `StatusBlocked`, and
`UpdateTask` transitioning a task INTO `Done` must un-block its direct
dependents whose every dependency is now met.

## Changes to make

Add `RunInTxWithGrants`-shaped access is not needed here — `AddEdge` already
has `TxRunner` available (same `RunInTx(ctx, fn func(ctx, TaskRepository,
EdgeRepository) error) error` shape `AIApply` uses). Rewrite
`backend-go/services/task-service/internal/usecase/add_edge.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type AddEdgeInput struct {
	FromTaskID string
	ToTaskID   string
	Kind       domain.EdgeKind
}

// AddEdge is task-service's edge-mutation usecase. Runs the cycle check
// (depends_on only) and the write in ONE transaction via TxRunner, closing
// README.md's "known gap": a SELECT ... FOR UPDATE over the depends_on edge
// set (EdgeRepository.ListByKindForUpdate) closes the race the prior
// two-call (ListByKind then Add) shape allowed. Also implements auto-block:
// adding "from depends_on to" means "from must wait for to" — if `to` isn't
// Done/Cancelled, `from` transitions to StatusBlocked.
type AddEdge struct {
	txRunner TxRunner
}

func NewAddEdge(txRunner TxRunner) *AddEdge {
	return &AddEdge{txRunner: txRunner}
}

func (uc *AddEdge) Execute(ctx context.Context, in AddEdgeInput) (domain.TaskEdge, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.TaskEdge{}, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	edge, err := domain.NewTaskEdge(in.FromTaskID, in.ToTaskID, in.Kind)
	if err != nil {
		return domain.TaskEdge{}, apperrors.New(apperrors.KindInvalidArgument, "TASK_EDGE_INVALID", err.Error(), err)
	}

	err = uc.txRunner.RunInTx(ctx, func(ctx context.Context, tasks TaskRepository, edges EdgeRepository) error {
		if edge.Kind == domain.EdgeKindDependsOn {
			existing, err := edges.ListByKindForUpdate(ctx, tenantID, domain.EdgeKindDependsOn)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "TASK_EDGE_LIST_FAILED", "failed to list existing edges for cycle check", err)
			}
			if domain.DetectCycle(existing, edge) {
				return apperrors.New(apperrors.KindFailedPrecondition, "TASK_CYCLIC_DEPENDENCY", domain.ErrCyclicDependency.Error(), domain.ErrCyclicDependency)
			}
		}
		if err := edges.Add(ctx, tenantID, edge); err != nil {
			return apperrors.New(apperrors.KindInternal, "TASK_EDGE_ADD_FAILED", "failed to persist edge", err)
		}

		if edge.Kind == domain.EdgeKindDependsOn {
			dep, err := tasks.Get(ctx, tenantID, edge.ToTaskID)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "TASK_EDGE_DEP_LOOKUP_FAILED", "failed to load dependency task", err)
			}
			if dep.Status != domain.StatusDone && dep.Status != domain.StatusCancelled {
				if err := tasks.UpdateStatus(ctx, tenantID, edge.FromTaskID, domain.StatusBlocked); err != nil {
					return apperrors.New(apperrors.KindInternal, "TASK_EDGE_AUTO_BLOCK_FAILED", "failed to auto-block dependent task", err)
				}
			}
		}
		return nil
	})
	if err != nil {
		return domain.TaskEdge{}, err
	}
	return edge, nil
}
```

Add `ListByKindForUpdate` and `ListTo` to
`backend-go/services/task-service/internal/usecase/ports.go`'s
`EdgeRepository` interface:

```go
	// ListByKindForUpdate is ListByKind's transaction-scoped, row-locked
	// variant — SELECT ... FOR UPDATE over the kind-scoped edge set, closing
	// the check-then-write race AddEdge's prior two-call shape allowed. Only
	// meaningful when called through TxRunner.RunInTx's fn (r.db is a
	// pgx.Tx there); called outside a transaction it behaves like ListByKind.
	ListByKindForUpdate(ctx context.Context, tenantID string, kind domain.EdgeKind) ([]domain.TaskEdge, error)
	// ListTo returns the edges of the given kind terminating AT toTaskID —
	// the symmetric counterpart to ListFrom, used by UpdateTask's un-block
	// step to find a task's dependents.
	ListTo(ctx context.Context, tenantID, toTaskID string, kind domain.EdgeKind) ([]domain.TaskEdge, error)
```

Implement both in
`backend-go/services/task-service/internal/adapter/postgres/edges.go`:

```go
func (r *Repository) ListByKindForUpdate(ctx context.Context, tenantID string, kind domain.EdgeKind) ([]domain.TaskEdge, error) {
	rows, err := r.db.Query(ctx, `
		SELECT from_task_id, to_task_id, edge_type
		FROM task.task_edges
		WHERE tenant_id = $1 AND edge_type = $2
		FOR UPDATE
	`, tenantID, string(kind))
	if err != nil {
		return nil, fmt.Errorf("postgres: query edges by kind for update: %w", err)
	}
	defer rows.Close()
	return scanEdges(rows)
}

func (r *Repository) ListTo(ctx context.Context, tenantID, toTaskID string, kind domain.EdgeKind) ([]domain.TaskEdge, error) {
	rows, err := r.db.Query(ctx, `
		SELECT from_task_id, to_task_id, edge_type
		FROM task.task_edges
		WHERE tenant_id = $1 AND to_task_id = $2 AND edge_type = $3
	`, tenantID, toTaskID, string(kind))
	if err != nil {
		return nil, fmt.Errorf("postgres: query edges to task: %w", err)
	}
	defer rows.Close()
	return scanEdges(rows)
}
```

Extend `usecase/update_task.go`'s `Execute` with an un-block post-write
step, run only when the update transitions status INTO `StatusDone`:

```go
	// Un-block step: a task transitioning to Done may unblock direct
	// dependents whose every depends_on edge is now satisfied.
	if in.Status != nil && *in.Status == domain.StatusDone {
		dependents, err := uc.edges.ListTo(ctx, tenantID, task.ID, domain.EdgeKindDependsOn)
		if err != nil {
			return domain.Task{}, apperrors.New(apperrors.KindInternal, "TASK_UNBLOCK_LOOKUP_FAILED", "failed to list dependents for unblock check", err)
		}
		for _, dep := range dependents {
			dependent, err := uc.repo.Get(ctx, tenantID, dep.FromTaskID)
			if err != nil || dependent.Status != domain.StatusBlocked {
				continue
			}
			blockers, err := uc.edges.ListFrom(ctx, tenantID, dependent.ID, domain.EdgeKindDependsOn)
			if err != nil {
				continue
			}
			allDone := true
			for _, b := range blockers {
				blocker, err := uc.repo.Get(ctx, tenantID, b.ToTaskID)
				if err != nil || (blocker.Status != domain.StatusDone && blocker.Status != domain.StatusCancelled) {
					allDone = false
					break
				}
			}
			if allDone {
				_ = uc.repo.UpdateStatus(ctx, tenantID, dependent.ID, domain.StatusOpen)
			}
		}
	}
```

`UpdateTask` needs an `edges EdgeRepository` field added to its struct and
`NewUpdateTask` constructor — update
`backend-go/services/task-service/cmd/server/main.go`'s
`usecase.NewUpdateTask(repo)` call to `usecase.NewUpdateTask(repo, repo)`.

Add `AddComment`/`ListComments` usecases,
`backend-go/services/task-service/internal/usecase/comment.go` (new):

```go
package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type AddComment struct{ comments CommentRepository }

func NewAddComment(comments CommentRepository) *AddComment { return &AddComment{comments: comments} }

func (uc *AddComment) Execute(ctx context.Context, taskID, content string) (domain.TaskComment, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.TaskComment{}, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	userID, _ := tenant.UserID(ctx)
	c, err := domain.NewTaskComment(uuid.NewString(), taskID, userID, content)
	if err != nil {
		return domain.TaskComment{}, apperrors.New(apperrors.KindInvalidArgument, "TASK_COMMENT_INVALID", err.Error(), err)
	}
	out, err := uc.comments.AddComment(ctx, tenantID, c)
	if err != nil {
		return domain.TaskComment{}, apperrors.New(apperrors.KindInternal, "TASK_COMMENT_ADD_FAILED", "failed to persist comment", err)
	}
	return out, nil
}

type ListComments struct{ comments CommentRepository }

func NewListComments(comments CommentRepository) *ListComments { return &ListComments{comments: comments} }

func (uc *ListComments) Execute(ctx context.Context, taskID, pageToken string, pageSize int32) ([]domain.TaskComment, string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, "", apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	return uc.comments.ListComments(ctx, tenantID, taskID, pageToken, pageSize)
}
```

Update every fake `EdgeRepository`/`TxRunner` in `internal/usecase/*_test.go`
(`fakes_test.go`, `worktree_fakes_test.go`-equivalent) to implement
`ListByKindForUpdate`/`ListTo` so the package compiles.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/usecase/... -run 'TestAddEdge|TestUpdateTask|TestAddComment|TestListComments' -v
```

Expected: `add_edge_test.go` — adding a `depends_on` edge to a not-Done task
sets `from` to `Blocked`; to an already-Done task leaves `from` untouched;
two concurrent `AddEdge` calls racing the same cycle window (fake repo with
an injected delay) — only one succeeds once `RunInTx` + `FOR UPDATE` land.
`update_task_test.go` — transitioning a single-dependency dependent's
blocker to `Done` un-blocks it to `Open`; a multi-dependency dependent stays
`Blocked` until every dependency clears.
