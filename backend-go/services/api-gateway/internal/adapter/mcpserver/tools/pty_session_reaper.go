package tools

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

// ReapSession is the durable half of auto-stop. The in-process hook only helps
// when this replica holds the session's streams; after a crash, a redeploy or a
// session that expired while no replica held it, infra-fleet still lists the
// terminals the MCP session created (origin.mcpSessionId), and they are closed
// here. Run it from the consumer of orca.mcp.session.closed; it is idempotent,
// and closing an already-dead terminal is a no-op. Agent sessions are found
// through PtyToolsConfig.AgentLister (infra-fleet ListAgentSessions by origin);
// without a lister only the in-process agents of this replica are stopped.
func (e *Executor) ReapSession(ctx context.Context, tenantID, userID, mcpSessionID string) error {
	e.sessions.CloseSession(mcpSessionID, "reaper")
	id := wscompat.Identity{TenantID: tenantID, UserID: userID}
	agentErr := e.reapAgents(ctx, id, mcpSessionID)
	res, err := e.disp.Dispatch(ctx, id, "terminal.list", mustArgs(map[string]any{}))
	if err != nil {
		return err
	}
	b, _ := json.Marshal(res)
	var rows []struct {
		PtyID  string `json:"ptyId"`
		Origin *struct {
			Type         string `json:"type"`
			McpSessionID string `json:"mcpSessionId"`
		} `json:"origin"`
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		return err
	}
	for _, r := range rows {
		if r.Origin == nil || r.Origin.Type != "mcp" || r.Origin.McpSessionID != mcpSessionID {
			continue
		}
		if _, err := e.disp.Dispatch(ctx, id, "terminal.close", mustArgs(map[string]any{"terminal": r.PtyID, "reason": closeReasonSessionClosed})); err != nil {
			e.log.Warn("mcp reaper: terminal.close failed", slog.String("pty", r.PtyID), slog.Any("error", err))
		}
	}
	return agentErr
}

// AgentSessionRef is one live agent session of an MCP session.
type AgentSessionRef struct{ SessionID, Status string }

// AgentOriginLister lists the not-yet-finished agent sessions an MCP session
// started, from the durable store (infra-fleet), not from this replica's memory.
type AgentOriginLister interface {
	ListMcpAgentSessions(ctx context.Context, id wscompat.Identity, mcpSessionID string) ([]AgentSessionRef, error)
}

// reapAgents stops (politely, then SIGKILL after the grace period) every agent
// the closed MCP session left behind. Failures are logged per agent and the
// first listing error is returned so the event is redelivered.
func (e *Executor) reapAgents(ctx context.Context, id wscompat.Identity, mcpSessionID string) error {
	lister := e.sessions.cfg.AgentLister
	if lister == nil {
		return nil
	}
	agents, err := lister.ListMcpAgentSessions(ctx, id, mcpSessionID)
	if err != nil {
		return err
	}
	grace := e.sessions.cfg.AgentGrace
	var wg sync.WaitGroup
	for _, a := range agents {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := e.disp.Dispatch(ctx, id, "agent.stop", mustArgs(map[string]any{"sessionId": a.SessionID})); err != nil {
				e.log.Warn("mcp reaper: agent.stop failed", slog.String("agent", a.SessionID), slog.Any("error", err))
			}
			select {
			case <-time.After(grace):
			case <-ctx.Done():
			}
			// Killing an agent that already exited is a harmless no-op error.
			if _, err := e.disp.Dispatch(ctx, id, "agent.kill", mustArgs(map[string]any{"sessionId": a.SessionID, "signal": "SIGKILL"})); err != nil {
				e.log.Warn("mcp reaper: agent.kill failed", slog.String("agent", a.SessionID), slog.Any("error", err))
			}
		}()
	}
	wg.Wait()
	return nil
}
