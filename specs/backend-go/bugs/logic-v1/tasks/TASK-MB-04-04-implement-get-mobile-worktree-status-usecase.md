# TASK-MB-04-04: Implement `GetMobileWorktreeStatus` usecase + `infra-fleet-service` client

**From Solution:** SOL-MB-04
**Priority:** P0
**Service:** `project-service`
**File:** `backend-go/services/project-service/internal/usecase/get_mobile_worktree_status.go`, `backend-go/services/project-service/internal/adapter/grpcclient/infra_fleet_terminal_status_resolver.go`
**Depends on:** TASK-MB-04-03, TASK-MB-04-02
**Status:** `[ ]` TODO

---

## Context

The worktree↔PTY correlation key is `Worktree.Path == TerminalSession.Cwd`
— a **string-equality correlation, not a foreign key** (neither domain
type has an FK to the other today). Flagged explicitly: a clean follow-up
(adding `worktree_id` to `SpawnTerminalSessionInput`/`terminal_sessions`)
would close this properly, but is not required to satisfy BL-MB-04's
response shape. `infra_fleet_dev_server_lister.go` (existing, in this same
`grpcclient` package) is the precedent this new client file follows.

`ListTerminalSessionsRequest` takes a `connection_id`, not a `dev_server_id`
— resolve `dev_server_id -> connection_id` first via the existing
`ResolveConnection` RPC (same one `channels_browser.go`/TASK-021's accounts
work already use), then call `ListTerminalSessions`.

## Changes to make

`backend-go/services/project-service/internal/adapter/grpcclient/infra_fleet_terminal_status_resolver.go`:

```go
package grpcclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// InfraFleetTerminalStatusResolver implements usecase.TerminalStatusResolver
// by resolving a dev server's active connectionId (ResolveConnection, the
// same RPC channels_browser.go already uses) then listing its terminal
// sessions — a thin client, mirrors InfraFleetDevServerLister's shape.
type InfraFleetTerminalStatusResolver struct {
	conn   *grpc.ClientConn
	client infrafleetv1.InfraFleetServiceClient
}

func NewInfraFleetTerminalStatusResolver(addr string) (*InfraFleetTerminalStatusResolver, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpcclient: dial infra-fleet-service at %q: %w", addr, err)
	}
	return &InfraFleetTerminalStatusResolver{conn: conn, client: infrafleetv1.NewInfraFleetServiceClient(conn)}, nil
}

func (c *InfraFleetTerminalStatusResolver) Close() error { return c.conn.Close() }

func (c *InfraFleetTerminalStatusResolver) ListSessionsForDevServer(ctx context.Context, devServerID string) ([]*infrafleetv1.TerminalSession, error) {
	resolved, err := c.client.ResolveConnection(ctx, &infrafleetv1.ResolveConnectionRequest{DevServerId: devServerID})
	if err != nil {
		return nil, fmt.Errorf("grpcclient: resolve connection for dev server %q: %w", devServerID, err)
	}
	if !resolved.GetConnected() {
		return nil, nil // not currently connected — no error, just no sessions
	}
	resp, err := c.client.ListTerminalSessions(ctx, &infrafleetv1.ListTerminalSessionsRequest{ConnectionId: resolved.GetDevServer().GetId()})
	if err != nil {
		return nil, fmt.Errorf("grpcclient: list terminal sessions: %w", err)
	}
	return resp.GetSessions(), nil
}
```

