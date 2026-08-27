# TASK-TM-03-08: Add `terminal.scrollback.*` wscompat channels + `RemoveWorktree` cleanup hook

**From Solution:** SOL-TM-03
**Priority:** P1
**Service:** `api-gateway` + `git-gateway-service`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal_scrollback.go`
**Depends on:** TASK-TM-03-03 (proto), TASK-TM-03-07 (server RPC handlers live)
**Status:** `[ ]` TODO

---

## Context

Two independent, small pieces of client-facing wiring that both depend on
the new proto RPCs being live:

1. `api-gateway` exposes `terminal.scrollback.save`/`terminal.scrollback.restore`
   as plain JSON wscompat channels — deliberately NOT part of
   `terminal.multiplex`'s binary opcode set (see SOL-TM-03's "two distinct
   snapshot mechanisms" rationale): this fires once per pane
   teardown/restore, not per keystroke, so there is no low-latency
   requirement forcing the binary framing.
2. `git-gateway-service`'s `RemoveWorktree` gains a best-effort call to the
   new `DeleteTerminalScrollbackSnapshots` RPC so a removed worktree's
   snapshot rows don't silently survive to the BR-TM-12 sweep 30 days
   later. Failure to clean up is logged, not surfaced as a `RemoveWorktree`
   error — worktree removal must not fail because this housekeeping call
   failed.

## Changes to make

### 1. `api-gateway` wscompat channels

Create `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal_scrollback.go`:

```go
// channels_terminal_scrollback.go registers terminal.scrollback.save /
// terminal.scrollback.restore — deliberately NOT part of
// terminal.multiplex's opcode set (see SOL-TM-03's "two distinct snapshot
// mechanisms" rationale: that protocol's SnapshotRequest resolves against a
// LIVE ptyId this flow structurally cannot have). Plain JSON channels,
// matching terminal.create/terminal.list's shape — this fires once per
// pane teardown/restore, not per keystroke, so there is no low-latency
// requirement forcing terminal.multiplex's binary framing.
package wscompat

import (
	"context"
	"encoding/json"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

type scrollbackSaveArgs struct {
	WorktreeID string `json:"worktreeId"`
	PaneKey    string `json:"paneKey"`
	Cols       int32  `json:"cols"`
	Rows       int32  `json:"rows"`
	Data       string `json:"data"` // xterm SerializeAddon output, UTF-8 text
	LastTitle  string `json:"lastTitle"`
}

type scrollbackRestoreArgs struct {
	WorktreeID string `json:"worktreeId"`
	PaneKey    string `json:"paneKey"`
}

func registerTerminalScrollbackChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("terminal.scrollback.save", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[scrollbackSaveArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		_, err = client.SaveTerminalScrollbackSnapshot(ctx, &infrafleetv1.SaveTerminalScrollbackSnapshotRequest{
			WorktreeId: in.WorktreeID, PaneKey: in.PaneKey, Cols: in.Cols, Rows: in.Rows,
			Data: []byte(in.Data), LastTitle: in.LastTitle,
		})
		return map[string]bool{"ok": err == nil}, err
	})

	r.Register("terminal.scrollback.restore", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[scrollbackRestoreArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.GetTerminalScrollbackSnapshot(ctx, &infrafleetv1.GetTerminalScrollbackSnapshotRequest{WorktreeId: in.WorktreeID, PaneKey: in.PaneKey})
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"found": resp.GetFound(), "cols": resp.GetCols(), "rows": resp.GetRows(),
			"data": string(resp.GetData()), "lastTitle": resp.GetLastTitle(),
			"updatedAt": resp.GetUpdatedAtUnixMs(),
		}, nil
	})
}
```

In `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`,
add the registration call next to the existing `registerTerminalChannels`
line (~line 124):

```go
registerTerminalChannels(r, infraFleetClient)
registerTerminalScrollbackChannels(r, infraFleetClient)
```

### 2. `git-gateway-service` cleanup hook

Add a small port in `backend-go/services/git-gateway-service/internal/usecase/ports.go`,
near `ProjectClient`:

```go
// ScrollbackCleaner wraps infra-fleet-service's DeleteTerminalScrollbackSnapshots
// RPC — called best-effort by RemoveWorktree; see that usecase's doc comment.
type ScrollbackCleaner interface {
	DeleteTerminalScrollbackSnapshots(ctx context.Context, worktreeID string) error
}
```

Implement it in `backend-go/services/git-gateway-service/internal/adapter/grpcclient/scrollback_cleaner.go`,
reusing the already-dialed `infraFleetClient` connection (`grpcclient.Dial`,
see `resolver.go`'s doc comment — this service already talks to
infra-fleet-service for connection resolution and relay dispatch):

```go
package grpcclient

