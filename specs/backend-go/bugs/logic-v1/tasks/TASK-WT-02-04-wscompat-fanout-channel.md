# TASK-WT-02-04: `worktree.fanOut` wscompat channel + composition-root wiring

**From Solution:** SOL-WT-02
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go`
**Depends on:** TASK-WT-02-02, TASK-WT-02-03
**Status:** `[ ]` TODO

---

## Context

Wires the saga up to the WS surface. `registerWorktreeChannels` is already called from `RegisterRealChannels` (`channels.go:119`) with `gitClient`/`projectClient` — this task adds `infraClient` as a third parameter (not currently threaded into `channels_worktree.go`) plus the constructed `*usecase.FanOutCreateWorktrees`.

## Changes to make

`backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go` — change `registerWorktreeChannels`'s signature and add the new channel:

```go
func registerWorktreeChannels(
	r *Registry,
	gitClient gitgatewayv1.GitGatewayServiceClient,
	projectClient projectv1.ProjectServiceClient,
	fanOutUseCase *usecase.FanOutCreateWorktrees,
) {
	// ...existing 8 channels unchanged...

	r.Register("worktree.fanOut", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type fanOutArgs struct {
			ProjectID    string `json:"projectId"`
			RepoID       string `json:"repoId"`
			BaseRef      string `json:"baseRef"`
			BranchPrefix string `json:"branchPrefix"`
			Prompt       string `json:"prompt"`
			AgentType    string `json:"agentType"`
			N            int    `json:"n"`
		}
		in, err := decodeArg[fanOutArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		results, err := fanOutUseCase.Execute(ctx, usecase.FanOutCreateWorktreesInput{
			ProjectID: in.ProjectID, RepoID: in.RepoID, BaseRef: in.BaseRef,
			BranchPrefix: in.BranchPrefix, Prompt: in.Prompt, N: in.N, AgentType: in.AgentType,
		})
		if err != nil {
			return nil, err // BR-WT-05 violation (n out of [1,10]) surfaces here, before any item runs
		}
		return map[string]any{"items": results}, nil
	})
}
```

Update `channels.go`'s call site (`channels.go:119`):

```go
	registerWorktreeChannels(r, gitClient, projectClient, fanOutUseCase)
```

`fanOutUseCase` is a new parameter `RegisterRealChannels` takes (add it to that function's own parameter list, alongside `infraFleetClient` etc. — `channels.go:71-86`), constructed once at `api-gateway`'s composition root in `cmd/server/main.go`:

```go
	fanOutUseCase := usecase.NewFanOutCreateWorktrees(
		fanout.NewGRPCWorktreeCreator(gitClient),
		fanout.NewGRPCAgentSpawner(projectClient, infraFleetClient),
		fanout.NewGRPCPromptInjector(infraFleetClient),
	)
```

placed alongside `main.go`'s existing `gitClient`/`projectClient`/`infraFleetClient` dial calls (`main.go:151` dials `infraFleetClient`), then passed into the existing `wscompat.RegisterRealChannels(...)` call.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
```

Expected: clean build. Channel-level test lands in [TASK-WT-02-06](./TASK-WT-02-06-tests-adapter-and-channel.md).