(Confirm the exact field `ResolveConnectionResponse` carries the resolved
`connectionId` in — the SOL's sketch and `channels_browser.go`'s existing
call may store it differently than `DevServer.GetId()`; read
`ResolveConnectionResponse`'s full message before finalizing this call.)

`backend-go/services/project-service/internal/usecase/get_mobile_worktree_status.go`:

```go
package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

type TerminalStatusResolver interface {
	ListSessionsForDevServer(ctx context.Context, devServerID string) ([]*infrafleetv1.TerminalSession, error)
}

type MobileWorktreeStatus struct {
	ID, Name, Agent, Status string
	DurationMs               int64
	LastOutput               string
}

type MobileStatusResult struct {
	Worktrees   []MobileWorktreeStatus
	GeneratedAt time.Time
}

type GetMobileWorktreeStatus struct {
	worktrees WorktreeRepository
	projects  ProjectRepository
	terminals TerminalStatusResolver
}

func NewGetMobileWorktreeStatus(worktrees WorktreeRepository, projects ProjectRepository, terminals TerminalStatusResolver) *GetMobileWorktreeStatus {
	return &GetMobileWorktreeStatus{worktrees: worktrees, projects: projects, terminals: terminals}
}

func (uc *GetMobileWorktreeStatus) Execute(ctx context.Context) (MobileStatusResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return MobileStatusResult{}, err
	}
	worktrees, err := uc.worktrees.ListActive(ctx, tenantID) // confirm actual method name on WorktreeRepository before wiring
	if err != nil {
		return MobileStatusResult{}, err
	}

	byDevServer := map[string][]domain.Worktree{}
	for _, wt := range worktrees {
		project, err := uc.projects.Get(ctx, tenantID, wt.ProjectID)
		if err != nil || project.DevServerID == "" {
			continue // no bound dev server: worktree has no runtime status to report, not an error
		}
		byDevServer[project.DevServerID] = append(byDevServer[project.DevServerID], wt)
	}

	out := make([]MobileWorktreeStatus, 0, len(worktrees))
	for devServerID, wts := range byDevServer {
		sessions, err := uc.terminals.ListSessionsForDevServer(ctx, devServerID)
		if err != nil {
			for _, wt := range wts {
				out = append(out, MobileWorktreeStatus{ID: wt.ID, Name: wt.Branch, Status: "unknown"}) // degraded dev server shouldn't fail the whole response
			}
			continue
		}
		byPath := indexSessionsByCwd(sessions)
		for _, wt := range wts {
			session, ok := byPath[wt.Path]
			if !ok {
				out = append(out, MobileWorktreeStatus{ID: wt.ID, Name: wt.Branch, Status: "idle"})
				continue
			}
			out = append(out, MobileWorktreeStatus{
				ID: wt.ID, Name: wt.Branch,
				Agent: agentKindFromSession(session), Status: statusFromSession(session),
				DurationMs: durationFromSession(session), LastOutput: session.GetLastOutputPreview(),
			})
		}
	}
	return MobileStatusResult{Worktrees: out, GeneratedAt: time.Now()}, nil
}

func indexSessionsByCwd(sessions []*infrafleetv1.TerminalSession) map[string]*infrafleetv1.TerminalSession {
	m := make(map[string]*infrafleetv1.TerminalSession, len(sessions))
	for _, s := range sessions {
		m[s.GetCwd()] = s
	}
	return m
}
```

`agentKindFromSession`/`statusFromSession`/`durationFromSession` are small
helpers — `TerminalSession` (the infrafleet proto message) does not itself
carry `AgentKind`/`AgentRunning`/`ReadyForInput` (those live on
`GetTerminalAgentStatusResponse`, a separate RPC per-`ptyId`). Either (a)
extend `ListTerminalSessions`'s response to also carry per-session
agent-status fields (a proto follow-up beyond TASK-MB-04-01's scope), or
(b) call `GetTerminalAgentStatus` once per matched session inside this
loop. Pick (b) for this task to avoid a second proto round-trip design —
document the extra per-session RPC cost in the usecase's doc comment.

Add `TerminalStatusResolver` to `usecase/ports.go`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/project-service/... && go vet ./services/project-service/...
go test ./services/project-service/internal/usecase/... -run GetMobileWorktreeStatus
```

Test cases: worktree with no bound dev server → omitted runtime fields, not
skipped from the list. Worktree whose `Path` matches no live session →
`Status: "idle"`. `ListSessionsForDevServer` erroring for one dev server →
that dev server's worktrees degrade to `"unknown"`, others unaffected.
`ListSessionsForDevServer` called exactly once per distinct `dev_server_id`
for N worktrees sharing a dev server (assert call count on the fake).
