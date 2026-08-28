# TASK-WT-02-01: Define fan-out ports (`WorktreeCreator`/`AgentSpawner`/`PromptInjector`) in `api-gateway`

**From Solution:** SOL-WT-02
**Priority:** P0 — every other task in this set depends on these types
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/usecase/ports.go`
**Depends on:** none
**Status:** `[x]` DONE — WorktreeCreator/AgentSpawner/PromptInjector appended to ports.go; api-gateway builds clean

---

## Context

[SOL-WT-02](../solutions/SOL-WT-02-fan-out-worktree.md) resolves [BUG-WT-02](../BUG-WT-02-fan-out-not-implemented.md): no service coordinates "create N worktrees, spawn N agents, inject N prompts" today. `api-gateway` is the only node with a direct dependency edge to all three services this saga needs (`git-gateway-service`, `project-service`, `infra-fleet-service` — `specs/backend-go/tdd/architecture/02-microservices-decomposition.md:110-166`), so the saga's ports are defined in `api-gateway`'s own `internal/usecase/ports.go`, next to its existing thin ports (identity validation, rate limiting).

## Changes to make

Append to `backend-go/services/api-gateway/internal/usecase/ports.go`:

```go
// WorktreeCreator wraps git-gateway-service's already-real CreateWorktree
// RPC — see SOL-WT-01 for its validated shape. This saga only needs the
// project_id/repo_id/branch/base_ref subset.
type WorktreeCreator interface {
	CreateWorktree(ctx context.Context, projectID, repoID, branch, baseRef string) (worktreeID, path, headSHA string, err error)
}

// AgentSpawner composes project-service.GetProjectContext +
// infra-fleet-service's ResolveConnection/SpawnTerminalSession — "starting
// an agent" in this architecture is spawning a PTY running the agent's CLI
// command (business-capabilities.md's project.agentSpawn -> agent.exec
// framing).
type AgentSpawner interface {
	SpawnAgentTerminal(ctx context.Context, projectID, worktreePath, agentType string) (ptyID, connectionID string, err error)
}

// PromptInjector wraps infra-fleet-service's AttachPty bidirectional stream
// — opens it, sends AttachToSession{pty_id} then PtyInput{data: prompt},
// closes.
type PromptInjector interface {
	InjectPrompt(ctx context.Context, connectionID, ptyID, prompt string) error
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
```

Expected: clean build (these are new, unimplemented interfaces — nothing references them yet until [TASK-WT-02-02](./TASK-WT-02-02-usecase-fan-out-execute.md)/[TASK-WT-02-03](./TASK-WT-02-03-adapter-fanout-implementations.md)).
