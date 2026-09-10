# TASK-CLI-01-02: `project-service` — `idempotency_key` column + `FindWorktreeByIdempotencyKey`

**From Solution:** SOL-CLI-01
**Priority:** P0 — `git-gateway-service`'s idempotency check (TASK-CLI-01-03) depends on this lookup existing
**Service:** `project-service`
**File:** `backend-go/services/project-service/migrations/0010_worktree_idempotency_key.up.sql`
**Depends on:** none (proto-independent — this is project-service's own storage + RPC surface)
**Status:** [x] DONE — migration `0009_worktree_idempotency_key.{up,down}.sql` (repo had no lineage migrations yet, so this is 0009 not 0010 as drafted), domain/repo/usecase/RPC surface added; `TestWorktreeRepository_FindWorktreeByIdempotencyKey_{RoundTrips,NoMatchReturnsFoundFalse}` pass against a real testcontainers Postgres.

---

## Context

BR-CLI-01's dedupe check needs somewhere to look up "has this `(project_id, idempotency_key)` pair already produced a worktree." This task adds the column, the repository method, the usecase-layer port, and the RPC that exposes it — following the exact pattern `0009_worktree_lineage.up.sql` and `worktree_repository.go`'s existing methods already establish.

## Changes to make

**1. New migration** `backend-go/services/project-service/migrations/0010_worktree_idempotency_key.up.sql`:

```sql
ALTER TABLE project.worktrees ADD COLUMN idempotency_key TEXT;

-- Unique per project, not global — the same idempotency_key value is
-- meaningless across two different projects (orca-cli scopes its default
-- key as sha256(project_id|repo_id|branch), which already includes
-- project_id, but a caller-supplied custom key might not).
CREATE UNIQUE INDEX worktrees_project_idempotency_key_idx
  ON project.worktrees (project_id, idempotency_key)
  WHERE idempotency_key IS NOT NULL;
```

`backend-go/services/project-service/migrations/0010_worktree_idempotency_key.down.sql`:

```sql
DROP INDEX IF EXISTS project.worktrees_project_idempotency_key_idx;
ALTER TABLE project.worktrees DROP COLUMN IF EXISTS idempotency_key;
```

**2. `domain.Worktree`** (`backend-go/services/project-service/internal/domain/worktree.go`) — add the field and thread it through `NewWorktree`/`WorktreeLineageCapture` is NOT the right place (idempotency key is not lineage) — add it as its own optional field:

```go
type Worktree struct {
	ID        string
	ProjectID string
	RepoID    string
	Path      string
	Branch    string
	Active    bool
	CreatedAt time.Time

	IdempotencyKey *string // BR-CLI-01: caller-supplied dedupe key, nil when not set

	ParentWorktreeID        *string
	// ... existing lineage fields unchanged ...
}
```

Add an `IdempotencyKey string` param to `NewWorktree`'s signature (empty string = nil, same `nonEmptyPtr` idiom already used for lineage fields):

```go
func NewWorktree(id, projectID, repoID, path, branch, idempotencyKey string, lineage WorktreeLineageCapture) (Worktree, error) {
	// ... existing validation unchanged ...
	wt := Worktree{
		ID: id, ProjectID: projectID, RepoID: repoID, Path: path, Branch: branch, Active: true,
		IdempotencyKey: nonEmptyPtr(idempotencyKey),
		ParentWorktreeID: nonEmptyPtr(lineage.ParentWorktreeID),
		// ... unchanged ...
	}
	// ... unchanged ...
}
```

**3. `worktreeColumns` + `scanWorktree`** (`backend-go/services/project-service/internal/adapter/postgres/worktree_repository.go`) — add `idempotency_key` to the column list and scan target, and add the new lookup method:

```go
const worktreeColumns = `id, project_id, repo_id, path, branch, active, created_at,
	idempotency_key,
	parent_worktree_id, origin, capture_source, capture_confidence, task_id,
	orchestration_run_id, coordinator_handle, created_by_terminal_handle`
```

```go
func scanWorktree(row rowScanner) (domain.Worktree, error) {
	var wt domain.Worktree
	if err := row.Scan(
		&wt.ID, &wt.ProjectID, &wt.RepoID, &wt.Path, &wt.Branch, &wt.Active, &wt.CreatedAt,
		&wt.IdempotencyKey,
		&wt.ParentWorktreeID, &wt.Origin, &wt.CaptureSource, &wt.CaptureConfidence, &wt.TaskID,
		&wt.OrchestrationRunID, &wt.CoordinatorHandle, &wt.CreatedByTerminalHandle,
	); err != nil {
		return domain.Worktree{}, err
	}
	return wt, nil
}

// FindWorktreeByIdempotencyKey backs BR-CLI-01 — a second CreateWorktree
// call with the same (project_id, idempotency_key) returns the existing row
// instead of git-gateway-service re-running `git worktree add`.
// found=false, err=nil means "no match yet", not an error.
func (r *WorktreeRepository) FindWorktreeByIdempotencyKey(ctx context.Context, projectID, idempotencyKey string) (domain.Worktree, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+worktreeColumns+`
		FROM project.worktrees
		WHERE project_id = $1 AND idempotency_key = $2
	`, projectID, idempotencyKey)

	out, err := scanWorktree(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Worktree{}, false, nil
	}
	if err != nil {
		return domain.Worktree{}, false, fmt.Errorf("postgres: find worktree by idempotency key: %w", err)
	}
	return out, true, nil
}
```

Update `RecordWorktreeCreated`'s `INSERT` (same file) to include `idempotency_key` in the column/value list and `wt.IdempotencyKey` in the bind args, matching the existing lineage-field pattern.

**4. `usecase.WorktreeRepository` port** (`backend-go/services/project-service/internal/usecase/ports.go`, `type WorktreeRepository interface` at line 133) — add the new method:

```go
	FindWorktreeByIdempotencyKey(ctx context.Context, projectID, idempotencyKey string) (domain.Worktree, bool, error)
```

**5. `RecordWorktreeCreatedInput`/`RecordWorktreeCreated.Execute`** (`backend-go/services/project-service/internal/usecase/record_worktree_created.go`) — add `IdempotencyKey string` to the input struct and thread it into `domain.NewWorktree`'s new parameter.

**6. Proto** — in `backend-go/proto/orca/project/v1/project.proto`:

- Extend `RecordWorktreeCreatedRequest` and `Worktree` with `optional string idempotency_key` (next available field number on each message).
- Add a new unary RPC `git-gateway-service` will call in TASK-CLI-01-03 (project-service has no RPC that exposes `WorktreeRepository` methods 1:1 today, so this is new surface, not a rename):

```protobuf
service ProjectService {
  // ... existing RPCs unchanged ...

  // GetWorktreeByIdempotencyKey backs BR-CLI-01 — git-gateway-service's
  // CreateWorktree saga calls this before running `git worktree add`.
  // found=false means "no dedupe match yet", not an error.
  rpc GetWorktreeByIdempotencyKey(GetWorktreeByIdempotencyKeyRequest) returns (GetWorktreeByIdempotencyKeyResponse);
}

message GetWorktreeByIdempotencyKeyRequest {
  string project_id = 1;
  string idempotency_key = 2;
}
message GetWorktreeByIdempotencyKeyResponse {
  bool found = 1;
  Worktree worktree = 2; // unset when found=false
}
```

Regenerate stubs (`buf generate proto`). Add a thin usecase (`backend-go/services/project-service/internal/usecase/get_worktree_by_idempotency_key.go`), matching every other read's `usecase.X{repo}`/`Execute` shape (e.g. `ListWorktrees`):

```go
type GetWorktreeByIdempotencyKey struct {
	repo WorktreeRepository
}

func NewGetWorktreeByIdempotencyKey(repo WorktreeRepository) *GetWorktreeByIdempotencyKey {
	return &GetWorktreeByIdempotencyKey{repo: repo}
}

func (uc *GetWorktreeByIdempotencyKey) Execute(ctx context.Context, projectID, idempotencyKey string) (domain.Worktree, bool, error) {
	return uc.repo.FindWorktreeByIdempotencyKey(ctx, projectID, idempotencyKey)
}
```

Wire it into `internal/adapter/grpc/server.go`'s `Server` struct/`New(...)` (same pattern as `listWorktrees`) and its new `GetWorktreeByIdempotencyKey` handler, and into `cmd/server/main.go`'s DI construction. Also update the existing `RecordWorktreeCreated` handler to pass `req.GetIdempotencyKey()` through to `RecordWorktreeCreatedInput`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/project-service/...
go test ./services/project-service/internal/adapter/postgres/... -run TestWorktreeRepository -v
go test ./services/project-service/internal/domain/... -run TestWorktree -v
go test ./services/project-service/internal/usecase/... -run TestRecordWorktreeCreated -v
```

Expected: clean build; a new `worktree_repository_test.go` case (`TestWorktreeRepository_FindWorktreeByIdempotencyKey_RoundTrips`, `TestWorktreeRepository_FindWorktreeByIdempotencyKey_NoMatchReturnsFoundFalse`) passes against `testcontainers-go` Postgres.
