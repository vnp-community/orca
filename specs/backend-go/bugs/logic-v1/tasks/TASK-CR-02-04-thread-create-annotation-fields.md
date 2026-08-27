# TASK-CR-02-04: Thread side/range/worktree/original-code through `CreateAnnotation` and the Postgres repository

**From Solution:** SOL-CR-02
**Priority:** P0
**Service:** `annotation-service`
**File:** `backend-go/services/annotation-service/internal/usecase/create_annotation.go`, `backend-go/services/annotation-service/internal/adapter/postgres/repository.go`
**Depends on:** TASK-CR-02-02, TASK-CR-02-03
**Status:** `[ ]` TODO

---

## Context

`CreateAnnotationInput` mirrors the gRPC request 1:1 today (no
`WorktreeID`/`EndLine`/`Side`/`OriginalCode`); the Postgres repository's
`CreateAnnotation`/`scanAnnotation`/`ListAnnotations`/`GetAnnotation`/
`UpdateAnnotation` SQL only reads/writes the pre-TASK-CR-02-02 columns. Both
need the new columns threaded through together so `CreateAnnotation`'s
usecase test can actually round-trip the new fields via the fake/real repo.

## Changes to make

### 1. `internal/usecase/create_annotation.go`

```go
type CreateAnnotationInput struct {
	RepoID       string
	WorktreeID   string // NEW
	FilePath     string
	Line         int32
	EndLine      int32 // NEW
	Side         domain.Side // NEW
	Ref          string
	Content      string
	OriginalCode string // NEW
	RequestID    string
}
```

In `Execute`, update the `NewAnchor`/`NewAnnotation` calls to the new
signatures from TASK-CR-02-02:

```go
	anchor, err := domain.NewAnchor(in.RepoID, in.WorktreeID, in.FilePath, in.Line, in.EndLine, in.Side, in.Ref)
	if err != nil {
		return domain.Annotation{}, apperrors.New(apperrors.KindInvalidArgument, "ANNOTATION_INVALID_ANCHOR", err.Error(), err)
	}

	// ... idempotency check unchanged ...

	now := time.Now().UTC()
	annotation, err := domain.NewAnnotation(uuid.NewString(), tenantID, authorID, anchor, in.Content, in.OriginalCode, false, in.RequestID, now, now)
```

### 2. `internal/adapter/postgres/repository.go`

Update `CreateAnnotation`'s INSERT, `scanAnnotation`, `ListAnnotations`,
`GetAnnotation`, and `UpdateAnnotation`'s SELECT/RETURNING column lists to
include the six new columns:

```go
func (r *Repository) CreateAnnotation(ctx context.Context, a domain.Annotation) (domain.Annotation, error) {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO annotation.annotations (
			id, tenant_id, author_id, repo_id, worktree_id, file_path, line, end_line, side, ref,
			content, original_code, resolved, sent_to_agent, sent_at, request_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
	`,
		a.ID, a.TenantID, a.AuthorID, a.Anchor.RepoID, nullString(a.Anchor.WorktreeID), a.Anchor.FilePath, a.Anchor.Line, a.Anchor.EndLine, int32(a.Anchor.Side), a.Anchor.Ref,
		a.Content, a.OriginalCode, a.Resolved, a.SentToAgent, a.SentAt, a.RequestID, a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return domain.Annotation{}, fmt.Errorf("postgres: insert annotation: %w", err)
	}
	return a, nil
}

// nullString converts "" to a NULL-capable pgx bind for the nullable
// worktree_id column — an empty WorktreeID means "not worktree-scoped",
// which must round-trip as SQL NULL, not the string "".
func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
```

Update `scanAnnotation` to scan the new columns (order must match every
SELECT/RETURNING column list below):

```go
func scanAnnotation(row rowScanner) (domain.Annotation, error) {
	var a domain.Annotation
	var worktreeID *string
	var side int32
	if err := row.Scan(
		&a.ID, &a.TenantID, &a.AuthorID, &a.Anchor.RepoID, &worktreeID, &a.Anchor.FilePath, &a.Anchor.Line, &a.Anchor.EndLine, &side, &a.Anchor.Ref,
		&a.Content, &a.OriginalCode, &a.Resolved, &a.SentToAgent, &a.SentAt, &a.RequestID, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return domain.Annotation{}, err
	}
	if worktreeID != nil {
		a.Anchor.WorktreeID = *worktreeID
	}
	a.Anchor.Side = domain.Side(side)
	return a, nil
}
```

Update the column list in `FindByRequestID`, `ListAnnotations`, and
`GetAnnotation`'s `SELECT` clauses, and `UpdateAnnotation`'s `RETURNING`
clause, to the same 18-column order as `CreateAnnotation`'s INSERT above
(content/resolved unchanged by `UpdateAnnotation`'s `SET`, only the
selected/returned columns grow).

## Verify

```bash
cd /opt/repos/orca/backend-go/services/annotation-service
go build ./...
go test ./internal/usecase/... -run TestCreateAnnotation -v
go test ./internal/adapter/postgres/... -v
```

Add a case to `usecase/create_annotation_test.go` asserting `WorktreeID`,
`EndLine`, `Side`, and `OriginalCode` round-trip through to the fake
repository's persisted `domain.Annotation` unchanged.
