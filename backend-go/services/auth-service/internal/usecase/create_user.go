package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// CreateUserInput mirrors CreateUserRequest 1:1. Note there is no password
// field — see CreateUser.Execute's doc comment for how this scaffold
// handles that.
type CreateUserInput struct {
	Email    string
	Name     string
	TenantID string
	Role     domain.Role
}

// CreateUser is an admin-console operation: creates a user with a bcrypt
// password hash (cost >= 12, enforced by adapter/bcrypt).
type CreateUser struct {
	users  UserRepository
	audit  AuditRepository
	hasher PasswordHasher
	clock  Clock
	opa    OPAClient
}

func NewCreateUser(users UserRepository, audit AuditRepository, hasher PasswordHasher, clock Clock, opa OPAClient) *CreateUser {
	return &CreateUser{users: users, audit: audit, hasher: hasher, clock: clock, opa: opa}
}

func (uc *CreateUser) Execute(ctx context.Context, in CreateUserInput) (domain.User, error) {
	actor, err := requireAdminActor(ctx, uc.users, uc.opa)
	if err != nil {
		return domain.User{}, err
	}

	if in.Role == "" {
		in.Role = domain.RoleUser
	}

	// CreateUserRequest (see proto/orca/auth/v1/auth.proto) carries no
	// password field — there is no invite/reset-link flow implemented in
	// this scaffold to hand a chosen password to the new user (see
	// README "Known gaps": SSO/first-run/invite flows are out of scope
	// here). A random, never-returned password is generated and hashed so
	// the account is unusable until a real credential-issuance flow (an
	// invite email, a forced first-login password reset, or SSO) is wired.
	randomPassword, err := generateRandomToken(24)
	if err != nil {
		return domain.User{}, apperrors.New(apperrors.KindInternal, "AUTH_PASSWORD_GEN_FAILED", "failed to generate initial password", err)
	}
	passwordHash, err := uc.hasher.Hash(randomPassword)
	if err != nil {
		return domain.User{}, apperrors.New(apperrors.KindInternal, "AUTH_PASSWORD_HASH_FAILED", "failed to hash initial password", err)
	}

	now := uc.clock.Now()
	user, err := domain.NewUser(uuid.NewString(), in.TenantID, in.Email, in.Name, in.Role, true, now)
	if err != nil {
		return domain.User{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_INVALID_USER", err.Error(), err)
	}

	created, err := uc.users.CreateUser(ctx, user, passwordHash)
	if errors.Is(err, ErrUserAlreadyExists) {
		return domain.User{}, apperrors.New(apperrors.KindAlreadyExists, "AUTH_USER_EXISTS", "a user with this email already exists in this tenant", err)
	}
	if err != nil {
		return domain.User{}, apperrors.New(apperrors.KindInternal, "AUTH_USER_CREATE_FAILED", "failed to create user", err)
	}

	if entry, err := domain.NewAuditEntry(uuid.NewString(), created.TenantID, actor.ID, "user.created", created.ID, now); err == nil {
		_ = uc.audit.Append(ctx, entry)
	}

	return created, nil
}
