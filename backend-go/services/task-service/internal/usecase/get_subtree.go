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
	Tasks          []domain.Task
	DependsOnEdges []domain.TaskEdge
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
