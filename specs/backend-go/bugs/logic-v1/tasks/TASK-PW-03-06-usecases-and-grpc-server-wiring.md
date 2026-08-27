# TASK-PW-03-06: Usecases (mode-gated dispatch) + `grpc.Server` wiring for merge/stash/branch-create/soft-delete

**From Solution:** SOL-PW-03
**Priority:** P0
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/merge_branch.go` (+4 sibling files), `backend-go/services/git-gateway-service/internal/adapter/grpc/server.go`
**Depends on:** TASK-PW-03-04, TASK-PW-03-05
**Status:** `[ ]` TODO

---

## Context

One usecase per RPC, per this service's existing convention
(`abort_merge.go` is the shape to follow). Unlike `dispatchExecutor`'s
existing helper (`ports.go:383-392`, used by `AbortMerge` and others),
these five usecases need `ResolvedConnection.Mode` directly to fail
closed on `relay-ssh` *before* ever attempting the relay call — so they
call `resolver.ResolveConnection` inline instead of going through
`dispatchExecutor`.

## Changes to make

`internal/usecase/merge_branch.go` (representative; `stash_push.go`,
`stash_pop.go`, `create_branch.go`, `delete_branch.go` are the same body
shape with different `GitExecutor` methods/result types):

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type MergeBranchInput struct {
	WorktreeID string
	Branch     string
	NoFF       bool
}

type MergeBranch struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewMergeBranch(resolver ConnectionResolver, local, relay GitExecutor) *MergeBranch {
	return &MergeBranch{resolver: resolver, local: local, relay: relay}
}

func (uc *MergeBranch) Execute(ctx context.Context, in MergeBranchInput) (domain.MergeResult, error) {
	if in.WorktreeID == "" || in.Branch == "" {
		return domain.MergeResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_ARGS", "worktree_id and branch are required", nil)
	}
	conn, err := uc.resolver.ResolveConnection(ctx, in.WorktreeID)
	if err != nil {
		return domain.MergeResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	if conn.Connected && conn.Mode == infrafleetv1.ConnectionMode_CONNECTION_MODE_RELAY_SSH {
		return domain.MergeResult{}, apperrors.New(apperrors.KindFailedPrecondition, "GITGATEWAY_MERGE_UNSUPPORTED_SSH_RELAY", "merge is not supported over an SSH-relay connection", domain.ErrGitOpUnsupportedOverSSHRelay)
	}
	executor := uc.local
	if conn.Connected {
		executor = uc.relay
	}
	result, err := executor.MergeBranch(ctx, conn.RepoPath, in.Branch, in.NoFF)
	if err != nil {
		return domain.MergeResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_MERGE_FAILED", "failed to merge branch", err)
	}
	return result, nil
}
```

Wire all five into `internal/adapter/grpc/server.go`, following
`ForceDeleteBranch`'s existing shape (`server.go:704-753`):

```go
func (s *Server) MergeBranch(ctx context.Context, req *gitgatewayv1.MergeBranchRequest) (*gitgatewayv1.MergeBranchResponse, error) {
	result, err := s.mergeBranch.Execute(ctx, usecase.MergeBranchInput{
		WorktreeID: req.GetWorktreeId(), Branch: req.GetBranch(), NoFF: req.GetNoFf(),
	})
	if err != nil {
		return nil, toFileGRPCStatus(err) // or whatever this file's git-op status-mapping helper is named — verify before copying toFileGRPCStatus verbatim, it may be file-op-specific
	}
	return &gitgatewayv1.MergeBranchResponse{Success: result.Success, HadConflicts: result.HadConflicts}, nil
}
// StashPush/StashPop/CreateBranch/DeleteBranch follow the same shape.
```

Wire the five new usecase constructors into `cmd/server/main.go`'s
composition root, alongside the existing `NewAbortMerge`/
`NewForceDeleteBranch` wiring.

## Test plan (add to each usecase's `_test.go`)

Fake `ConnectionResolver`/`GitExecutor`: local dispatch calls the local
executor; relay dispatch with `Mode=CONNECTION_MODE_RELAY_WEBSOCKET`
calls the relay executor; relay dispatch with
`Mode=CONNECTION_MODE_RELAY_SSH` returns `ErrGitOpUnsupportedOverSSHRelay`
**without** calling the relay executor at all (assert the fake records
zero calls).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/git-gateway-service/...
go test ./services/git-gateway-service/internal/usecase/... -run 'TestMergeBranch|TestStashPush|TestStashPop|TestCreateBranch|TestDeleteBranch' -v
go test ./services/git-gateway-service/internal/adapter/grpc/... -run 'TestMergeBranch|TestStashPush|TestStashPop|TestCreateBranch|TestDeleteBranch' -v
```

Expected: clean build; the relay-ssh-mode zero-call regression guard
passes for all five usecases.
