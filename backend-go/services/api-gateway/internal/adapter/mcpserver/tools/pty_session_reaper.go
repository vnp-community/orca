package tools

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

// ReapSession is the durable half of auto-stop. The in-process hook only helps
// when this replica holds the session's streams; after a crash, a redeploy or a
// session that expired while no replica held it, infra-fleet still lists the
// terminals the MCP session created (origin.mcpSessionId), and they are closed
// here. Run it from the consumer of orca.mcp.session.closed; it is idempotent,
// and closing an already-dead terminal is a no-op. Agent sessions have no list
// RPC yet, so a dead replica's agents are only reaped in-process (documented gap).
func (e *Executor) ReapSession(ctx context.Context, tenantID, userID, mcpSessionID string) error {
	e.sessions.CloseSession(mcpSessionID, "reaper")
	id := wscompat.Identity{TenantID: tenantID, UserID: userID}
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
		if _, err := e.disp.Dispatch(ctx, id, "terminal.close", mustArgs(map[string]any{"terminal": r.PtyID})); err != nil {
			e.log.Warn("mcp reaper: terminal.close failed", slog.String("pty", r.PtyID), slog.Any("error", err))
		}
	}
	return nil
}
