# TASK-CLI-02-02: `GetAgentTerminalSession` usecase — worktree → live pty resolution

**From Solution:** SOL-CLI-02
**Priority:** P0 — `api-gateway`'s `resolveAgentPtyID` (TASK-CLI-02-05) is built on this
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/usecase/get_agent_terminal_session.go`
**Depends on:** TASK-CLI-02-01
**Status:** [x] DONE — `GetAgentTerminalSession` usecase added, wired into `cmd/server/main.go`; all 4 verify-listed test cases pass (exact match, subdirectory non-match, latest-`LastActiveAt` tie-break, no-connection-resolved).

---

## Context

`--worktree <id>` -> `ptyId` resolution is genuine business logic (matching by `cwd`, tie-breaking by `last_active_at`), so per `03-clean-architecture-guidelines.md`'s usecase-owns-business-decisions rule it belongs in a new `infra-fleet-service` usecase, not as `if`-logic in `api-gateway`'s REST handler. It composes two ports that already exist: `ConnectionResolver.ResolveConnection` (worktree_id -> connection_id/repo_path) and `TerminalSessionRepository.List` (connection_id -> open sessions).

## Changes to make

`backend-go/services/infra-fleet-service/internal/usecase/get_agent_terminal_session.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// GetAgentTerminalSession resolves worktreeID -> the live TerminalSession
// whose Cwd matches that worktree's resolved repo path. Backs the CLI's
// `--worktree <id>` flag (BUG-CLI-02) — closeable with zero new state,
// composing ConnectionResolver + TerminalSessionRepository the same way
// resolveTerminalSession composes them in the reverse direction (pty ->
// connection, here connection -> pty).
type GetAgentTerminalSession struct {
	resolver ConnectionResolver
	sessions TerminalSessionRepository
}

func NewGetAgentTerminalSession(resolver ConnectionResolver, sessions TerminalSessionRepository) *GetAgentTerminalSession {
	return &GetAgentTerminalSession{resolver: resolver, sessions: sessions}
}

func (uc *GetAgentTerminalSession) Execute(ctx context.Context, worktreeID string) (domain.TerminalSession, bool, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.TerminalSession{}, false, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	connected, _, conn, err := uc.resolver.ResolveConnectionByWorktree(ctx, tenantID, worktreeID)
	if err != nil {
		return domain.TerminalSession{}, false, apperrors.New(apperrors.KindInternal, "INFRA_RESOLVE_FAILED", "failed to resolve worktree's connection", err)
	}
	if !connected {
		return domain.TerminalSession{}, false, nil
	}

	sessions, err := uc.sessions.List(ctx, tenantID, conn.ID)
	if err != nil {
		return domain.TerminalSession{}, false, apperrors.New(apperrors.KindInternal, "INFRA_TERMINAL_LIST_FAILED", "failed to list terminal sessions", err)
	}

	var best domain.TerminalSession
	found := false
	for _, s := range sessions {
		// Exact match only — a subdirectory cwd is a different terminal
		// (e.g. the user cd'd into a subfolder), not "the" agent session.
		if s.Cwd != conn.RepoPath {
			continue
		}
		if !found || s.LastActiveAt.After(best.LastActiveAt) {
			best, found = s, true
		}
	}
	return best, found, nil
}
```

Wire into `internal/adapter/grpc/server.go`'s `Server` struct/`New(...)` and `cmd/server/main.go`'s DI construction (`usecase.NewGetAgentTerminalSession(repo, terminalSessionStore)`, matching `getTerminalAgentStatusUC`'s existing construction line).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/usecase/... -run TestGetAgentTerminalSession -v
```

Expected new test file `get_agent_terminal_session_test.go` (fake `ConnectionResolver`/`TerminalSessionRepository`): exact `cwd` match returns `found=true`; subdirectory `cwd` does not match; two sessions with matching `cwd` return the one with the later `LastActiveAt`; no connection resolved (`worktree_id` unknown) returns `found=false`, not an error.
