# TASK-AUTH-04-04: `UpdateUser` usecase + `UserRepository.UpdateUser` (postgres)

**From Solution:** SOL-AUTH-04
**Priority:** P1
**Service:** `auth-service` (usecase + postgres)
**File:** `backend-go/services/auth-service/internal/usecase/update_user.go` (new), `backend-go/services/auth-service/internal/usecase/ports.go`, `backend-go/services/auth-service/internal/adapter/postgres/user_repository.go`
**Depends on:** TASK-AUTH-04-01
**Status:** `[ ]` TODO

---

## Context

`auth-service.md:98` already names `UpdateUser (email, role, profile fields...)` in the "Admin — users" RPC group, but only the narrower `UpdateUserRole` exists today. This adds a real partial update of email/name/role, using `*string`/`*domain.Role` to distinguish "leave unchanged" from "set". `UpdateUserRole` is kept as-is, not deprecated.

## Changes to make

Add to `UserRepository` in `backend-go/services/auth-service/internal/usecase/ports.go`:

```go
// UpdateUser applies a partial update — nil fields are left unchanged
// (COALESCE semantics at the SQL layer). Distinct from UpdateUserRole
// (kept as-is, a narrower single-field update).
UpdateUser(ctx context.Context, userID string, email, name *string, role *domain.Role) (domain.User, error)
```

Create `backend-go/services/auth-service/internal/usecase/update_user.go`:

```go
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
	if entry, err := domain.NewAuditEntry(uuid.NewString(), user.TenantID, actor.ID, "user.updated", user.ID, uc.clock.Now()); err == nil {
		_ = uc.audit.Append(ctx, entry)
	}
	return user, nil
}
```

Implement in `backend-go/services/auth-service/internal/adapter/postgres/user_repository.go`:

```go
func (r *Repository) UpdateUser(ctx context.Context, userID string, email, name *string, role *domain.Role) (domain.User, error) {
	var roleStr *string
	if role != nil {
		s := string(*role)
		roleStr = &s
	}
	row := r.pool.QueryRow(ctx, `
		UPDATE auth.users
		SET email = COALESCE($2, email), name = COALESCE($3, name), role = COALESCE($4, role)
		WHERE id = $1
		RETURNING id, tenant_id, email, name, role, is_active, created_at
	`, userID, email, name, roleStr)

	var u domain.User
	err := row.Scan(&u.ID, &u.TenantID, &u.Email, &u.Name, &u.Role, &u.IsActive, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, fmt.Errorf("postgres: update user: %w", usecase.ErrUserNotFound)
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("postgres: scan updated user row: %w", err)
	}
	return u, nil
}
```

Match the existing `ErrUserNotFound`-on-`pgx.ErrNoRows` pattern already used by `UpdateUserRole` in the same file.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/auth-service/...
go test ./services/auth-service/internal/usecase/... -run TestUpdateUser -v
go test ./services/auth-service/internal/adapter/postgres/... -run TestUserRepository_UpdateUser -v
```

Expected: partial update (`Email` set, `Name`/`Role` nil) → repository called with `nil` for the untouched fields, existing values preserved; invalid email/role → `KindInvalidArgument` before any DB call; postgres integration test confirms `COALESCE` semantics — updating only `email` leaves `name`/`role` unchanged in the DB.