import (
	"context"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// ScrollbackCleaner implements usecase.ScrollbackCleaner against
// infra-fleet-service's DeleteTerminalScrollbackSnapshots RPC — shares the
// same *grpc.ClientConn as ConnectionResolver/RelayExecutor.
type ScrollbackCleaner struct {
	client infrafleetv1.InfraFleetServiceClient
}

func NewScrollbackCleaner(client infrafleetv1.InfraFleetServiceClient) *ScrollbackCleaner {
	return &ScrollbackCleaner{client: client}
}

func (c *ScrollbackCleaner) DeleteTerminalScrollbackSnapshots(ctx context.Context, worktreeID string) error {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return err
	}
	_, err = c.client.DeleteTerminalScrollbackSnapshots(ctx, &infrafleetv1.DeleteTerminalScrollbackSnapshotsRequest{WorktreeId: worktreeID})
	return err
}
```

In `backend-go/services/git-gateway-service/internal/usecase/remove_worktree.go`,
add the dependency and the best-effort call, logged not surfaced:

```go
type RemoveWorktree struct {
	resolver   ConnectionResolver
	projects   ProjectClient
	scrollback ScrollbackCleaner
	local      GitExecutor
	relay      GitExecutor
}

func NewRemoveWorktree(resolver ConnectionResolver, projects ProjectClient, scrollback ScrollbackCleaner, local, relay GitExecutor) *RemoveWorktree {
	return &RemoveWorktree{resolver: resolver, projects: projects, scrollback: scrollback, local: local, relay: relay}
}

func (uc *RemoveWorktree) Execute(ctx context.Context, worktreeID string, force bool) error {
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}
	if err := executor.RemoveWorktree(ctx, repoPath, force); err != nil {
		return apperrors.New(apperrors.KindInternal, "WORKTREE_REMOVE_FAILED", "git worktree remove failed", err)
	}
	if err := uc.projects.RecordWorktreeRemoved(ctx, worktreeID); err != nil {
		return apperrors.New(apperrors.KindInternal, "WORKTREE_BOOKKEEPING_STALE", "worktree removed but bookkeeping update failed; will self-heal via worktree.detectedList", err)
	}
	// Best-effort: an orphaned scrollback row is caught by BR-TM-12's 30-day
	// sweep either way, so a cleanup failure here must not fail the worktree
	// removal itself.
	if err := uc.scrollback.DeleteTerminalScrollbackSnapshots(ctx, worktreeID); err != nil {
		// TODO: thread a structured logger into RemoveWorktree if one isn't
		// already available at this call site; log.Printf is a placeholder.
		log.Printf("remove_worktree: best-effort scrollback cleanup failed for worktree %s: %v", worktreeID, err)
	}
	return nil
}
```

Wire `grpcclient.NewScrollbackCleaner(infraFleetClient)` into
`usecase.NewRemoveWorktree(...)`'s call site in
`backend-go/services/git-gateway-service/cmd/server/main.go` (the existing
`infraFleetClient` variable there already exists for `ConnectionResolver`/
`RelayExecutor` — reuse it, do not dial a second connection).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/... ./services/git-gateway-service/...
```

Add `wscompat/channels_terminal_scrollback_test.go`: `terminal.scrollback.save`
round-trips through a fake `InfraFleetServiceClient`; `terminal.scrollback.restore`
on a never-saved pane returns `{found: false}`, not an error.

Extend `git-gateway-service`'s `remove_worktree_test.go`: a case asserting
`DeleteTerminalScrollbackSnapshots` is called with the removed worktree's
ID, and that a cleanup RPC failure does not fail `RemoveWorktree` itself
(the overall `Execute` call still returns nil).

```bash
go test ./services/api-gateway/internal/adapter/wscompat/... -run TestTerminalScrollback -v
go test ./services/git-gateway-service/internal/usecase/... -run TestRemoveWorktree -v
```

Expected: clean build, all cases pass.
