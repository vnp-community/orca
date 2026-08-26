# TASK-194: Add required (not optional) `ForceDeleteBranch` to `GitExecutor` + wire all 7 new RPCs into `grpc/server.go` and `main.go`

**From Solution:** SOL-031 (design part 3: "the required (not optional) `ForceDeleteBranch` interface fix")
**Priority:** P0
**Service:** `git-gateway-service`
**File:** `internal/usecase/ports.go`, `internal/usecase/force_delete_branch.go` (new), `internal/adapter/localgit/executor.go`, `internal/adapter/grpcclient/relay_executor.go`, `internal/adapter/grpc/server.go`, `cmd/server/main.go`
**Depends on:** TASK-193
**Status:** `[x]` DONE — implemented in worktree `agent-aa8bd8599a599323a` (team/terminal/workflow/worktree pass, merged into `integration/missing-v1` as commit `baa34819a`); this task doc's own Status line was never updated by that implementing pass (a task-doc-capture gap, not a missing-code gap) — verified against the current merged code+tests during a later re-audit: build/vet/test clean.

---

## Context

BUG-031 cites the exact defect to avoid: the old TS `GitProvider` interface
declared `forceDeletePreservedBranch?` with a `?` — optional — and only
`SshGitProvider` implemented it, so calling it against a provider variant
that didn't crashed at runtime. Go's static interface satisfaction makes
the equivalent mistake structurally impossible **if** the method is added
to `GitExecutor` itself, not bolted onto one implementation with a runtime
type-assertion escape hatch — the compiler then forces every
`GitExecutor` implementation to have it before the package even builds.

This task also does the one-shot wiring of all 7 new RPCs (TASK-192's
proto + TASK-193's 6 usecases + this task's `ForceDeleteBranch`) into
`grpc/server.go` and `cmd/server/main.go`, since by now every usecase
exists — following the exact translate-only pattern
`git-gateway-service`'s `grpc/server.go` already uses for `GetStatus`/
`GetDiff`/etc.

## Changes to make

### Step 1 — `internal/usecase/ports.go`: add `ForceDeleteBranch` to `GitExecutor`

Find the `GitExecutor` interface (as extended by TASK-193) and add the
final method:

```go
type GitExecutor interface {
	GetStatus(ctx context.Context, repoPath string) (domain.GitStatus, error)
	GetDiff(ctx context.Context, repoPath string, staged bool) (domain.DiffResult, error)
	Commit(ctx context.Context, repoPath, message string, paths []string) (domain.CommitResult, error)
	Push(ctx context.Context, repoPath, remote, branch string) (domain.PushResult, error)
	Pull(ctx context.Context, repoPath string) (domain.PullResult, error)
	CreateWorktree(ctx context.Context, repoPath, branch, baseRef string) (domain.WorktreeCreateResult, error)
	RemoveWorktree(ctx context.Context, worktreePath string, force bool) error
	FetchAndResolveRef(ctx context.Context, repoPath, ref string) (sha string, err error)
	ListWorktreePaths(ctx context.Context, repoPath string) ([]string, error)
	// ForceDeleteBranch is REQUIRED on every GitExecutor implementation —
	// deliberately not an optional/type-asserted method. This is the
	// structural fix for the old TS backend's forceDeletePreservedBranch?
	// crash-bug class (BUG-031): Go's compiler now refuses to build ANY
	// GitExecutor implementation missing this method, closing the "one
	// provider variant silently lacks it" gap by construction, not by
	// convention. The operational fallback for an outdated relay-side
	// agent build that genuinely doesn't support this is handled inside
	// RelayExecutor.ForceDeleteBranch's own body (a typed, caught error),
	// independent of this compile-time guarantee.
	ForceDeleteBranch(ctx context.Context, repoPath, branch string) error
}
```

(`ListWorktreePaths` is included here per TASK-193 Step 8's note that it
belongs on the interface, not behind a runtime type assertion — if
TASK-193 was implemented with the type-assertion sketch instead, resolve
that now, before adding `ForceDeleteBranch`, so `GitExecutor` ends this
task with a clean, fully-required method set.)

### Step 2 — `internal/adapter/localgit/executor.go`: implement `ForceDeleteBranch`, `ListWorktreePaths` (if not already done in TASK-193)

```go
// ForceDeleteBranch runs `git branch -D <branch>` — force delete, no
// merge-check (the caller has already decided this branch's worktree is
// being torn down). Available since Git 2.5, same baseline as this
// package's other commands.
func (e *Executor) ForceDeleteBranch(ctx context.Context, repoPath, branch string) error {
	_, err := e.run(ctx, repoPath, "branch", "-D", branch)
	return err
}

// ListWorktreePaths runs `git worktree list --porcelain` and extracts
// every `worktree <path>` line — the raw on-disk truth DetectWorktrees
// needs, with no bookkeeping join.
func (e *Executor) ListWorktreePaths(ctx context.Context, repoPath string) ([]string, error) {
	out, err := e.run(ctx, repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		if p, ok := strings.CutPrefix(line, "worktree "); ok {
			paths = append(paths, p)
		}
	}
	return paths, nil
}
```

