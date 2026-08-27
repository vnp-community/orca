package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// UpdateUserRole is an admin-console operation.
type UpdateUserRole struct {
	users UserRepository
	audit AuditRepository
	clock Clock
	opa   OPAClient
}

func NewUpdateUserRole(users UserRepository, audit AuditRepository, clock Clock, opa OPAClient) *UpdateUserRole {
	return &UpdateUserRole{users: users, audit: audit, clock: clock, opa: opa}
}

func (uc *UpdateUserRole) Execute(ctx context.Context, userID string, role domain.Role) (domain.User, error) {
	actor, err := requireAdminActor(ctx, uc.users, uc.opa)
	if err != nil {
		return domain.User{}, err
	}
	if !role.Valid() {
		return domain.User{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_INVALID_ROLE", "invalid role", nil)
	}

	before, err := uc.users.GetUserByID(ctx, userID) // capture old role before mutating
	oldRole := ""
	if err == nil {
		oldRole = string(before.Role)
	}

	updated, err := uc.users.UpdateUserRole(ctx, userID, role)
	if errors.Is(err, ErrUserNotFound) {
		return domain.User{}, apperrors.New(apperrors.KindNotFound, "AUTH_USER_NOT_FOUND", "user not found", err)
	}
	if err != nil {
		return domain.User{}, apperrors.New(apperrors.KindInternal, "AUTH_UPDATE_ROLE_FAILED", "failed to update user role", err)
	}

	now := uc.clock.Now()
	if entry, err := domain.NewAuditEntry(uuid.NewString(), updated.TenantID, actor.ID, "user.role_updated",
		"user", updated.ID, map[string]any{"from": oldRole, "to": string(role)}, "", now); err == nil {
		_ = uc.audit.Append(ctx, entry)
	}

	return updated, nil
}
