# TASK-WT-03-05: Extend `RemoveWorktree.Execute` with BR-WT-09/10 server-side guards + agent-kill step

**From Solution:** SOL-WT-03
**Priority:** P0
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/remove_worktree.go`
**Depends on:** TASK-WT-03-01, TASK-WT-03-02, TASK-WT-03-03
**Status:** `[x]` DONE — remove_worktree.go rewritten with BR-WT-09/10 guards + agent-kill step; gRPC handler updated for RemoveWorktreeResponse; wired terminalSessionLister in main.go. go build+test clean; remove_worktree_test.go updated for new signature + added uncommitted-changes-rejects-without-force case.

---

## Context

Today `RemoveWorktree.Execute` (`remove_worktree.go:33-45`) has no safety checks at all — it relies entirely on git's own dirty-worktree refusal, which `force=true` fully bypasses (this bug's own finding). This task re-runs the same checks `CheckWorktreeDeleteSafety` ([TASK-WT-03-04](./TASK-WT-03-04-usecase-check-delete-safety.md)) exposes as a server-side guard against a client that skips the pre-check call or races a change between check and confirm, per [SOL-WT-03](../solutions/SOL-WT-03-xoa-worktree.md).

## Changes to make

Replace `backend-go/services/git-gateway-service/internal/usecase/remove_worktree.go`:

```go
package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type RemoveWorktree struct {
	resolver  ConnectionResolver
	projects  ProjectClient
	local     GitExecutor
	relay     GitExecutor
	terminals TerminalSessionLister
}

func NewRemoveWorktree(resolver ConnectionResolver, projects ProjectClient, local, relay GitExecutor, terminals TerminalSessionLister) *RemoveWorktree {
	return &RemoveWorktree{resolver: resolver, projects: projects, local: local, relay: relay, terminals: terminals}
}

type RemoveWorktreeInput struct {
	WorktreeID string
	Force      bool
	StopAgents bool
}

func (uc *RemoveWorktree) Execute(ctx context.Context, in RemoveWorktreeInput) (domain.RemoveWorktreeResult, error) {
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.RemoveWorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}

	// BR-WT-09 — re-run the uncommitted-changes check server-side, don't
	// trust that the client already called CheckWorktreeDeleteSafety.
	var uncommittedCount int
	if status, err := executor.GetStatus(ctx, repoPath); err == nil {
		uncommittedCount = len(status.Files)
		if uncommittedCount > 0 && !in.Force {
			return domain.RemoveWorktreeResult{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKTREE_HAS_UNCOMMITTED_CHANGES",
				fmt.Sprintf("%d files uncommitted", uncommittedCount), nil)
		}
	}

	// BR-WT-10 — same re-check for active agent sessions.
	var stoppedPtyIDs []string
	if conn, cErr := uc.resolver.ResolveConnection(ctx, in.WorktreeID); cErr == nil && conn.Connected {
		if sessions, lErr := uc.terminals.ListSessions(ctx, conn.ConnectionID); lErr == nil {
			var active []string
			for _, s := range sessions {
				if strings.HasPrefix(s.Cwd, repoPath) {
					active = append(active, s.PtyID)
				}
			}
			if len(active) > 0 && !in.StopAgents {
				return domain.RemoveWorktreeResult{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKTREE_AGENT_RUNNING",
					fmt.Sprintf("%d active session(s) in this worktree", len(active)), nil)
			}
			for _, ptyID := range active {
				if err := uc.terminals.Kill(ctx, ptyID); err != nil {
					// Best-effort — a kill failure must not block a delete
					// the user explicitly confirmed; the orphaned PTY
					// self-heals when its process exits against a
					// now-removed cwd.
					continue
				}
				stoppedPtyIDs = append(stoppedPtyIDs, ptyID)
			}
		}
	}

	if err := executor.RemoveWorktree(ctx, repoPath, in.Force); err != nil {
		return domain.RemoveWorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_REMOVE_FAILED", "git worktree remove failed", err)
	}
	if err := uc.projects.RecordWorktreeRemoved(ctx, in.WorktreeID); err != nil {
		return domain.RemoveWorktreeResult{StoppedPtyIDs: stoppedPtyIDs}, apperrors.New(apperrors.KindInternal, "WORKTREE_BOOKKEEPING_STALE", "worktree removed but bookkeeping update failed; will self-heal via worktree.detectedList", err)
	}
	return domain.RemoveWorktreeResult{UncommittedFilesDiscarded: uncommittedCount, StoppedPtyIDs: stoppedPtyIDs}, nil
}
```

Add `domain.RemoveWorktreeResult` to `domain.go`:

```go
// RemoveWorktreeResult is RemoveWorktree's answer — UncommittedFilesDiscarded
// is only meaningful when Force was true (echoes what was overridden, for
// the UI's post-delete confirmation toast).
type RemoveWorktreeResult struct {
	UncommittedFilesDiscarded int
	StoppedPtyIDs             []string
}
```

Update the gRPC handler in `backend-go/services/git-gateway-service/internal/adapter/grpc/server.go` (`server.go:697-702`):

```go
func (s *Server) RemoveWorktree(ctx context.Context, req *gitgatewayv1.RemoveWorktreeRequest) (*gitgatewayv1.RemoveWorktreeResponse, error) {
	result, err := s.removeWorktree.Execute(ctx, usecase.RemoveWorktreeInput{
		WorktreeID: req.GetWorktreeId(), Force: req.GetForce(), StopAgents: req.GetStopAgents(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.RemoveWorktreeResponse{
		UncommittedFilesDiscarded: int32(result.UncommittedFilesDiscarded),
		StoppedPtyIds:             result.StoppedPtyIDs,
	}, nil
}
```

`NewRemoveWorktree`'s new `terminals TerminalSessionLister` param means its construction call site in `cmd/server/main.go` needs the same `infraclient.NewTerminalSessionLister(infraFleetClient)` value [TASK-WT-03-04](./TASK-WT-03-04-usecase-check-delete-safety.md) already introduces — construct it once, pass to both usecases.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/git-gateway-service/...
```

Expected: clean build once [TASK-WT-03-01](./TASK-WT-03-01-proto-delete-safety-and-stop-agents.md)'s regenerated stubs are in place. Behavior tests land in [TASK-WT-03-07](./TASK-WT-03-07-tests.md).
