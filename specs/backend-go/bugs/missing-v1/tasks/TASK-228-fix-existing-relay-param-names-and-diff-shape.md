# TASK-228: Fix `RelayExecutor`'s param names + `GetDiff`'s per-file shape for the 5 already-implemented `git.*` relay methods

**From Solution:** SOL-036 / SOL-032 §0
**Priority:** P0 — without this, `git.status`/`git.diff`/`git.commit`/`git.push`/`git.pull` remain broken against a real agent even after TASK-227 lands (TASK-227 makes the methods *reachable*; this task makes the *calls* correct)
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/ports.go`, `internal/usecase/get_diff.go`, `internal/adapter/grpcclient/relay_executor.go`, `internal/adapter/localgit/executor.go`, `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`, `internal/adapter/grpc/server.go`
**Depends on:** TASK-227 (no point fixing param names for a method the agent still can't reach — though these two tasks can be developed in parallel and merged in either order, since they touch disjoint files)
**Status:** `[x]` DONE — `RelayExecutor`'s 5 methods now send `worktreePath` (not `repoPath`); `GetDiff` threads a required `FilePath` through proto (`GetDiffRequest.file_path`), usecase, `localgit.Executor`, `RelayExecutor`, and the gRPC adapter. Confirmed frontend call sites (`DiffViewer.tsx`, `runtime-git-client.ts`) already send `filePath` on every `git.diff` call, so this is not a breaking wire-contract change. `GenerateCommitMessage`'s own internal whole-diff need is now served by a new `gatherFullDiff` helper (`GetStatus` + one `GetDiff` per changed file) instead of a filepath-less `GetDiff` call. `buf generate` clean, `go build`/`go vet`/`go test` clean for `git-gateway-service` and `api-gateway`. `Push`/`Pull`'s `pushTarget` redesign is explicitly NOT done (SOL-032 §0 open question #1, out of this task's scope per its own Context section).

---

## Context

`RelayExecutor`'s 5 existing methods (`GetStatus`, `GetDiff`, `Commit`,
`Push`, `Pull`) all send `repoPath` as the JSON param key; the real agent
(both Part B's native contract and, once TASK-227 lands, Part A's
re-export of it) expects `worktreePath`. `GetDiff` additionally sends no
`filePath` at all, even though the real `git.diff` is a per-file
operation — this is a genuine missing parameter, not a naming mismatch,
and requires a signature change up through the proto layer.

`Push`/`Pull` have a deeper problem SOL-032 §0's open design question #1
flags: the real agent resolves a structured `pushTarget` object with real
safety logic (blocks pushing a fork-tracking branch to `origin` unless
configured), not a bare `{remote, branch}` pair. **This task fixes the
param-naming half only** (`repoPath`→`worktreePath`); the `pushTarget`
redesign is out of scope here — track it as a follow-up once
`git-handler-push-target.ts` has been read in full and a `PushTargetInput`
design exists (see SOL-032 §0). Sending `{worktreePath, remote, branch}`
today will likely fail the agent's shape validator or bypass the
fork-branch safety check; document this as a known limitation if this
task ships before the `pushTarget` redesign lands, rather than silently
shipping the unsafe version.

## Changes to make

### Step 1: `GetDiff`'s signature — `internal/usecase/ports.go` + `get_diff.go`

```go
// ports.go — GitExecutor
GetDiff(ctx context.Context, repoPath, filePath string, staged bool) (domain.DiffResult, error)
```

Update `get_diff.go`'s `GetDiffInput`/`Execute` to thread a required
`FilePath` field through (mirror `Checkout`'s multi-field-validation shape
from TASK-207 once that lands, or validate inline if TASK-228 ships
first — same `apperrors.KindInvalidArgument` pattern either way):

```go
type GetDiffInput struct {
	WorktreeID string
	FilePath   string
	Staged     bool
}

