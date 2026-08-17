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
	// Action is the operation the caller wants to perform on TaskID —
	// one of task_grant.rego's level_actions keys ("read"/"write"/
	// "execute"/"admin"). The generated ResolvePermissionRequest proto
	// message has no action-equivalent field yet (see this service's
	// README "Known gaps"), so internal/adapter/grpc.Server.ResolvePermission
	// defaults this to "read" today — every named GrantLevel authorizes
	// "read", so this only changes behavior for the (already-denied)
	// not-found case. Pass Action explicitly once the wire contract grows
	// a real field for it.
	Action string
}

// ResolvePermission is task-service's core security-surface usecase
// (task-service.md §4.1/§9): it gathers the ancestor chain and the grants
// recorded on it, resolves the caller's team membership via
// TeamScopeResolver, hands all three to the pure domain.ResolveGrant BFS
// walk, then — per §9's domain-computes/OPA-decides split — asks OPAClient
// whether the resolved level actually authorizes the requested Action.
//
// The BFS walk's own not-found result and an OPA deny are both surfaced as
// the identical PermissionDenied/TASK_NO_GRANT error: callers don't get to
// tell "no grant at all" apart from "a grant exists but doesn't cover this
// action" from the error alone, which is intentional — neither case should
// leak which ancestor/grant rows exist to a caller who isn't authorized to
// see them.
type ResolvePermission struct {
	tasks    TaskRepository
	grants   GrantRepository
	teams    TeamScopeResolver
	opa      OPAClient
	maxDepth int
}

func NewResolvePermission(tasks TaskRepository, grants GrantRepository, teams TeamScopeResolver, opa OPAClient) *ResolvePermission {
	return &ResolvePermission{tasks: tasks, grants: grants, teams: teams, opa: opa, maxDepth: domain.DefaultMaxAncestorDepth}
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
		return domain.GrantLevelUnspecified, errNoGrant(nil)
	}

	// Fail closed: an evaluation error is treated exactly like an explicit
	// deny, never like an implicit allow (§9's "OPA's job is the ... final
	// allow/deny" — a broken policy evaluation must never fall back to
	// trusting the resolved level alone).
	allowed, err := uc.opa.Decision(ctx, level, in.Action, tenantID)
	if err != nil || !allowed {
		return domain.GrantLevelUnspecified, errNoGrant(err)
	}
	return level, nil
}

// errNoGrant builds the single PermissionDenied/TASK_NO_GRANT-shaped error
// Execute returns for both "no matching grant" and "a grant matched but
// OPA denied the requested action" — see ResolvePermission's doc comment.
func errNoGrant(cause error) error {
	return apperrors.New(apperrors.KindPermissionDenied, "TASK_NO_GRANT", "no applicable grant found for caller", cause)
}
