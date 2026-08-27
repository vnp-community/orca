# TASK-TG-01-06: Usecase — `GetSubtree` (batched access filter) and `RecalculateProgress` (bottom-up cascade)

**From Solution:** SOL-TG-01
**Priority:** P1
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/usecase/get_subtree.go` (new), `backend-go/services/task-service/internal/usecase/recalculate_progress.go` (new)
**Depends on:** TASK-TG-01-05
**Status:** `[ ]` TODO

---

## Context

Per `task-service.md §8`'s grant-resolution hot-path NFR, `GetSubtree` must
not call `ResolvePermission` once per node (an N-call fan-out) — it
batch-fetches every grant on every task in the subtree in one query
(`ListGrantsForAncestors`, reused as-is) and resolves each node's access
in-memory by reusing `domain.ResolveGrant` unchanged, since every node's
ancestor chain is a prefix of the path already walked to reach it.
`RecalculateProgress` reduces `GetSubtreeWithChildPercents`'s deepest-first
rows through `domain.CalculateProgress` and persists in one batched write.

## Changes to make

Create `backend-go/services/task-service/internal/usecase/get_subtree.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type GetSubtreeInput struct {
	RootID string
}

type GetSubtreeResult struct {
	Tasks           []domain.Task
	DependsOnEdges  []domain.TaskEdge
}

// GetSubtree returns the visible portion of RootID's subtree for the
// calling user — per-node access filter, not a whole-branch cut: a mid-tree
// task the caller has no grant on is excluded but its own children (if
// independently granted) are not. Batches grant resolution (one
// ListGrantsForAncestors call for the whole subtree, not one ResolveGrant
// round-trip per node) per task-service.md §8's hot-path NFR.
type GetSubtree struct {
	tasks    TaskRepository
	grants   GrantRepository
	teams    TeamScopeResolver
	maxDepth int
}

func NewGetSubtree(tasks TaskRepository, grants GrantRepository, teams TeamScopeResolver) *GetSubtree {
	return &GetSubtree{tasks: tasks, grants: grants, teams: teams, maxDepth: domain.DefaultMaxAncestorDepth}
}

func (uc *GetSubtree) Execute(ctx context.Context, in GetSubtreeInput) (GetSubtreeResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return GetSubtreeResult{}, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	userID, _ := tenant.UserID(ctx)

	nodes, edges, err := uc.tasks.GetSubtree(ctx, tenantID, in.RootID, uc.maxDepth)
	if err != nil {
		return GetSubtreeResult{}, apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found while resolving subtree", err)
	}

	taskIDs := make([]string, 0, len(nodes))
	byID := make(map[string]domain.Task, len(nodes))
	for _, n := range nodes {
		taskIDs = append(taskIDs, n.ID)
		byID[n.ID] = n
	}

	grantsByTask, err := uc.grants.ListGrantsForAncestors(ctx, tenantID, taskIDs)
	if err != nil {
		return GetSubtreeResult{}, apperrors.New(apperrors.KindInternal, "TASK_GRANT_LIST_FAILED", "failed to list grants for subtree", err)
	}
	teamIDs, err := uc.teams.ResolveTeams(ctx, tenantID, userID)
	if err != nil {
		return GetSubtreeResult{}, apperrors.New(apperrors.KindInternal, "TASK_TEAM_RESOLVE_FAILED", "failed to resolve caller's team membership", err)
	}
	caller := domain.CallerIdentity{UserID: userID, TeamIDs: teamIDs, CompanyID: tenantID}

	visible := make([]domain.Task, 0, len(nodes))
	visibleIDs := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		chain := chainOf(n.ID, byID) // n, parent, grandparent, ... root — no extra query, walks already-fetched ParentID pointers
		if _, ok := domain.ResolveGrant(chain, grantsByTask, caller, uc.maxDepth); ok {
			visible = append(visible, n)
			visibleIDs[n.ID] = true
		}
	}

	filteredEdges := make([]domain.TaskEdge, 0, len(edges))
	for _, e := range edges {
		if visibleIDs[e.FromTaskID] {
			filteredEdges = append(filteredEdges, e)
		}
	}
	return GetSubtreeResult{Tasks: visible, DependsOnEdges: filteredEdges}, nil
}

// chainOf walks n's ParentID pointers within the already-fetched subtree
// map (no DB call) to build [n, parent, grandparent, ..., root] — exactly
// the ancestorChain shape domain.ResolveGrant expects.
func chainOf(id string, byID map[string]domain.Task) []string {
	chain := make([]string, 0, 8)
	for id != "" {
		chain = append(chain, id)
		t, ok := byID[id]
		if !ok {
			break
		}
		id = t.ParentID
	}
	return chain
}
```

Create `backend-go/services/task-service/internal/usecase/recalculate_progress.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// RecalculateProgress reduces a subtree bottom-up (deepest-first) through
// domain.CalculateProgress, then persists every changed value in ONE
// batched write — task-service.md §8's "one WITH RECURSIVE aggregate query
// rather than N+1 fetches" NFR.
type RecalculateProgress struct {
	tasks TaskRepository
}

func NewRecalculateProgress(tasks TaskRepository) *RecalculateProgress {
	return &RecalculateProgress{tasks: tasks}
}

func (uc *RecalculateProgress) Execute(ctx context.Context, rootID string) (int, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return 0, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}

	subtree, err := uc.tasks.GetSubtreeWithChildPercents(ctx, tenantID, rootID)
	if err != nil {
		return 0, apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found while recalculating progress", err)
	}

	updates := make(map[string]int, len(subtree))
	childPercentsByParent := map[string][]int{}
	rootPercent := 0
	for _, node := range subtree { // already ordered deepest-first by the repository query
		children := childPercentsByParent[node.Task.ID]
		p := domain.CalculateProgress(node.Task, children)
		updates[node.Task.ID] = p
		if node.Task.ParentID != "" {
			childPercentsByParent[node.Task.ParentID] = append(childPercentsByParent[node.Task.ParentID], p)
		}
		if node.Task.ID == rootID {
			rootPercent = p
		}
	}

	if err := uc.tasks.BatchUpdateProgress(ctx, tenantID, updates); err != nil {
		return 0, apperrors.New(apperrors.KindInternal, "TASK_PROGRESS_UPDATE_FAILED", "failed to persist recalculated progress", err)
	}
	return rootPercent, nil
}
```

Note: `GetSubtreeWithChildPercents` returns `[]subtreeProgressRow`
(unexported in `postgres`, see `TASK-TG-01-05`) — expose it via the
`TaskRepository` port as a usecase-local named type instead
(`usecase.SubtreeProgressNode{ Task domain.Task; Depth int; ChildPercents
[]int }`), with `postgres.Repository.GetSubtreeWithChildPercents` returning
that type directly. Add the port method to `ports.go`:

```go
	GetSubtree(ctx context.Context, tenantID, rootID string, maxDepth int) ([]domain.Task, []domain.TaskEdge, error)
	GetSubtreeWithChildPercents(ctx context.Context, tenantID, rootID string) ([]SubtreeProgressNode, error)
	BatchUpdateProgress(ctx context.Context, tenantID string, updates map[string]int) error
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/usecase/... -run 'TestGetSubtree|TestRecalculateProgress' -v
```

Expected: `get_subtree_test.go` (fake repos) — a subtree with a mid-tree task
the caller has no grant on is excluded but its own independently-granted
children are not. `recalculate_progress_test.go` — three-level fixture
matching `domain/progress_test.go`'s cascade case; `BatchUpdateProgress`
asserted called once with every changed node (regression guard against
N+1).
