# TASK-CR-02-06: Add `MarkAnnotationsSent` usecase and `Repository.MarkSent`

**From Solution:** SOL-CR-02
**Priority:** P1
**Service:** `annotation-service`
**File:** `backend-go/services/annotation-service/internal/usecase/mark_annotations_sent.go` (new), `backend-go/services/annotation-service/internal/usecase/ports.go`, `backend-go/services/annotation-service/internal/adapter/postgres/repository.go`, `backend-go/services/annotation-service/internal/adapter/grpc/server.go`
**Depends on:** TASK-CR-02-02, TASK-CR-02-04
**Status:** `[ ]` TODO

---

## Context

SOL-CR-03's send-to-agent flow needs a bulk transition to "sent" after PTY
delivery succeeds. This is a tenant-scoped bulk update, mirroring
`create_annotation.go`'s tenant/actor extraction pattern, backed by one
`UPDATE ... WHERE id = ANY($1)` statement (not N round trips).

## Changes to make

### 1. `internal/usecase/ports.go` — extend `Repository`

```go
type Repository interface {
	// ... existing methods unchanged ...

	// MarkSent transitions every annotation in ids (scoped to tenantID) to
	// SentToAgent=true/SentAt=sentAt in one statement. Any id not found for
	// the tenant is silently skipped, not a hard failure — SOL-CR-03 calls
	// this after PTY injection already succeeded, so a partial id mismatch
	// (e.g. a concurrently-deleted annotation) must not turn a successful
	// send into an error response.
	MarkSent(ctx context.Context, tenantID string, ids []string, sentAt time.Time) ([]domain.Annotation, error)
}
```

(Add `"time"` to the import block if not already present.)

### 2. `internal/usecase/mark_annotations_sent.go` (new)

```go
package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/annotation-service/internal/domain"
)

type MarkAnnotationsSentInput struct {
	IDs []string
}

// MarkAnnotationsSent bulk-transitions a set of annotations to sent-to-agent
// state — used exclusively by api-gateway's annotation.sendToAgent
// composition (SOL-CR-03), never a standalone user action.
type MarkAnnotationsSent struct {
	repo Repository
}

func NewMarkAnnotationsSent(repo Repository) *MarkAnnotationsSent {
	return &MarkAnnotationsSent{repo: repo}
}

func (uc *MarkAnnotationsSent) Execute(ctx context.Context, in MarkAnnotationsSentInput) ([]domain.Annotation, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "ANNOTATION_NO_TENANT", "no tenant in request context", err)
	}
	if len(in.IDs) == 0 {
		return nil, nil
	}
	updated, err := uc.repo.MarkSent(ctx, tenantID, in.IDs, time.Now().UTC())
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "ANNOTATION_MARK_SENT_FAILED", "failed to mark annotations sent", err)
	}
	return updated, nil
}
```

### 3. `internal/adapter/postgres/repository.go` — implement `MarkSent`

```go
func (r *Repository) MarkSent(ctx context.Context, tenantID string, ids []string, sentAt time.Time) ([]domain.Annotation, error) {
	rows, err := r.pool.Query(ctx, `
		UPDATE annotation.annotations
		SET sent_to_agent = true, sent_at = $1
		WHERE tenant_id = $2 AND id = ANY($3)
		RETURNING id, tenant_id, author_id, repo_id, worktree_id, file_path, line, end_line, side, ref,
		          content, original_code, resolved, sent_to_agent, sent_at, request_id, created_at, updated_at
	`, sentAt, tenantID, ids)
	if err != nil {
		return nil, fmt.Errorf("postgres: mark annotations sent: %w", err)
	}
	defer rows.Close()

	var out []domain.Annotation
	for rows.Next() {
		a, err := scanAnnotation(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan marked annotation: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate marked annotation rows: %w", err)
	}
	return out, nil
}
```

(Add `"time"` to this file's imports.)

### 4. `internal/adapter/grpc/server.go` — RPC handler

Add a `MarkAnnotationsSent` handler translating
`MarkAnnotationsSentRequest.ids` to `MarkAnnotationsSentInput.IDs` and the
usecase's `[]domain.Annotation` result to
`MarkAnnotationsSentResponse.annotations`, following the exact pattern the
existing `CreateAnnotation`/`DeleteAnnotation` handlers already use in that
file. Wire `usecase.NewMarkAnnotationsSent(repo)` into the server's
composition root (`cmd/server/main.go`) alongside the other usecases.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/annotation-service
go build ./...
go test ./internal/usecase/... -run TestMarkAnnotationsSent -v
go test ./internal/adapter/postgres/... -run TestMarkSent -v
```

Add `usecase/mark_annotations_sent_test.go`: marks all ids present on the
fake repo, returns the updated set, no error for an empty `IDs` slice.
`adapter/postgres/annotation_repository_test.go`: `MarkSent` updates
exactly the given ids in one query and skips a nonexistent id without
error.
