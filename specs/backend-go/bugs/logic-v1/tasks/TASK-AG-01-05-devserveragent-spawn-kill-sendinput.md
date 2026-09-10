# TASK-AG-01-05: Implement `SpawnAgent`/`KillAgent`/`SendAgentInput` in `adapter/devserveragent`

**From Solution:** SOL-AG-01
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/agent_methods.go` (new)
**Depends on:** TASK-AG-01-02
**Status:** `[x]` DONE — agent_methods.go adds SpawnAgent/KillAgent/SendAgentInput; agent_methods_test.go (fake-agent-over-real-websocket, mirrors methods_test.go's pattern) asserts exact method names/param shapes and result decoding — all passing.

---

## Context

Typed wrappers over the agent's real `agent.spawn`/`agent.kill`/`agent.sendInput` JSON-RPC methods (confirmed live in `agent/src/relay/agent-rpc-dispatch.ts`), following the exact `sess.call(ctx, "<method>", params)` pattern the sibling `pty.*` wrappers in `methods.go` already use.

## Changes to make

Create `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/agent_methods.go`:

```go
// Package devserveragent — agent_methods.go adds typed wrappers over the
// agent's real agent.spawn/agent.kill/agent.sendInput JSON-RPC methods
// (agent/src/relay/agent-rpc-dispatch.ts's 'agent.spawn'/'agent.kill'/
// 'agent.sendInput' cases), alongside methods.go's pty.* wrappers. Agent-
// spawned PTYs live in agent-spawner.ts's own PTY_REGISTRY, a separate
// store from the pty-daemon's — pty.* RPCs do not reach them, so these are
// NOT thin wrappers around SpawnPty/KillPty/WritePty.
package devserveragent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

// SpawnAgent calls agent.spawn. Params mirror agent-spawner.ts's
// AgentSpawnRequest field names exactly (taskId, userId, model, accountId,
// cwd, resumeId, worktreePath, branchName, cols, rows, trustPreset) —
// resolvedApiKey is deliberately never populated here, see TASK-AG-01-04.
func (c *Client) SpawnAgent(ctx context.Context, devServer domain.DevServer, in usecase.SpawnAgentInput) (usecase.SpawnAgentResult, error) {
	sess, err := c.getOrCreateSession(ctx, devServer)
	if err != nil {
		return usecase.SpawnAgentResult{}, err
	}

	params := map[string]any{
		"taskId": in.TaskID,
		"userId": in.UserID,
		"model":  in.ModelID,
		"cwd":    in.Cwd,
		"cols":   in.Cols,
		"rows":   in.Rows,
	}
	if in.AccountID != "" {
		params["accountId"] = in.AccountID
	}
	if in.ResumeID != "" {
		params["resumeId"] = in.ResumeID
	}
	if in.WorktreePath != "" {
		params["worktreePath"] = in.WorktreePath
	}
	if in.BranchName != "" {
		params["branchName"] = in.BranchName
	}
	if in.TrustPreset != "" {
		params["trustPreset"] = in.TrustPreset
	}

	raw, err := sess.call(ctx, "agent.spawn", params)
	if err != nil {
		return usecase.SpawnAgentResult{}, err
	}

	var out struct {
		PtyID string `json:"ptyId"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			return usecase.SpawnAgentResult{}, fmt.Errorf("devserveragent: decoding agent.spawn result: %w", err)
		}
	}
	if out.PtyID == "" {
		return usecase.SpawnAgentResult{}, fmt.Errorf("devserveragent: agent.spawn response missing ptyId")
	}
	return usecase.SpawnAgentResult{PtyID: out.PtyID}, nil
}

// KillAgent calls agent.kill. Params: { id, signal } — matches
// handleAgentKill's read of the PTY_REGISTRY entry by id.
func (c *Client) KillAgent(ctx context.Context, devServer domain.DevServer, ptyID, signal string) error {
	sess, err := c.getOrCreateSession(ctx, devServer)
	if err != nil {
		return err
	}
	if signal == "" {
		signal = "SIGKILL"
	}
	_, err = sess.call(ctx, "agent.kill", map[string]any{"id": ptyID, "signal": signal})
	return err
}

// SendAgentInput calls agent.sendInput. Params: { id, data } — data is
// forwarded as raw bytes (string-encoded, matching WritePty's convention).
func (c *Client) SendAgentInput(ctx context.Context, devServer domain.DevServer, ptyID string, data []byte) error {
	sess, err := c.getOrCreateSession(ctx, devServer)
	if err != nil {
		return err
	}
	_, err = sess.call(ctx, "agent.sendInput", map[string]any{"id": ptyID, "data": string(data)})
	return err
}
```

This matches `methods.go`'s existing inline-`json.Unmarshal`-per-call style
(`SpawnPty`) — there is no shared decode helper in this package today, so
don't introduce one here.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/devserveragent/... -run TestSpawnAgent -v
go test ./services/infra-fleet-service/internal/adapter/devserveragent/... -run TestKillAgent -v
go test ./services/infra-fleet-service/internal/adapter/devserveragent/... -run TestSendAgentInput -v
```

Add `agent_methods_test.go` alongside the existing `methods_test.go`
asserting each call sends the exact `agent.spawn`/`agent.kill`/
`agent.sendInput` method name and param keys listed above, and decodes
`{ok, ptyId}` from a fake session transport (reuse whatever fake/mock the
existing `methods_test.go` already sets up for `SpawnPty`).