func (uc *GetDiff) Execute(ctx context.Context, in GetDiffInput) (domain.DiffResult, error) {
	if in.WorktreeID == "" {
		return domain.DiffResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	if in.FilePath == "" {
		return domain.DiffResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_FILE_PATH", "file_path is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.DiffResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	diff, err := executor.GetDiff(ctx, repoPath, in.FilePath, in.Staged)
	if err != nil {
		return domain.DiffResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_DIFF_FAILED", "failed to get git diff", err)
	}
	return diff, nil
}
```

**Frontend impact check before merging**: `rpc-catalog.md`'s `git.diff`
call sites (`DiffViewer.tsx`, `use-code-review.ts`) need to already pass a
`filePath` — if they currently call `git.diff` expecting a whole-repo
diff back, this is a breaking change to `wscompat`'s `git.diff` channel
contract, not just an internal signature fix. Read those call sites before
merging; if they genuinely need a whole-repo diff, this task's direction
is wrong and `GetDiff` should stay whole-repo, composed instead via
`git.status` (enumerate files) + one `git.diff` call per file, entirely
inside `git-gateway-service` — don't just thread `filePath` through to the
wire channel if the frontend never sends one today.

### Step 2: Proto — add `file_path` to `GetDiffRequest`

```protobuf
message GetDiffRequest {
  string worktree_id = 1;
  string file_path = 2; // NEW — required
  bool staged = 3;
}
```

Additive at the message level (new field), but changes the field's
*required* semantics at the application layer — note this in the PR
description even though `buf breaking` won't flag it (proto field
addition is never a wire-breaking change, but an app-level required-field
change is a real behavior change for any existing caller).

### Step 3: `RelayExecutor` — fix all 5 methods' param key

```go
func (r *RelayExecutor) GetStatus(ctx context.Context, repoPath string) (domain.GitStatus, error) {
	var result domain.GitStatus
	err := r.relay(ctx, repoPath, "git.status", map[string]any{
		"worktreePath": repoPath, // was "repoPath" — real agent contract, see BUG-036
	}, &result)
	return result, err
}

func (r *RelayExecutor) GetDiff(ctx context.Context, repoPath, filePath string, staged bool) (domain.DiffResult, error) {
	var result domain.DiffResult
	err := r.relay(ctx, repoPath, "git.diff", map[string]any{
		"worktreePath": repoPath, "filePath": filePath, "staged": staged,
	}, &result)
	return result, err
}

func (r *RelayExecutor) Commit(ctx context.Context, repoPath, message string, paths []string) (domain.CommitResult, error) {
	var result domain.CommitResult
	// NOTE: "paths" is sent but the real agent ignores it — commit assumes
	// pre-staged content (via a prior git.stage/git.bulkStage call, see
	// TASK-208). Left in the wire payload for now rather than silently
	// dropping the Go signature's paths param — harmless either way until
	// TASK-208's staging usecases exist and callers are updated to stage
	// first.
	err := r.relay(ctx, repoPath, "git.commit", map[string]any{
		"worktreePath": repoPath, "message": message,
	}, &result)
	return result, err
}

func (r *RelayExecutor) Push(ctx context.Context, repoPath, remote, branch string) (domain.PushResult, error) {
	var result domain.PushResult
	// KNOWN LIMITATION (see this task's Context section): real agent wants
	// a structured pushTarget, not bare remote/branch. This param shape
	// may be rejected by the agent's shape validator or bypass fork-branch
	// push safety. Do not consider Push "fixed" until the pushTarget
	// redesign (SOL-032 §0 open question #1) lands.
	err := r.relay(ctx, repoPath, "git.push", map[string]any{
		"worktreePath": repoPath, "remote": remote, "branch": branch,
	}, &result)
	return result, err
}

func (r *RelayExecutor) Pull(ctx context.Context, repoPath string) (domain.PullResult, error) {
	var result domain.PullResult
	// Same pushTarget limitation as Push above.
	err := r.relay(ctx, repoPath, "git.pull", map[string]any{
		"worktreePath": repoPath,
	}, &result)
	return result, err
}
```

### Step 4: `localgit.Executor.GetDiff` — update signature to match

```go
// GetDiff runs `git diff [--staged] -- <filePath>` — per-file, matching
// the real Dev Server Agent contract this local path must stay
// consistent with (see BUG-036/TASK-228).
func (e *Executor) GetDiff(ctx context.Context, repoPath, filePath string, staged bool) (domain.DiffResult, error) {
	args := []string{"diff"}
	if staged {
		args = append(args, "--staged")
	}
	args = append(args, "--", filePath)
	out, err := e.run(ctx, repoPath, args...)
	if err != nil {
		return domain.DiffResult{}, err
	}
	return domain.DiffResult{UnifiedDiff: out}, nil
}
```

### Step 5: gRPC adapter — thread `file_path` through

```go
func (s *Server) GetDiff(ctx context.Context, req *gitgatewayv1.GetDiffRequest) (*gitgatewayv1.GetDiffResponse, error) {
	result, err := s.getDiff.Execute(ctx, usecase.GetDiffInput{
		WorktreeID: req.GetWorktreeId(), FilePath: req.GetFilePath(), Staged: req.GetStaged(),
	})
	// ... unchanged from here
}
```

### Step 6: `wscompat` channel — thread `filePath` through (touches `channels.go`, not just `git-gateway-service`)

```go
r.Register("git.diff", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
	type diffArgs struct {
		WorktreeID string `json:"worktreeId"`
		FilePath   string `json:"filePath"` // NEW
		Staged     bool   `json:"staged"`
	}
	in, err := decodeArg[diffArgs](args, 0)
	if err != nil {
		return nil, err
	}
	resp, err := client.GetDiff(ctx, &gitgatewayv1.GetDiffRequest{
		WorktreeId: in.WorktreeID, FilePath: in.FilePath, Staged: in.Staged,
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
})
```

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
cd services/git-gateway-service
go build ./... && go vet ./...
go test ./... -run "TestGetDiff|TestGetStatus|TestCommit|TestPush|TestPull"
cd ../api-gateway
go build ./internal/adapter/wscompat/... && go test ./internal/adapter/wscompat/... -run TestGitDiff
```

Then, once TASK-227 has also shipped: an integration test against a real
or faithfully faked agent connection confirming `git.status`/`git.diff`/
`git.commit` return real data. `git.push`/`git.pull` stay flagged as
"param-name-fixed but shape-incomplete" until the `pushTarget` redesign
closes separately.
