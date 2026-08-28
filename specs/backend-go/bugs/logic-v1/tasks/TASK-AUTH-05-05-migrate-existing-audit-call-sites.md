# TASK-AUTH-05-05: Update every existing `NewAuditEntry` call site to the new signature

**From Solution:** SOL-AUTH-05
**Priority:** P0 — the auth-service module does not compile until this lands
**Service:** `auth-service` (usecase)
**File:** `backend-go/services/auth-service/internal/usecase/login.go`, `logout.go`, `create_user.go`, `deactivate_user.go`, `reactivate_user.go`, `update_user_role.go`, `revoke_session.go`, `force_revoke_all_sessions.go`, `bootstrap.go`
**Depends on:** TASK-AUTH-05-02
**Status:** `[x]` DONE — updated all 9 listed call sites plus `update_user.go` (new since this table was written, TASK-AUTH-04-04); corrected `logout.go`/`revoke_session.go` to target `"session"`/tokenHash per the table (previously targeted the user, a pre-existing inconsistency); full `auth-service` build + `go test -race ./services/auth-service/...` green.

---

## Context

`domain.NewAuditEntry`'s signature changed from `(id, tenantID, actorID, action, target string, occurredAt time.Time)` to `(id, tenantID, actorID, action, targetType, targetID string, metadata map[string]any, ipAddress string, occurredAt time.Time)` in TASK-AUTH-05-02. Every current call follows the old shape `domain.NewAuditEntry(uuid.NewString(), tenantID, actorID, "action.name", targetID, now)` — this task updates all nine of them mechanically per the table below. `update_user_role.go` is the one call site that needed a real, structured `Metadata` payload the old single-`Target` string could never carry — the clearest concrete illustration of why this migration matters, not just a rename.

## Changes to make

| Call site | Old `target` | New `targetType`/`targetID` | New `metadata` |
|---|---|---|---|
| `login.go` (`user.login`, success path) | `user.ID` | `"user"`, `user.ID` | `map[string]any{"ip": user's login IP, "userAgent": ...}` if `LoginInput.IP`/`.UserAgent` exist in this branch (SOL-AUTH-01), else `map[string]any{}` |
| `login.go` (`login.fail`, if TASK-AUTH-01-02 landed) | `in.Email` | `"user"`, best-effort resolved user ID or `""` | `map[string]any{"ip": in.IP, "email": in.Email, "reason": reason}` |
| `logout.go` (`user.logout`) | session-derived user ID | `"session"`, token hash | `map[string]any{}` |
| `create_user.go` (`user.created`) | new user's ID | `"user"`, new user's ID | `map[string]any{"targetEmail": created.Email, "role": string(created.Role)}` |
| `deactivate_user.go` (`user.deactivated`) | user ID | `"user"`, user ID | `map[string]any{}` |
| `reactivate_user.go` (`user.reactivated`) | user ID | `"user"`, user ID | `map[string]any{}` |
| `update_user_role.go` (`user.role_updated`) | user ID | `"user"`, user ID | `map[string]any{"from": string(oldRole), "to": string(role)}` — capture `oldRole` from the pre-update user fetch before calling `UpdateUserRole` |
| `revoke_session.go` (`session.revoked`) | token hash | `"session"`, token hash | `map[string]any{}` |
| `force_revoke_all_sessions.go` (`session.force_revoke_all`) | user ID | `"user"`, user ID | `map[string]any{"revokedCount": n}` |
| `bootstrap.go` (`user.bootstrap_created`) | new admin's ID | `"user"`, new admin's ID | `map[string]any{}` |

Every call site's `NewAuditEntry(...)` invocation changes shape from:

```go
domain.NewAuditEntry(uuid.NewString(), tenantID, actorID, "action.name", targetID, now)
```

to:

```go
domain.NewAuditEntry(uuid.NewString(), tenantID, actorID, "action.name", "targetType", targetID, metadata, ipAddress, now)
```

using each row's `targetType`/`metadata` above; `ipAddress` is `""` for every call site except `login.go`'s two entries (use `in.IP` there if that field exists in this branch, else `""`).

Example — `update_user_role.go`, the one call site needing a real payload:

```go
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
```

Apply the analogous minimal-diff change to each of the other eight files per the table.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/auth-service/...
go test ./services/auth-service/internal/usecase/... -v
```

Expected: full `auth-service` build succeeds; per-call-site regression test for each row in the table above — e.g. `update_user_role_test.go` asserts the appended entry's `Metadata["from"]`/`Metadata["to"]` match the actual role transition; every other call site's fake `AuditRepository` receives the documented `TargetType`/`TargetID`.
