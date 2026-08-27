# TASK-WT-03-02: Add `TerminalSessionLister` port

**From Solution:** SOL-WT-03
**Priority:** P0
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/ports.go`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

[SOL-WT-03](../solutions/SOL-WT-03-xoa-worktree.md)'s agent-running check (spec step 2c) and stop-agents kill step need `infra-fleet-service.ListTerminalSessions`/`KillTerminalSession` — both already real RPCs (`infra-fleet-service.md:131-132`, confirmed in `infrafleet.proto`: `ListTerminalSessionsRequest{connection_id}` / `ListTerminalSessionsResponse{repeated TerminalSession sessions}` / `KillTerminalSessionRequest{pty_id}`). This adds one more call on the already-existing `git-gateway-service --> infra-fleet-service` dependency edge (`git-gateway-service.md` §7) — no new edge.

## Changes to make

Append to `backend-go/services/git-gateway-service/internal/usecase/ports.go`:

```go
// TerminalSessionLister wraps infra-fleet-service's ListTerminalSessions/
// KillTerminalSession — both already-real RPCs (infra-fleet-service.md
// :131-132) — reusing the existing git-gateway-service --> infra-fleet-service
// dependency edge (git-gateway-service.md §7), not a new one.
type TerminalSessionLister interface {
	ListSessions(ctx context.Context, connectionID string) ([]domain.TerminalSessionRef, error)
	Kill(ctx context.Context, ptyID string) error
}
```

Add `domain.TerminalSessionRef` to `backend-go/services/git-gateway-service/internal/domain/domain.go`:

```go
// TerminalSessionRef is one active PTY session, as reported by
// infra-fleet-service.ListTerminalSessions — the subset CheckWorktreeDeleteSafety/
// RemoveWorktree need to determine whether a session's cwd falls under a
// worktree's path.
type TerminalSessionRef struct {
	PtyID string
	Cwd   string
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/git-gateway-service/...
```

Expected: clean build (interface has no implementation yet — added in [TASK-WT-03-03](./TASK-WT-03-03-adapter-infraclient.md)).
