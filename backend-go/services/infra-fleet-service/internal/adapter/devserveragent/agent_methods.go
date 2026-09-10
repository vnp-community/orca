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

// StreamAgentHooks subscribes to every agent.hook notification on
// devServer's persistent session — one long-lived subscription per
// devServer connection (not per AgentSession), consumed by
// usecase.RecordAgentHookProviderSession.
func (c *Client) StreamAgentHooks(ctx context.Context, devServer domain.DevServer) (<-chan usecase.AgentHookEvent, func(), error) {
	sess, err := c.getOrCreateSession(ctx, devServer)
	if err != nil {
		return nil, nil, err
	}
	raw := sess.subscribeAgentHooks()
	out := make(chan usecase.AgentHookEvent)
	go func() {
		defer close(out)
		for r := range raw {
			select {
			case out <- usecase.AgentHookEvent{WorktreeID: r.WorktreeID, PtyID: r.PtyID, ProviderSessionKey: r.ProviderSessionKey, ProviderSessionID: r.ProviderSessionID}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, func() { sess.unsubscribeAgentHooks(raw) }, nil
}
