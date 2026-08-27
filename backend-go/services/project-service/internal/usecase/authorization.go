package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// projectActionOwnerOnly/projectActionAnyMember are the two action values
// project.rego's action_roles table keys on — see that file for the exact
// mapping. Every OPA-gated usecase in this package passes one of these two
// constants to requireProjectAccess.
const (
	// projectActionOwnerOnly gates UpdateProject/DeleteProject/AddMember/
	// RebindDevServer per project-service.md §9, plus this service's
	// judgment-call extension to AddRepo/RemoveRepo/ReorderRepos — a
	// repo/worktree belongs to exactly one project, so the project's
	// owner-or-admin rule is the natural fit for its catalog mutations. See
	// this service's README "Known gaps" for the RPCs deliberately left
	// out of that extension (worktree mutation RPCs, ProjectGroup CRUD).
	projectActionOwnerOnly = "owner_only"
	// projectActionAnyMember gates GetProject/ListRepos/ListWorktrees —
	// any membership (owner/member/viewer) or global admin.
	projectActionAnyMember = "any_member"
)

// callerGlobalRole resolves the acting user's system-wide role for
// project.rego's admin-override branch. Always "" today: no role claim
// propagates from api-gateway into a service's request context yet — the
// same gap annotation-service's OPAClient.Decision doc comment documents
// for its own actor_role parameter (see
// annotation-service/internal/adapter/opaclient/client.go). Reusing that
// documented convention here (rather than inventing a new lookup, e.g. a
// new gRPC call to auth-service) means the global-admin-override branch in
// project.rego is inert — proven correct by policy/orca-authz/
// project_test.rego at the Rego layer, but not reachable through this
// service's Go code — until the upstream claim-propagation gap closes.
// Tracked in this service's README "Known gaps", not silently ignored.
func callerGlobalRole(_ context.Context) string {
	return ""
}

// requireProjectAccess resolves the acting user's membership role in
// projectID (if any) and asks OPA (data.orca.authz.project.allow) whether
// action is authorized for that role — a global admin (once
// callerGlobalRole is wired to a real claim) always passes regardless of
// membership. Fails closed on every error path: a missing actor, a
// membership-lookup failure, or a policy-evaluation error are all treated
// as deny, never as allow — matching auth-service.requireAdminActor's exact
// contract (internal/usecase/authorization.go in that service).
func requireProjectAccess(ctx context.Context, membership MembershipRepository, opa OPAClient, projectID, action string) error {
	actorID, ok := tenant.UserID(ctx)
	if !ok {
		return apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_USER", "no user in request context", nil)
	}

	callerProjectRole := ""
	m, err := membership.GetMembership(ctx, projectID, actorID)
	switch {
	case errors.Is(err, domain.ErrMembershipNotFound):
		// No membership row — callerProjectRole stays "". project.rego's
		// action_roles has no "" entry, so this alone can never allow; only
		// a global-admin caller_global_role can still pass.
	case err != nil:
		return apperrors.New(apperrors.KindInternal, "PROJECT_MEMBERSHIP_LOOKUP_FAILED", "failed to resolve caller's project membership", err)
	default:
		callerProjectRole = string(m.Role)
	}

	allowed, err := opa.Decision(ctx, callerProjectRole, callerGlobalRole(ctx), action)
	if err != nil {
		// Fail closed: a policy-evaluation error is never treated as an
		// allow — matching every other OPA call site in this codebase
		// (common/policy.Evaluator's doc comment).
		return apperrors.New(apperrors.KindInternal, "PROJECT_POLICY_EVAL_FAILED", "failed to evaluate authorization policy", err)
	}
	if !allowed {
		return apperrors.New(apperrors.KindPermissionDenied, "PROJECT_NOT_AUTHORIZED", "caller is not authorized for this action", nil)
	}
	return nil
}
