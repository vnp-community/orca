# TASK-AUTH-04-03: `ListSessions` usecase + `SessionRepository.ListForTenant` (postgres)

**From Solution:** SOL-AUTH-04
**Priority:** P1
**Service:** `auth-service` (usecase + postgres)
**File:** `backend-go/services/auth-service/internal/usecase/list_sessions.go` (new), `backend-go/services/auth-service/internal/usecase/ports.go`, `backend-go/services/auth-service/internal/adapter/postgres/session_repository.go`
**Depends on:** TASK-AUTH-04-01
**Status:** `[ ]` TODO

---

## Context

The admin dashboard's "active sessions across all users" view has no backing RPC — only `ListSessionsForUser` (single-user scope) exists. This task adds the tenant-scoped cross-user usecase and its postgres query. Scoping to `actor.TenantID` (the resolved admin's own tenant), never a caller-supplied `tenant_id`, follows `07-security-architecture.md`'s multi-tenancy isolation layer 2.

## Changes to make

Add to `SessionRepository` in `backend-go/services/auth-service/internal/usecase/ports.go`:

```go
// ListForTenant returns a page of sessions for tenantID joined with each
// session's owning user's email (denormalized to avoid an N+1 lookup in
// the admin dashboard).
ListForTenant(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.SessionWithUser, string, error)
```

(Add a matching `domain.SessionWithUser{ Session domain.Session; UserEmail string }` type in `internal/domain/session.go` if one doesn't exist.)

Create `backend-go/services/auth-service/internal/usecase/list_sessions.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type ListSessionsInput struct {
	PageToken string
	PageSize  int32
}

type ListSessionsOutput struct {
	Sessions      []domain.SessionWithUser
	NextPageToken string
}

type ListSessions struct {
	users    UserRepository
	sessions SessionRepository
	opa      OPAClient
}

func NewListSessions(users UserRepository, sessions SessionRepository, opa OPAClient) *ListSessions {
	return &ListSessions{users: users, sessions: sessions, opa: opa}
}

func (uc *ListSessions) Execute(ctx context.Context, in ListSessionsInput) (ListSessionsOutput, error) {
	actor, err := requireAdminActor(ctx, uc.users, uc.opa)
	if err != nil {
		return ListSessionsOutput{}, err
	}
	// actor.TenantID, never a caller-supplied tenant_id — see Context above.
	rows, next, err := uc.sessions.ListForTenant(ctx, actor.TenantID, in.PageToken, in.PageSize)
	if err != nil {
		return ListSessionsOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_LIST_SESSIONS_FAILED", "failed to list sessions", err)
	}
	return ListSessionsOutput{Sessions: rows, NextPageToken: next}, nil
}
```

Add `"github.com/stablyai/orca-go/services/auth-service/internal/domain"` import as needed.

Implement in `backend-go/services/auth-service/internal/adapter/postgres/session_repository.go`:

```go
func (r *Repository) ListForTenant(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.SessionWithUser, string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.token_hash, s.user_id, s.tenant_id, s.created_at, s.expires_at,
		       s.revoked_at, s.last_seen_at, COALESCE(s.ip::text, ''), COALESCE(s.user_agent, ''), u.email
		FROM auth.sessions s
		JOIN auth.users u ON u.id = s.user_id
		WHERE s.tenant_id = $1 AND s.token_hash > $2
		ORDER BY s.token_hash
		LIMIT $3
	`, tenantID, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: query sessions for tenant: %w", err)
	}
	defer rows.Close()

	var out []domain.SessionWithUser
	for rows.Next() {
		var sw domain.SessionWithUser
		if err := rows.Scan(&sw.Session.TokenHash, &sw.Session.UserID, &sw.Session.TenantID,
			&sw.Session.CreatedAt, &sw.Session.ExpiresAt, &sw.Session.RevokedAt,
			&sw.Session.LastSeenAt, &sw.Session.IP, &sw.Session.UserAgent, &sw.UserEmail); err != nil {
			return nil, "", fmt.Errorf("postgres: scan session-with-user row: %w", err)
		}
		out = append(out, sw)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("postgres: iterate session-with-user rows: %w", err)
	}

	next := ""
	if int32(len(out)) == pageSize && len(out) > 0 {
		next = out[len(out)-1].Session.TokenHash
	}
	return out, next, nil
}
```

Note: `last_seen_at`/`ip`/`user_agent` columns depend on SOL-AUTH-02's migration (TASK-AUTH-02-01) — if that hasn't landed yet in this branch, these columns don't exist; this task's `ListForTenant` query then needs those three `SELECT` columns and `Scan` targets removed until SOL-AUTH-02 lands (sequencing note, not a hard blocker).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/auth-service/...
go test ./services/auth-service/internal/usecase/... -run TestListSessions -v
go test ./services/auth-service/internal/adapter/postgres/... -run TestSessionRepository_ListForTenant -v
```

Expected: fake `SessionRepository.ListForTenant` scoped to `actor.TenantID` even when a caller passes a different `tenant_id` in the request (assert the fake receives the actor's tenant, not the request's); non-admin actor → `PermissionDenied`, `ListForTenant` never called; postgres integration test confirms tenant-scoping (a session belonging to a different tenant never appears in results) and pagination.
