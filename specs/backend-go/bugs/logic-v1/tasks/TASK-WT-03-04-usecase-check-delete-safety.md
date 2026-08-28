# TASK-WT-03-04: `CheckWorktreeDeleteSafety` usecase + gRPC handler

**From Solution:** SOL-WT-03
**Priority:** P0
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/check_worktree_delete_safety.go` (new)
**Depends on:** TASK-WT-03-01, TASK-WT-03-02, TASK-WT-03-03
**Status:** `[ ]` TODO

---

## Context

The read RPC a client calls before rendering the delete-confirm dialog. Uncommitted/untracked counts come from `GitExecutor.GetStatus` (already required, `ports.go:62`); the agent-running heuristic filters `TerminalSessionLister.ListSessions` by `cwd` prefix — flagged as imprecise per [SOL-WT-03](../solutions/SOL-WT-03-xoa-worktree.md) (a session could be an ordinary shell, not an AI-CLI process; closing that gap needs `SpawnTerminalSession`-time tagging, out of scope here).

## Changes to make

Create `backend-go/services/git-gateway-service/internal/usecase/check_worktree_delete_safety.go`:

```go
package usecase

import (
	"context"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type CheckWorktreeDeleteSafety struct {
	resolver  ConnectionResolver
	local     GitExecutor
	relay     GitExecutor
	terminals TerminalSessionLister
}

func NewCheckWorktreeDeleteSafety(resolver ConnectionResolver, local, relay GitExecutor, terminals TerminalSessionLister) *CheckWorktreeDeleteSafety {
	return &CheckWorktreeDeleteSafety{resolver: resolver, local: local, relay: relay, terminals: terminals}
}

func (uc *CheckWorktreeDeleteSafety) Execute(ctx context.Context, worktreeID string) (domain.DeleteSafetyReport, error) {
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return domain.DeleteSafetyReport{}, apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}
	status, err := executor.GetStatus(ctx, repoPath)
	if err != nil {
		return domain.DeleteSafetyReport{}, apperrors.New(apperrors.KindInternal, "WORKTREE_STATUS_FAILED", "failed to check worktree status", err)
	}

	report := domain.DeleteSafetyReport{}
	for _, f := range status.Files {
		if f.State == domain.FileStateUntracked {
			report.UntrackedFiles++
		} else {
			report.UncommittedFiles++ // modified/added/deleted/conflicted
		}
	}

	conn, err := uc.resolver.ResolveConnection(ctx, worktreeID)
	if err == nil && conn.Connected {
		if sessions, listErr := uc.terminals.ListSessions(ctx, conn.ConnectionID); listErr == nil {
			for _, s := range sessions {
				if strings.HasPrefix(s.Cwd, repoPath) {
					report.ActivePtyIDs = append(report.ActivePtyIDs, s.PtyID)
				}
			}
		}
	}
	report.AgentRunning = len(report.ActivePtyIDs) > 0
	report.SafeToDelete = report.UncommittedFiles == 0 && report.UntrackedFiles == 0 && !report.AgentRunning
	return report, nil
}
```

Add `domain.DeleteSafetyReport` to `backend-go/services/git-gateway-service/internal/domain/domain.go`:

```go
// DeleteSafetyReport is CheckWorktreeDeleteSafety's answer — see that
// usecase's doc comment for AgentRunning's heuristic-not-precise caveat.
type DeleteSafetyReport struct {
	UncommittedFiles int
	UntrackedFiles   int
	AgentRunning     bool
	ActivePtyIDs     []string
	SafeToDelete     bool
}
```

Add the gRPC handler to `backend-go/services/git-gateway-service/internal/adapter/grpc/server.go` (near the existing `RemoveWorktree` handler, `server.go:697`), and thread `checkWorktreeDeleteSafety *usecase.CheckWorktreeDeleteSafety` through the `Server` struct + `New(...)` constructor the same way every other usecase in that file is wired:

```go
func (s *Server) CheckWorktreeDeleteSafety(ctx context.Context, req *gitgatewayv1.CheckWorktreeDeleteSafetyRequest) (*gitgatewayv1.CheckWorktreeDeleteSafetyResponse, error) {
	report, err := s.checkWorktreeDeleteSafety.Execute(ctx, req.GetWorktreeId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.CheckWorktreeDeleteSafetyResponse{
		UncommittedFiles: int32(report.UncommittedFiles),
		UntrackedFiles:   int32(report.UntrackedFiles),
		AgentRunning:     report.AgentRunning,
		ActivePtyIds:     report.ActivePtyIDs,
		SafeToDelete:     report.SafeToDelete,
	}, nil
}
```

Wire `checkWorktreeDeleteSafety` into `cmd/server/main.go`'s existing `grpc.New(...)` call (add the new usecase construction: `usecase.NewCheckWorktreeDeleteSafety(resolver, localExecutor, relayExecutor, infraclient.NewTerminalSessionLister(infraFleetClient))`, next to where `removeWorktree`/`createWorktree` are already constructed).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/git-gateway-service/...
```

Expected: clean build. Behavior tests land in [TASK-WT-03-07](./TASK-WT-03-07-tests.md).
