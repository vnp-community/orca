# TASK-AUTH-02-05: `Login.Execute` sets the new session's `IP`/`UserAgent`

**From Solution:** SOL-AUTH-02
**Priority:** P0
**Service:** `auth-service` (usecase)
**File:** `backend-go/services/auth-service/internal/usecase/login.go`
**Depends on:** TASK-AUTH-02-02
**Status:** `[ ]` TODO

---

## Context

`LoginInput` already carries `IP`/`UserAgent` (added by SOL-AUTH-01/TASK-AUTH-01-02, since `login.fail` audit needs them too). This task is the small remaining piece: pass them into the newly created `domain.Session` via `WithClientInfo` before `CreateSession`.

If TASK-AUTH-01-02 has not landed yet in this branch, `LoginInput` will not yet have `IP`/`UserAgent` fields — add them here first (same shape as SOL-AUTH-01's task) before proceeding with the change below.

## Changes to make

In `backend-go/services/auth-service/internal/usecase/login.go`, inside `Execute`, change:

```go
session, err := domain.NewSession(domain.HashSessionToken(rawToken), user.ID, user.TenantID, now, now.Add(uc.sessionTTL))
if err != nil {
	return LoginOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_INVALID_SESSION", err.Error(), err)
}
if err := uc.sessions.CreateSession(ctx, session); err != nil {
	return LoginOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_SESSION_CREATE_FAILED", "failed to create session", err)
}
```

to:

```go
session, err := domain.NewSession(domain.HashSessionToken(rawToken), user.ID, user.TenantID, now, now.Add(uc.sessionTTL))
if err != nil {
	return LoginOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_INVALID_SESSION", err.Error(), err)
}
session = session.WithClientInfo(in.IP, in.UserAgent)
if err := uc.sessions.CreateSession(ctx, session); err != nil {
	return LoginOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_SESSION_CREATE_FAILED", "failed to create session", err)
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/auth-service/...
go test ./services/auth-service/internal/usecase/... -run TestLogin -v
```

Expected: a fake `SessionRepository.CreateSession` call in `login_test.go` receives a `domain.Session` whose `IP`/`UserAgent` match `LoginInput.IP`/`.UserAgent` when set.
