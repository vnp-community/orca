package usecase

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

type UpdateUserInput struct {
	UserID string
	Email  *string // nil = leave unchanged
	Name   *string
	Role   *domain.Role
}

// UpdateUser is an admin-console operation: a real partial update of
// email/name/role, distinct from the narrower UpdateUserRole (kept as-is).
type UpdateUser struct {
	users UserRepository
	audit AuditRepository
	clock Clock
	opa   OPAClient
}

func NewUpdateUser(users UserRepository, audit AuditRepository, clock Clock, opa OPAClient) *UpdateUser {
	return &UpdateUser{users: users, audit: audit, clock: clock, opa: opa}
}

func (uc *UpdateUser) Execute(ctx context.Context, in UpdateUserInput) (domain.User, error) {
	actor, err := requireAdminActor(ctx, uc.users, uc.opa)
	if err != nil {
		return domain.User{}, err
	}
	if in.Email != nil && !strings.Contains(*in.Email, "@") {
		return domain.User{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_INVALID_EMAIL", "email must contain '@'", nil)
	}
	if in.Role != nil && !in.Role.Valid() {
		return domain.User{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_INVALID_ROLE", "invalid role", nil)
	}
	user, err := uc.users.UpdateUser(ctx, in.UserID, in.Email, in.Name, in.Role)
	if err != nil {
		return domain.User{}, apperrors.New(apperrors.KindInternal, "AUTH_UPDATE_USER_FAILED", "failed to update user", err)
	}
	// "user.updated", distinct from update_user_role.go's "user.role_updated"
	// when role also changes — SOL-AUTH-05's metadata field is where the
	// before/after diff eventually lands; a bare Target-only entry for now.
	if entry, err := domain.NewAuditEntry(uuid.NewString(), user.TenantID, actor.ID, "user.updated", "user", user.ID, map[string]any{}, "", uc.clock.Now()); err == nil {
		_ = uc.audit.Append(ctx, entry)
	}
	return user, nil
}
