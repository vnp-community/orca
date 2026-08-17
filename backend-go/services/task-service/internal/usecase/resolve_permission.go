package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type ResolvePermissionInput struct {
	TaskID string
	UserID string
}

// ResolvePermission is task-service's core security-surface usecase
// (task-service.md §4.1/§9): it gathers the ancestor chain and the grants
// recorded on it, resolves the caller's team membership via
// TeamScopeResolver, then hands all three to the pure
// domain.ResolveGrant BFS walk.
//
// Per §9, this usecase returns the RESOLVED LEVEL, not a final allow/deny —
// that decision belongs to an OPA policy evaluation this scaffold doesn't
// wire (no adapter/opaclient/), consuming this result as its input
// document. See this service's README.
type ResolvePermission struct {
	tasks    TaskRepository
	grants   GrantRepository
	teams    TeamScopeResolver
	maxDepth int
}

func NewResolvePermission(tasks TaskRepository, grants GrantRepository, teams TeamScopeResolver) *ResolvePermission {
	return &ResolvePermission{tasks: tasks, grants: grants, teams: teams, maxDepth: domain.DefaultMaxAncestorDepth}
}

func (uc *ResolvePermission) Execute(ctx context.Context, in ResolvePermissionInput) (domain.GrantLevel, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.GrantLevelUnspecified, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}

	ancestors, err := uc.tasks.GetAncestors(ctx, tenantID, in.TaskID, uc.maxDepth)
	if err != nil {
		return domain.GrantLevelUnspecified, apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found while resolving ancestors", err)
	}
	chain := make([]string, 0, len(ancestors))
	for _, a := range ancestors {
		chain = append(chain, a.ID)
	}

	grantsByTask, err := uc.grants.ListGrantsForAncestors(ctx, tenantID, chain)
	if err != nil {
		return domain.GrantLevelUnspecified, apperrors.New(apperrors.KindInternal, "TASK_GRANT_LIST_FAILED", "failed to list grants for ancestor chain", err)
	}

	teamIDs, err := uc.teams.ResolveTeams(ctx, tenantID, in.UserID)
	if err != nil {
		return domain.GrantLevelUnspecified, apperrors.New(apperrors.KindInternal, "TASK_TEAM_RESOLVE_FAILED", "failed to resolve caller's team membership", err)
	}

	caller := domain.CallerIdentity{UserID: in.UserID, TeamIDs: teamIDs, CompanyID: tenantID}
	level, found := domain.ResolveGrant(chain, grantsByTask, caller, uc.maxDepth)
	if !found {
		return domain.GrantLevelUnspecified, apperrors.New(apperrors.KindPermissionDenied, "TASK_NO_GRANT", "no applicable grant found for caller", nil)
	}
	return level, nil
}
