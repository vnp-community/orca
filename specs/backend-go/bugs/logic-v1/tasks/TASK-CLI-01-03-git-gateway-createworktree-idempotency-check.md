# TASK-CLI-01-03: `git-gateway-service` — `CreateWorktree.Execute` idempotency check (BR-CLI-01)

**From Solution:** SOL-CLI-01
**Priority:** P0 — the REST route (TASK-CLI-01-04) forwards `idempotency_key` through this usecase
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/create_worktree.go`
**Depends on:** TASK-CLI-01-01 (proto field), TASK-CLI-01-02 (project-service lookup RPC)
**Status:** `[ ]` TODO

---

## Context

`create_worktree.go:41`'s `Execute` has no dedupe check today — a second call with the same `(project_id, repo_id, branch)` re-runs `git worktree add` and fails. This task adds the BR-CLI-01 short-circuit: if `IdempotencyKey` is set and a match already exists, return it without touching `GitExecutor` again.

## Changes to make

**1. `ProjectClient` port** (`backend-go/services/git-gateway-service/internal/usecase/ports.go`, `type ProjectClient interface` at line 341) — add the new method:

```go
type ProjectClient interface {
	GetRepo(ctx context.Context, repoID string) (domain.RepoInfo, error)
	RecordWorktreeCreated(ctx context.Context, projectID, repoID, path, branch string, lineage domain.WorktreeLineageCapture) (domain.WorktreeRecord, error)
	RecordWorktreeRemoved(ctx context.Context, worktreeID string) error
	// FindWorktreeByIdempotencyKey backs BR-CLI-01 — see CreateWorktree.Execute.
	// found=false, err=nil means "no match yet".
	FindWorktreeByIdempotencyKey(ctx context.Context, projectID, idempotencyKey string) (domain.WorktreeRecord, bool, error)
}
```

**2. `CreateWorktreeInput`** — add the field:

```go
type CreateWorktreeInput struct {
	ProjectID, RepoID, Branch, BaseRef, IdempotencyKey string
	Lineage                                            domain.WorktreeLineageCapture
}
```

**3. `Execute`** (`create_worktree.go:41-71`) — prepend the check:

```go
func (uc *CreateWorktree) Execute(ctx context.Context, in CreateWorktreeInput) (domain.WorktreeResult, error) {
	if in.IdempotencyKey != "" {
		if existing, found, err := uc.projects.FindWorktreeByIdempotencyKey(ctx, in.ProjectID, in.IdempotencyKey); err != nil {
			return domain.WorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_IDEMPOTENCY_LOOKUP_FAILED", "failed to check for existing worktree", err)
		} else if found {
			// BR-CLI-01: same (project_id, idempotency_key) -> return the
			// existing worktree, not a second `git worktree add` attempt.
			// HeadSHA is not stored on the bookkeeping record (project-service's
			// Worktree message has no head_sha field) — left empty here rather
			// than re-resolving it with an extra GetStatus call the caller
			// didn't ask for; orca-cli's Result.HeadSHA is therefore only
			// populated on a genuinely fresh create.
			return domain.WorktreeResult{WorktreeID: existing.ID, Path: existing.Path}, nil
		}
	}

	repo, err := uc.projects.GetRepo(ctx, in.RepoID)
	// ... unchanged from here ...
}
```

**4. `internal/adapter/grpcclient/project_client.go`** — implement the new port method against the RPC TASK-CLI-01-02 added:

```go
func (p *ProjectClient) FindWorktreeByIdempotencyKey(ctx context.Context, projectID, idempotencyKey string) (domain.WorktreeRecord, bool, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return domain.WorktreeRecord{}, false, err
	}
	resp, err := p.client.GetWorktreeByIdempotencyKey(ctx, &projectv1.GetWorktreeByIdempotencyKeyRequest{
		ProjectId: projectID, IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		return domain.WorktreeRecord{}, false, err
	}
	if !resp.GetFound() {
		return domain.WorktreeRecord{}, false, nil
	}
	wt := resp.GetWorktree()
	return domain.WorktreeRecord{ID: wt.GetId(), Path: wt.GetPath(), Branch: wt.GetBranch()}, true, nil
}
```

**5. `internal/adapter/grpc/server.go`** (git-gateway-service's gRPC adapter) — pass the new field through from the wire request:

```go
func (s *Server) CreateWorktree(ctx context.Context, req *gitgatewayv1.CreateWorktreeRequest) (*gitgatewayv1.CreateWorktreeResponse, error) {
	out, err := s.createWorktree.Execute(ctx, usecase.CreateWorktreeInput{
		ProjectID: req.GetProjectId(), RepoID: req.GetRepoId(), Branch: req.GetBranch(), BaseRef: req.GetBaseRef(),
		IdempotencyKey: req.GetIdempotencyKey(),
		Lineage: /* ...unchanged lineage construction... */,
	})
	// ... unchanged ...
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/git-gateway-service/...
go test ./services/git-gateway-service/internal/usecase/... -run TestCreateWorktree -v
```

Expected new test (`create_worktree_test.go`, per `worktree_fakes_test.go`'s existing fake `ProjectClient` shape): `TestCreateWorktree_IdempotencyKeyMatch_ReturnsExistingWithoutExecutorCall` — a fake `ProjectClient.FindWorktreeByIdempotencyKey` returns `found=true`; assert the fake `GitExecutor.CreateWorktree` is never invoked (zero calls) and the returned `WorktreeResult` matches the existing record.