### Step 3 — `internal/adapter/grpcclient/relay_executor.go`: implement `ForceDeleteBranch` with the operational fallback

```go
// ErrForceDeleteBranchUnsupported is returned when the relay target's Dev
// Server Agent build predates a force-delete-branch method — the
// operational counterpart to Step 1's compile-time guarantee: the
// structural fix is "every GitExecutor has the method", this is "the
// method call fails cleanly" for an outdated agent build, per BUG-031's
// cited old-TS fallback comment ("older SSH relays predate
// git.forceDeletePreservedBranch").
var ErrForceDeleteBranchUnsupported = errors.New("grpcclient: relay target does not support force-delete-branch")

func (r *RelayExecutor) ForceDeleteBranch(ctx context.Context, repoPath, branch string) error {
	err := r.relay(ctx, repoPath, "git.forceDeleteBranch", map[string]any{
		"repoPath": repoPath, "branch": branch,
	}, nil)
	if err != nil && isMethodNotFoundError(err) {
		return fmt.Errorf("%w: %v", ErrForceDeleteBranchUnsupported, err)
	}
	return err
}

// isMethodNotFoundError is a placeholder heuristic for detecting an
// agent's "unknown method" response through the Relay RPC's error path —
// FLAGGED: confirm the real error shape RelayResponse/infra-fleet-service's
// Relay RPC surfaces for an unsupported agent method before finalizing;
// this may need to check a gRPC status code (codes.Unimplemented) rather
// than string-matching, depending on how infra-fleet-service's Relay
// usecase translates an agent-side JSON-RPC "method not found" today.
func isMethodNotFoundError(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "method not found") ||
		strings.Contains(strings.ToLower(err.Error()), "unknown method")
}
```

Add `"errors"` and `"strings"` to this file's import block if not already
present. `WORKTREE_FORCE_DELETE_UNSUPPORTED`'s mapping to a clear gRPC
status happens at the usecase layer (Step 4 below), not here — this
adapter method only needs to distinguish "unsupported" from "any other
failure" via the sentinel.

### Step 4 — `internal/usecase/force_delete_branch.go` (new)

```go
package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/adapter/grpcclient"
)

type ForceDeleteBranch struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewForceDeleteBranch(resolver ConnectionResolver, local, relay GitExecutor) *ForceDeleteBranch {
	return &ForceDeleteBranch{resolver: resolver, local: local, relay: relay}
}

func (uc *ForceDeleteBranch) Execute(ctx context.Context, worktreeID, branch string) error {
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}
	if err := executor.ForceDeleteBranch(ctx, repoPath, branch); err != nil {
		if errors.Is(err, grpcclient.ErrForceDeleteBranchUnsupported) {
			return apperrors.New(apperrors.KindFailedPrecondition, "WORKTREE_FORCE_DELETE_UNSUPPORTED", "the target dev server's agent does not support force-deleting a branch", err)
		}
		return apperrors.New(apperrors.KindInternal, "WORKTREE_FORCE_DELETE_FAILED", "failed to force-delete branch", err)
	}
	return nil
}
```

`usecase` importing `internal/adapter/grpcclient` for the sentinel is a
one-off exception to strict layering (the sentinel needs a stable home);
if this creates an import cycle (it should not — `grpcclient` already
imports `usecase` for the interfaces it implements, so `usecase` importing
`grpcclient` back WOULD cycle), move `ErrForceDeleteBranchUnsupported` to
`internal/domain` instead (a domain-level sentinel, same tier as
`domain.ErrWorktreeNotFound`-equivalent errors) and have
`relay_executor.go` return that one instead of a `grpcclient`-local
sentinel. Resolve this before implementation — do not leave a real import
cycle in the merged code.

### Step 5 — `internal/adapter/grpc/server.go`: wire all 7 new RPCs

Add 7 fields to `Server` and 7 params to `New`:

```go
	createWorktree     *usecase.CreateWorktree
	removeWorktree     *usecase.RemoveWorktree
	forceDeleteBranch  *usecase.ForceDeleteBranch
	detectWorktrees    *usecase.DetectWorktrees
	prefetchCreateBase *usecase.PrefetchCreateBase
	resolvePrBase      *usecase.ResolvePrBase
	resolveMrBase      *usecase.ResolveMrBase
```

Add the 7 handlers, following `GetStatus`'s exact translate-only shape:

