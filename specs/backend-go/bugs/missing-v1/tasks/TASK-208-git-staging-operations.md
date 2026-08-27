# TASK-208: Add Group B — staging RPCs to `git-gateway-service` (2 RPCs backing 4 methods)

**From Solution:** SOL-032 (Part 2, Group B)
**Priority:** P1 — unblocks the core staging workflow, ship alongside TASK-207
**Service:** `git-gateway-service`
**File:** `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`, `internal/usecase/ports.go`, `internal/usecase/stage.go` (new), `internal/usecase/unstage.go` (new), `internal/adapter/localgit/executor.go`, `internal/adapter/grpcclient/relay_executor.go`, `internal/adapter/grpc/server.go`, `cmd/server/main.go`
**Depends on:** TASK-227 (agent reachability) — `stage`/`unstage`/
`bulkStage`/`bulkUnstage` are all currently unreachable on the agent
process backend-go's transport actually reaches (SOL-032 §0), so this
group's relay leg won't produce a working result until TASK-227 lands.
This task builds NEW RPCs, so unlike TASK-206 there is nothing here
"already implemented" to fall back on in the meantime; TASK-208 and
TASK-227 can still be developed in parallel since they touch disjoint
files. Otherwise touches the same `ports.go`/`server.go`/`main.go` files
as TASK-207/209/210/211 — rebase onto whichever has already merged.
**Status:** `[x]` DONE — `Stage`/`Unstage` RPCs added to proto; usecases, `localgit.Executor` (`git add --`/`git restore --staged --`), and `RelayExecutor` (always targets `git.bulkStage`/`git.bulkUnstage` regardless of path count, per this task's Contract correction section) all implemented; gRPC server + `main.go` wiring done. `go build`/`go vet`/`go test` clean. Runtime correctness against a real relay-connected worktree is still contingent on TASK-227 (agent reachability) having landed separately — out of this task's own scope.

---

## ⚠️ Contract correction (read before implementing)

SOL-032 §0 confirms this group is in GOOD shape once one specific fix is
applied — **no genuine redesign needed, unlike Groups A and C's
BLOCKED methods.**

The real agent has NO bulk variant of `git.stage`/`git.unstage` that
takes an arbitrary list — those two methods are **single-file only**
(`worktreePath, filePath`). The bulk behavior lives on two SEPARATE agent
methods, `git.bulkStage`/`git.bulkUnstage` (`worktreePath, filePaths[]`),
which accept any count ≥1 (including 1).

This task's design — 2 RPCs, `Stage`/`Unstage`, each taking `repeated
paths` — still works, **provided `RelayExecutor.Stage`/`Unstage` always
relay to `"git.bulkStage"`/`"git.bulkUnstage"`** (never
`"git.stage"`/`"git.unstage"`) regardless of how many paths were passed.
Since the bulk variants accept any count ≥1, this is safe and simpler
than branching on `len(paths)==1` to pick between two different relay
method names and param shapes. Fixed directly in this file's
`RelayExecutor.Stage`/`Unstage` code blocks below: (a) always target the
bulk method name, (b) use `filePaths` as the JSON key (not `paths`), (c)
a short comment explaining why.

---

## Context

`stage`/`bulkStage`/`unstage`/`bulkUnstage` are the same wire operation at
different call-site granularity (single file vs. multi-select). Per
SOL-032, collapse them onto **2 RPCs** that already take a repeated field —
`wscompat` still registers 4 separate channels (`git.stage`, `git.bulkStage`,
`git.unstage`, `git.bulkUnstage`, added in TASK-212), all calling the same 2
RPCs. The collapse happens at the proto/usecase layer only.

## Changes to make

### Step 1: Proto

Add to the `GitGatewayService` service block:

```protobuf
  rpc Stage(StageRequest) returns (StageResponse);
  rpc Unstage(UnstageRequest) returns (UnstageResponse);
```

Append messages:

```protobuf
message StageRequest {
  string worktree_id = 1;
  repeated string paths = 2; // single-element for git.stage, full selection for git.bulkStage
}
message StageResponse {
  bool success = 1;
}

message UnstageRequest {
  string worktree_id = 1;
  repeated string paths = 2; // single-element for git.unstage, full selection for git.bulkUnstage
}
message UnstageResponse {
  bool success = 1;
}
```

### Step 2: Extend `GitExecutor` — `internal/usecase/ports.go`

```go
	Stage(ctx context.Context, repoPath string, paths []string) (domain.SimpleResult, error)
	Unstage(ctx context.Context, repoPath string, paths []string) (domain.SimpleResult, error)
```

(`domain.SimpleResult` is the bare-success-flag type added in TASK-207's
`domain.go` change — this task depends on that type existing, so land
TASK-207 first or add `SimpleResult` here if TASK-207 hasn't merged yet;
whichever PR lands first owns adding it, the second just reuses it.)

### Step 3: Usecases — `internal/usecase/`

`stage.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type StageInput struct {
	WorktreeID string
	Paths      []string
}

type Stage struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewStage(resolver ConnectionResolver, local, relay GitExecutor) *Stage {
	return &Stage{resolver: resolver, local: local, relay: relay}
}

func (uc *Stage) Execute(ctx context.Context, in StageInput) (domain.SimpleResult, error) {
	if in.WorktreeID == "" {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	if len(in.Paths) == 0 {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_PATHS", "paths is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	result, err := executor.Stage(ctx, repoPath, in.Paths)
	if err != nil {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_STAGE_FAILED", "failed to stage paths", err)
	}
	return result, nil
}
```

`unstage.go` is identical to `stage.go` with `Stage`→`Unstage`,
`GITGATEWAY_STAGE_FAILED`→`GITGATEWAY_UNSTAGE_FAILED`.

### Step 4: `localgit.Executor`

```go
// Stage runs `git add -- <paths...>`.
func (e *Executor) Stage(ctx context.Context, repoPath string, paths []string) (domain.SimpleResult, error) {
	args := append([]string{"add", "--"}, paths...)
	if _, err := e.run(ctx, repoPath, args...); err != nil {
		return domain.SimpleResult{}, err
	}
	return domain.SimpleResult{Success: true}, nil
}

// Unstage runs `git restore --staged -- <paths...>` (Git 2.23+, well under
// this service's Git 2.5 baseline for the rest of its command set — but
// still above the project's Git 2.25 compatibility floor per
// docs/reference/git-compatibility.md, so no `git reset HEAD --` fallback
// branch is needed; confirm this against git-compatibility.md before
// merging per that doc's required-check convention).
func (e *Executor) Unstage(ctx context.Context, repoPath string, paths []string) (domain.SimpleResult, error) {
	args := append([]string{"restore", "--staged", "--"}, paths...)
	if _, err := e.run(ctx, repoPath, args...); err != nil {
		return domain.SimpleResult{}, err
	}
	return domain.SimpleResult{Success: true}, nil
}
```

### Step 5: `RelayExecutor`

```go
// Stage always relays to "git.bulkStage", never "git.stage" — the real
// agent's git.stage is single-file only (no repeated-paths support), but
// git.bulkStage accepts any count ≥1, so it's a strict superset that
// covers both the single-file (stage) and multi-select (bulkStage)
// frontend call sites without branching on len(paths). See SOL-032 §0 /
// this task's Contract correction section.
func (r *RelayExecutor) Stage(ctx context.Context, repoPath string, paths []string) (domain.SimpleResult, error) {
	var result domain.SimpleResult
	err := r.relay(ctx, repoPath, "git.bulkStage", map[string]any{
		"worktreePath": repoPath, "filePaths": paths,
	}, &result)
	return result, err
}

// Unstage always relays to "git.bulkUnstage" — same reasoning as Stage
// above.
func (r *RelayExecutor) Unstage(ctx context.Context, repoPath string, paths []string) (domain.SimpleResult, error) {
	var result domain.SimpleResult
	err := r.relay(ctx, repoPath, "git.bulkUnstage", map[string]any{
		"worktreePath": repoPath, "filePaths": paths,
	}, &result)
	return result, err
}
```

### Step 6: gRPC adapter — `internal/adapter/grpc/server.go`

```go
func (s *Server) Stage(ctx context.Context, req *gitgatewayv1.StageRequest) (*gitgatewayv1.StageResponse, error) {
	result, err := s.stage.Execute(ctx, usecase.StageInput{WorktreeID: req.GetWorktreeId(), Paths: req.GetPaths()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.StageResponse{Success: result.Success}, nil
}

func (s *Server) Unstage(ctx context.Context, req *gitgatewayv1.UnstageRequest) (*gitgatewayv1.UnstageResponse, error) {
	result, err := s.unstage.Execute(ctx, usecase.UnstageInput{WorktreeID: req.GetWorktreeId(), Paths: req.GetPaths()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.UnstageResponse{Success: result.Success}, nil
}
```

Add `stage *usecase.Stage` / `unstage *usecase.Unstage` fields to `Server`
and 2 params to `New`.

### Step 7: Composition root — `cmd/server/main.go`

```go
	stageUC := usecase.NewStage(resolver, local, relay)
	unstageUC := usecase.NewUnstage(resolver, local, relay)
```

Extend the `gitgatewaygrpc.New(...)` call with `stageUC, unstageUC`.

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
cd services/git-gateway-service
go build ./... && go vet ./...
```

**This does not confirm `Stage`/`Unstage` work end-to-end for a
relay-dispatched (SSH-connected) worktree** — a clean build only confirms
the Go compiles, not that the relay calls succeed against a real agent.
That requires TASK-227 (agent reachability) to land first. Local
(unconnected) worktrees are unaffected by this gap.
