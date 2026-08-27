# TASK-WT-02-03: Implement `WorktreeCreator`/`AgentSpawner`/`PromptInjector` against real gRPC clients

**From Solution:** SOL-WT-02
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/fanout/` (new package)
**Depends on:** TASK-WT-02-01
**Status:** `[x]` DONE — internal/adapter/fanout package created (GRPCWorktreeCreator/GRPCAgentSpawner/GRPCPromptInjector); field names verified against real generated protos (GetProjectRequest.Id, not ProjectId); builds clean

---

## Context

Per [SOL-WT-02](../solutions/SOL-WT-02-fan-out-worktree.md)'s "genuine extension is narrow" finding: every RPC these adapters call is already real (`git-gateway-service.CreateWorktree`, `project-service.GetProjectContext`, `infra-fleet-service.ResolveConnection`/`SpawnTerminalSession`/`AttachPty`) — this task only wraps them behind the three ports from [TASK-WT-02-01](./TASK-WT-02-01-usecase-ports-fanout.md). Confirm `project-service.GetProjectContext` and `infra-fleet-service.ResolveConnection`/`SpawnTerminalSession`/`AttachPty`'s exact request/response field names against the real proto before wiring (`proto/orca/project/v1/project.proto`, `proto/orca/infrafleet/v1/infrafleet.proto`) — the field names below match the RPCs confirmed real during this task set's research; verify `SpawnTerminalSessionRequest`'s `shell` field name if this task's implementation diverges from it.

## Changes to make

`backend-go/services/api-gateway/internal/adapter/fanout/worktree_creator.go` (new):

```go
package fanout

import (
	"context"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
)

type GRPCWorktreeCreator struct {
	client gitgatewayv1.GitGatewayServiceClient
}

func NewGRPCWorktreeCreator(client gitgatewayv1.GitGatewayServiceClient) *GRPCWorktreeCreator {
	return &GRPCWorktreeCreator{client: client}
}

func (w *GRPCWorktreeCreator) CreateWorktree(ctx context.Context, projectID, repoID, branch, baseRef string) (string, string, string, error) {
	resp, err := w.client.CreateWorktree(ctx, &gitgatewayv1.CreateWorktreeRequest{
		ProjectId: projectID, RepoId: repoID, Branch: branch, BaseRef: baseRef,
	})
	if err != nil {
		return "", "", "", err
	}
	return resp.GetWorktreeId(), resp.GetPath(), resp.GetHeadSha(), nil
}
```

`backend-go/services/api-gateway/internal/adapter/fanout/agent_spawner.go` (new):

```go
package fanout

import (
	"context"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
)

type GRPCAgentSpawner struct {
	projectClient projectv1.ProjectServiceClient
	infraClient   infrafleetv1.InfraFleetServiceClient
}

func NewGRPCAgentSpawner(projectClient projectv1.ProjectServiceClient, infraClient infrafleetv1.InfraFleetServiceClient) *GRPCAgentSpawner {
	return &GRPCAgentSpawner{projectClient: projectClient, infraClient: infraClient}
}

// SpawnAgentTerminal composes project-service's context resolution with
// infra-fleet-service's connect+spawn — mirrors project-service.md §2's own
// prescribed "resolve context here, then call the execution-owning
// service" two-step pattern.
func (s *GRPCAgentSpawner) SpawnAgentTerminal(ctx context.Context, projectID, worktreePath, agentType string) (string, string, error) {
	proj, err := s.projectClient.GetProject(ctx, &projectv1.GetProjectRequest{ProjectId: projectID})
	if err != nil {
		return "", "", err
	}
	conn, err := s.infraClient.ResolveConnection(ctx, &infrafleetv1.ResolveConnectionRequest{DevServerId: proj.GetProject().GetDevServerId()})
	if err != nil {
		return "", "", err
	}
	resp, err := s.infraClient.SpawnTerminalSession(ctx, &infrafleetv1.SpawnTerminalSessionRequest{
		ConnectionId: conn.GetConnectionId(),
		Cwd:          worktreePath,
		Shell:        agentLaunchCommand(agentType),
	})
	if err != nil {
		return "", "", err
	}
	return resp.GetSession().GetPtyId(), conn.GetConnectionId(), nil
}

// agentLaunchCommand is a small, explicit lookup table — not part of this
// saga's coordination logic, just the CLI command each supported agent
// type launches with. Extend as new agent types are supported.
func agentLaunchCommand(agentType string) string {
	switch agentType {
	case "claude":
		return "claude"
	case "codex":
		return "codex"
	default:
		return "" // empty = agent applies its own default, per SpawnTerminalSessionRequest.shell's doc comment
	}
}
```

`backend-go/services/api-gateway/internal/adapter/fanout/prompt_injector.go` (new):

```go
package fanout

import (
	"context"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

type GRPCPromptInjector struct {
	client infrafleetv1.InfraFleetServiceClient
}

func NewGRPCPromptInjector(client infrafleetv1.InfraFleetServiceClient) *GRPCPromptInjector {
	return &GRPCPromptInjector{client: client}
}

func (p *GRPCPromptInjector) InjectPrompt(ctx context.Context, connectionID, ptyID, prompt string) error {
	stream, err := p.client.AttachPty(ctx)
	if err != nil {
		return err
	}
	defer stream.CloseSend()
	if err := stream.Send(&infrafleetv1.PtyClientFrame{Frame: &infrafleetv1.PtyClientFrame_Attach{Attach: &infrafleetv1.AttachToSession{PtyId: ptyID}}}); err != nil {
		return err
	}
	return stream.Send(&infrafleetv1.PtyClientFrame{Frame: &infrafleetv1.PtyClientFrame_Input{Input: &infrafleetv1.PtyInput{Data: []byte(prompt + "\n")}}})
}
```

Before implementing, run:

```bash
cd /opt/repos/orca/backend-go
grep -n "message GetProjectRequest\|message GetProjectResponse\|dev_server_id" proto/orca/project/v1/project.proto
grep -n "message ResolveConnectionRequest\|message ResolveConnectionResponse\|message SpawnTerminalSessionRequest\|message PtyClientFrame\|message AttachToSession\|message PtyInput" proto/orca/infrafleet/v1/infrafleet.proto
```

and adjust field names above to match exactly — this task's snippets are grounded in this task set's own research but the exact accessor names (e.g. `GetDevServerId()`, `GetConnectionId()`) must match the generated stubs, not be assumed.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
```

Expected: clean build; `GRPCWorktreeCreator`/`GRPCAgentSpawner`/`GRPCPromptInjector` satisfy the `usecase.WorktreeCreator`/`AgentSpawner`/`PromptInjector` interfaces from [TASK-WT-02-01](./TASK-WT-02-01-usecase-ports-fanout.md).