```go
func (s *Server) CreateWorktree(ctx context.Context, req *gitgatewayv1.CreateWorktreeRequest) (*gitgatewayv1.CreateWorktreeResponse, error) {
	result, err := s.createWorktree.Execute(ctx, usecase.CreateWorktreeInput{
		ProjectID: req.GetProjectId(), RepoID: req.GetRepoId(), Branch: req.GetBranch(), BaseRef: req.GetBaseRef(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.CreateWorktreeResponse{WorktreeId: result.WorktreeID, Path: result.Path, HeadSha: result.HeadSHA}, nil
}

func (s *Server) RemoveWorktree(ctx context.Context, req *gitgatewayv1.RemoveWorktreeRequest) (*emptypb.Empty, error) {
	if err := s.removeWorktree.Execute(ctx, req.GetWorktreeId(), req.GetForce()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) ForceDeleteBranch(ctx context.Context, req *gitgatewayv1.ForceDeleteBranchRequest) (*emptypb.Empty, error) {
	if err := s.forceDeleteBranch.Execute(ctx, req.GetWorktreeId(), req.GetBranch()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) DetectWorktrees(ctx context.Context, req *gitgatewayv1.DetectWorktreesRequest) (*gitgatewayv1.DetectWorktreesResponse, error) {
	paths, err := s.detectWorktrees.Execute(ctx, req.GetRepoId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.DetectWorktreesResponse{OnDiskPaths: paths}, nil
}

func (s *Server) PrefetchCreateBase(ctx context.Context, req *gitgatewayv1.PrefetchCreateBaseRequest) (*gitgatewayv1.PrefetchCreateBaseResponse, error) {
	sha, err := s.prefetchCreateBase.Execute(ctx, req.GetRepoId(), req.GetBaseRef())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.PrefetchCreateBaseResponse{ResolvedSha: sha}, nil
}

func (s *Server) ResolvePrBase(ctx context.Context, req *gitgatewayv1.ResolvePrBaseRequest) (*gitgatewayv1.ResolveBaseResponse, error) {
	resolved, err := s.resolvePrBase.Execute(ctx, req.GetRepoId(), req.GetPrNumber())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.ResolveBaseResponse{BaseBranch: resolved.Branch, BaseSha: resolved.SHA}, nil
}

func (s *Server) ResolveMrBase(ctx context.Context, req *gitgatewayv1.ResolveMrBaseRequest) (*gitgatewayv1.ResolveBaseResponse, error) {
	resolved, err := s.resolveMrBase.Execute(ctx, req.GetRepoId(), req.GetMrNumber())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.ResolveBaseResponse{BaseBranch: resolved.Branch, BaseSha: resolved.SHA}, nil
}
```

Add `emptypb "google.golang.org/protobuf/types/known/emptypb"` to
`server.go`'s import block.

### Step 6 — `cmd/server/main.go`: dial `project-service`/`scm-integration-service`, construct and wire everything

`git-gateway-service`'s `cmd/server/main.go` currently dials ONLY
`infra-fleet-service`. This task adds two new outbound connections:

```go
	projectConn, err := grpcclient.Dial(cfg.ProjectServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing project-service: %w", err)
	}
	defer func() { _ = projectConn.Close() }()
	projectServiceClient := projectv1.NewProjectServiceClient(projectConn)
	projectClient := grpcclient.NewProjectClient(projectServiceClient)

	scmConn, err := grpcclient.Dial(cfg.SCMIntegrationServiceAddr)
	if err != nil {
		return fmt.Errorf("dialing scm-integration-service: %w", err)
	}
	defer func() { _ = scmConn.Close() }()
	scmIntegrationClient := scmintegrationv1.NewScmIntegrationServiceClient(scmConn)
	scmClient := scmclient.New(scmIntegrationClient)
```

Add `ProjectServiceAddr`/`SCMIntegrationServiceAddr string` fields to
`internal/config/config.go`'s `Config` struct (env vars, e.g.
`PROJECT_SERVICE_ADDR`/`SCM_INTEGRATION_SERVICE_ADDR`), mirroring
`InfraFleetServiceAddr`'s existing pattern in that file exactly.

Add the new imports:

```go
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/adapter/scmclient"
```

Construct and wire the 7 new usecases:

```go
createWorktreeUC := usecase.NewCreateWorktree(resolver, projectClient, local, relay)
removeWorktreeUC := usecase.NewRemoveWorktree(resolver, projectClient, local, relay)
forceDeleteBranchUC := usecase.NewForceDeleteBranch(resolver, local, relay)
detectWorktreesUC := usecase.NewDetectWorktrees(resolver, local, relay)
prefetchCreateBaseUC := usecase.NewPrefetchCreateBase(resolver, local, relay)
resolvePrBaseUC := usecase.NewResolvePrBase(scmClient, resolver, local, relay)
resolveMrBaseUC := usecase.NewResolveMrBase(scmClient, resolver, local, relay)
```

Pass all 7 into the existing `gitgatewaygrpc.New(...)` call, in the same
field order Step 5 added them to `Server`. Register health checks for both
new connections alongside the existing ones (`healthSrv.Register(...)`, if
this file has one — confirm against the actual file; `git-gateway-service`'s
`cmd/server/main.go` currently has no `healthSrv.Register` calls at all
per its "no database... /readyz reports healthy as soon as the process is
up" doc comment — if that's still true, downstream connection health for
the two new clients should still be registered the way `api-gateway`'s
`main.go` does it via `grpcConnHealthCheck`, since these ARE now real
outbound dependencies this service can't function without).

## Verify

```bash
cd /opt/repos/orca/backend-go/services/git-gateway-service
go build ./... && go vet ./...
```
