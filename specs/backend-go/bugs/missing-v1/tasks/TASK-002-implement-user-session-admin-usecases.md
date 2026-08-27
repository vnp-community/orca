# TASK-002: Implement user deactivate/reactivate + session-admin usecases

**From Solution:** SOL-001
**Priority:** P0
**Service:** `auth-service`
**File:** `services/auth-service/internal/usecase/deactivate_user.go` (new), `reactivate_user.go` (new), `list_sessions_for_user.go` (new), `force_revoke_all_sessions.go` (new), plus repository interface + Postgres impl
**Depends on:** TASK-001
**Status:** `[x]` DONE — `DeactivateUser`/`ReactivateUser`/`ListSessionsForUser`/`ForceRevokeAllSessionsForUser` usecases, their repository ports, Postgres impls, and gRPC handlers all exist and pass `go build`/`go vet`.

---

## Context

Per `03-clean-architecture-guidelines.md`, usecases depend on repository
*ports* (interfaces), never a concrete Postgres type. Add the port methods
first, then the usecases, then the Postgres implementation.

## Changes to make

### 1. Extend `UserRepository` port (`internal/usecase/ports.go`)

```go
type UserRepository interface {
    // ... existing methods ...
    SetActive(ctx context.Context, userID string, active bool) error
}

type SessionRepository interface {
    // ... existing methods ...
    ListForUser(ctx context.Context, userID string) ([]domain.Session, error)
    RevokeAllForUser(ctx context.Context, userID string) (int, error)
}
```

### 2. Usecases

```go
// internal/usecase/deactivate_user.go
func (uc *UserUseCase) DeactivateUser(ctx context.Context, userID string) (*domain.User, error) {
    if err := uc.repo.SetActive(ctx, userID, false); err != nil {
        return nil, err
    }
    return uc.repo.Get(ctx, userID)
}

// internal/usecase/reactivate_user.go
func (uc *UserUseCase) ReactivateUser(ctx context.Context, userID string) (*domain.User, error) {
    if err := uc.repo.SetActive(ctx, userID, true); err != nil {
        return nil, err
    }
    return uc.repo.Get(ctx, userID)
}

// internal/usecase/list_sessions_for_user.go
func (uc *SessionUseCase) ListSessionsForUser(ctx context.Context, userID string) ([]domain.Session, error) {
    return uc.repo.ListForUser(ctx, userID)
}

// internal/usecase/force_revoke_all_sessions.go
func (uc *SessionUseCase) ForceRevokeAllSessionsForUser(ctx context.Context, userID string) (int, error) {
    return uc.repo.RevokeAllForUser(ctx, userID)
}
```

### 3. Postgres impl (`internal/adapter/postgres/repository.go`)

```go
func (r *Repository) SetActive(ctx context.Context, userID string, active bool) error {
    _, err := r.db.ExecContext(ctx, `UPDATE auth.users SET is_active = $1 WHERE id = $2`, active, userID)
    return err
}

func (r *Repository) ListForUser(ctx context.Context, userID string) ([]domain.Session, error) {
    rows, err := r.db.QueryContext(ctx, `SELECT id, user_id, created_at, expires_at, last_seen_at, ip, user_agent
        FROM auth.sessions WHERE user_id = $1 ORDER BY created_at DESC`, userID)
    // ... scan into []domain.Session ...
}

func (r *Repository) RevokeAllForUser(ctx context.Context, userID string) (int, error) {
    res, err := r.db.ExecContext(ctx, `DELETE FROM auth.sessions WHERE user_id = $1`, userID)
    if err != nil {
        return 0, err
    }
    n, _ := res.RowsAffected()
    return int(n), nil
}
```

### 4. gRPC server wiring (`internal/adapter/grpc/server.go`)

Add the 4 new RPC handler methods, translating request/response to/from
the usecase calls above — follow the exact pattern the existing
`DeactivateUser`-adjacent handlers (`CreateUser`, `RevokeSession`) already
use in that file.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/auth-service
go build ./...
go vet ./...
```
