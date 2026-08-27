# TASK-AUTH-04-02: `CreateUser.Execute` hashes the admin-supplied password instead of discarding a random one

**From Solution:** SOL-AUTH-04
**Priority:** P0
**Service:** `auth-service` (usecase)
**File:** `backend-go/services/auth-service/internal/usecase/create_user.go`
**Depends on:** TASK-AUTH-04-01
**Status:** `[ ]` TODO

---

## Context

`CreateUser.Execute` currently generates a random 24-char password, hashes it, stores the hash, and discards the plaintext — the created account has no way to ever log in. This task makes it use `CreateUserInput.Password` instead, with a minimal length floor mirroring SOL-AUTH-01's login-side format check.

## Changes to make

In `backend-go/services/auth-service/internal/usecase/create_user.go`, change `CreateUserInput`:

```go
// CreateUserInput mirrors CreateUserRequest 1:1.
type CreateUserInput struct {
	Email    string
	Name     string
	TenantID string
	Password string
	Role     domain.Role
}
```

Replace the password-generation block inside `Execute`:

```go
func (uc *CreateUser) Execute(ctx context.Context, in CreateUserInput) (domain.User, error) {
	actor, err := requireAdminActor(ctx, uc.users, uc.opa)
	if err != nil {
		return domain.User{}, err
	}

	if in.Role == "" {
		in.Role = domain.RoleUser
	}

	if len(in.Password) < 8 { // mirrors SOL-AUTH-01's login-side format floor
		return domain.User{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_WEAK_PASSWORD", "password must be at least 8 characters", nil)
	}
	passwordHash, err := uc.hasher.Hash(in.Password)
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
```

Remove the now-unused `generateRandomToken(24)` call and its surrounding doc comment (update the doc comment above `CreateUser` to no longer say "there is no invite/reset-link flow... a random, never-returned password is generated"). Do not remove `generateRandomToken` itself if `login.go`/`bootstrap.go` still use it elsewhere.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/auth-service/...
go test ./services/auth-service/internal/usecase/... -run TestCreateUser -v
```

Expected: `Password` too short → `AUTH_WEAK_PASSWORD`, no DB write; valid password → `PasswordHasher.Hash` called with the exact plaintext, stored hash verifiable via `PasswordHasher.Compare`; regression — the prior random-24-char-generate-and-discard code path is fully removed (assert the created user's password round-trips exactly what was passed in).
