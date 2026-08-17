package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// requireAdminActor resolves the acting user from context and checks its
// role is admin.
//
// This is a placeholder, not the intended long-term mechanism: per
// auth-service.md §9, every admin-console usecase method should call the
// embedded OPA SDK with the caller's resolved role/claims as input, the
// same mechanism every other service uses for its own fine-grained checks
// — a simple "if role == admin" check here is exactly the bug class
// (TS system's requireAdmin/requireOwnerOrAdmin) that design is meant to
// close structurally. Replace this with the OPA integration described in
// auth-service.md §6/§9 before production use; see this service's README
// "Known gaps".
func requireAdminActor(ctx context.Context, users UserRepository) (domain.User, error) {
	actorID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.User{}, apperrors.New(apperrors.KindUnauthenticated, "AUTH_NO_ACTOR", "no authenticated user in request context", nil)
	}
	actor, err := users.GetUserByID(ctx, actorID)
	if err != nil {
		return domain.User{}, apperrors.New(apperrors.KindUnauthenticated, "AUTH_ACTOR_NOT_FOUND", "acting user not found", err)
	}
	if actor.Role != domain.RoleAdmin {
		return domain.User{}, apperrors.New(apperrors.KindPermissionDenied, "AUTH_NOT_ADMIN", "admin role required", nil)
	}
	return actor, nil
}
