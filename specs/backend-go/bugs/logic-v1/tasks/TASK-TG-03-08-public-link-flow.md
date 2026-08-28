# TASK-TG-03-08: Public share-link flow (`CreatePublicLink`/`RevokePublicLink`/`ResolvePublicLink`)

**From Solution:** SOL-TG-03
**Priority:** P2
**Service:** `task-service`
**File:** `backend-go/proto/orca/task/v1/task.proto`, `backend-go/services/task-service/internal/usecase/public_link.go` (new)
**Depends on:** TASK-TG-03-05 (migration — `task.task_share_links` table)
**Status:** `[ ]` TODO

---

## Context

Spec: "anonymous read-only access via a random token." `ResolvePublicLink`
is the one RPC in this service meaningfully callable without a JWT — this
task implements the 3 RPCs and their token-handling at the `task-service`
layer only.

**Explicitly flagged, not resolved by this task**: `07-security-architecture.md`'s
AuthN table has no row for an unauthenticated client mechanism — every
listed client mechanism today assumes an authenticated identity.
`api-gateway` needs a new unauthenticated route class
(`GET /v1/tasks/share/{token}`, bypassing normal JWT validation for exactly
this one path) to actually expose this to a browser. That routing change is
a new trust boundary and needs its own design pass against
`07-security-architecture.md`'s AuthN section — it is explicitly **out of
scope for this task**. Land the RPCs/table/usecases here; do not wire
`api-gateway` to them until that separate design pass happens.

## Changes to make

Add to `task.proto`'s `TaskService` service block:

```protobuf
  rpc CreatePublicLink(CreatePublicLinkRequest) returns (CreatePublicLinkResponse); // requires 'manage'; returns the plaintext token ONCE
  rpc RevokePublicLink(RevokePublicLinkRequest) returns (google.protobuf.Empty);
  rpc ResolvePublicLink(ResolvePublicLinkRequest) returns (ResolvePublicLinkResponse); // token -> task_id + read-only grant, no auth required
```

```protobuf
message CreatePublicLinkRequest { string task_id = 1; google.protobuf.Timestamp expires_at = 2; }
message CreatePublicLinkResponse { string id = 1; string token = 2; } // token returned ONCE, plaintext, never again
message RevokePublicLinkRequest { string id = 1; }
message ResolvePublicLinkRequest { string token = 1; }
message ResolvePublicLinkResponse { string task_id = 1; }
```

Create `backend-go/services/task-service/internal/usecase/public_link.go`:

```go
package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type CreatePublicLink struct {
	links             ShareLinkRepository
	resolvePermission *ResolvePermission
}

func NewCreatePublicLink(links ShareLinkRepository, resolvePermission *ResolvePermission) *CreatePublicLink {
	return &CreatePublicLink{links: links, resolvePermission: resolvePermission}
}

// Execute requires 'manage' on taskID, generates a random 256-bit token,
// stores only its SHA-256 hash, and returns the plaintext exactly once —
// same posture as the Dev Server Agent's own bearer token
// (07-security-architecture.md: "hashed at rest... not plaintext").
func (uc *CreatePublicLink) Execute(ctx context.Context, taskID string) (id, token string, err error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return "", "", apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	callerID, _ := tenant.UserID(ctx)
	if _, err := uc.resolvePermission.Execute(ctx, ResolvePermissionInput{TaskID: taskID, UserID: callerID, Action: "manage"}); err != nil {
		return "", "", err
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", apperrors.New(apperrors.KindInternal, "TASK_PUBLIC_LINK_TOKEN_GEN_FAILED", "failed to generate share token", err)
	}
	token = hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(sum[:])

	id, err = uc.links.Create(ctx, tenantID, taskID, tokenHash, callerID)
	if err != nil {
		return "", "", apperrors.New(apperrors.KindInternal, "TASK_PUBLIC_LINK_CREATE_FAILED", "failed to persist share link", err)
	}
	return id, token, nil
}

type ResolvePublicLink struct {
	links ShareLinkRepository
}

func NewResolvePublicLink(links ShareLinkRepository) *ResolvePublicLink {
	return &ResolvePublicLink{links: links}
}

// Execute hashes the incoming token and looks up by token_hash, checking
// revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now()) — this
// does NOT go through domain.ResolveGrant (no subject_id exists for an
// anonymous caller); it's a distinct, deliberately narrower code path that
// never touches the BFS walk.
func (uc *ResolvePublicLink) Execute(ctx context.Context, tenantID, token string) (taskID string, err error) {
	sum := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(sum[:])
	taskID, err = uc.links.ResolveActive(ctx, tenantID, tokenHash)
	if err != nil {
		return "", apperrors.New(apperrors.KindNotFound, "TASK_PUBLIC_LINK_NOT_FOUND", "share link not found, expired, or revoked", err)
	}
	return taskID, nil
}

type RevokePublicLink struct {
	links             ShareLinkRepository
	resolvePermission *ResolvePermission
	tasks             TaskRepository
}

func NewRevokePublicLink(links ShareLinkRepository, resolvePermission *ResolvePermission, tasks TaskRepository) *RevokePublicLink {
	return &RevokePublicLink{links: links, resolvePermission: resolvePermission, tasks: tasks}
}

func (uc *RevokePublicLink) Execute(ctx context.Context, linkID string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	taskID, err := uc.links.TaskIDFor(ctx, tenantID, linkID)
	if err != nil {
		return apperrors.New(apperrors.KindNotFound, "TASK_PUBLIC_LINK_NOT_FOUND", "share link not found", err)
	}
	callerID, _ := tenant.UserID(ctx)
	if _, err := uc.resolvePermission.Execute(ctx, ResolvePermissionInput{TaskID: taskID, UserID: callerID, Action: "manage"}); err != nil {
		return err
	}
	return uc.links.Revoke(ctx, tenantID, linkID)
}

var _ = uuid.NewString // placeholder import guard if unused elsewhere in this file
```

Add a `ShareLinkRepository` port to `ports.go`:

```go
type ShareLinkRepository interface {
	Create(ctx context.Context, tenantID, taskID, tokenHash, createdBy string) (id string, err error)
	ResolveActive(ctx context.Context, tenantID, tokenHash string) (taskID string, err error)
	Revoke(ctx context.Context, tenantID, linkID string) error
	TaskIDFor(ctx context.Context, tenantID, linkID string) (taskID string, err error)
}
```

Implement it in `backend-go/services/task-service/internal/adapter/postgres/share_links.go`
(new) against `task.task_share_links` from `TASK-TG-03-05`'s migration —
`ResolveActive`'s query is exactly `WHERE tenant_id = $1 AND token_hash =
$2 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now())`.

Wire the 3 usecases into `cmd/server/main.go` and `server.go` following
this file's existing handler pattern. **Do not** add any `api-gateway`
route for `ResolvePublicLink` in this task — see the Context section above.

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
go build ./...
go test ./services/task-service/internal/usecase/... -run 'TestCreatePublicLink|TestResolvePublicLink|TestRevokePublicLink' -v
go test ./services/task-service/internal/adapter/postgres/... -run TestShareLinks -v
```

Expected: `ResolvePublicLink` on a revoked/expired token returns not-found,
never a stale grant; a test asserts `task_share_links` never contains the
plaintext token (only its SHA-256 hash); `CreatePublicLink` requires
`manage` on the target task before issuing a link.
