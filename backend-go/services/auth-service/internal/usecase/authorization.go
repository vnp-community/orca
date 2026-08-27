package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// requireAdminActor resolves the acting user from context and checks it's
// authorized as admin via the embedded OPA policy decision
// (data.orca.authz.admin.allow, internal/adapter/opaclient) — per
// auth-service.md §6/§9, this replaces the earlier inline "role == admin"
// check (exactly the bug class the TS system's requireAdmin/
// requireOwnerOrAdmin were: a login-only check baked into usecase code
// instead of a policy decision). See this service's README "Known gaps"
// for what OPA does/doesn't cover yet.
func requireAdminActor(ctx context.Context, users UserRepository, opa OPAClient) (domain.User, error) {
	actorID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.User{}, apperrors.New(apperrors.KindUnauthenticated, "AUTH_NO_ACTOR", "no authenticated user in request context", nil)
	}
	actor, err := users.GetUserByID(ctx, actorID)
	if err != nil {
		return domain.User{}, apperrors.New(apperrors.KindUnauthenticated, "AUTH_ACTOR_NOT_FOUND", "acting user not found", err)
	}
	allowed, err := opa.Decision(ctx, actor)
	if err != nil {
		// Fail closed: a policy-evaluation error is never treated as an
		// allow, matching every other Epic E call site's fail-closed
		// contract (common/policy.Evaluator's doc comment).
		return domain.User{}, apperrors.New(apperrors.KindInternal, "AUTH_OPA_EVAL_FAILED", "policy evaluation failed", err)
	}
	if !allowed {
		return domain.User{}, apperrors.New(apperrors.KindPermissionDenied, "AUTH_NOT_ADMIN", "admin role required", nil)
	}
	return actor, nil
}
