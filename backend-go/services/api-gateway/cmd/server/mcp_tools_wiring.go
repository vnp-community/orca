package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpmetrics"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcppolicy"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/tools"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

// mcpToolStack is the tool catalog + executor over the real channel registry.
type mcpToolStack struct {
	Catalog  *tools.Catalog
	Executor *tools.Executor
	PtyCfg   tools.PtyToolsConfig // terminal/agent tool limits (BE-MCP-SOL-009)
}

// buildMCPToolStack must run AFTER every non-mcp channel is registered (the
// catalog expands hard-deny patterns against the inventory). gate nil =
// fail-closed default (only risk=read tools run); governance supplies the
// real gate. Env: MCP_TOOL_PACKS_ENABLED (default "1"), MCP_TOOL_TIMEOUT,
// MCP_SCM_RATE_PER_MIN (30), MCP_PII_MASK (directory), MCP_SENSITIVE_PATH_EXTRA.
func buildMCPToolStack(reg *wscompat.Registry, gate mcpserver.PolicyGate, logger *slog.Logger) (*mcpToolStack, error) {
	cfg := tools.DefaultConfig()
	packs, err := tools.ParsePacks(os.Getenv("MCP_TOOL_PACKS_ENABLED"))
	if err != nil {
		return nil, err
	}
	cfg.Packs = packs
	if cfg, err = cfg.ApplyEnv(os.Getenv); err != nil {
		return nil, err
	}
	if v := os.Getenv("MCP_TOOL_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return nil, fmt.Errorf("MCP_TOOL_TIMEOUT: %q is not a positive duration", v)
		}
		cfg.ToolTimeout = d
	}
	if gate == nil {
		logger.Warn("no MCP policy gate wired: only read-only tools are available")
		gate = mcpserver.FailClosedGate{}
	}
	cat, err := tools.NewCatalog(tools.AllSpecs(), cfg, gate, reg.Channels())
	if err != nil {
		return nil, err
	}
	ptyCfg, err := tools.PtyToolsConfigFromEnv(os.Getenv)
	if err != nil {
		return nil, err
	}
	ex := tools.NewExecutor(cat, mcpmetrics.TraceDispatcher(reg), gate, nil, cfg, logger).WithPtyTools(ptyCfg)
	return &mcpToolStack{Catalog: cat, Executor: ex, PtyCfg: ptyCfg}, nil
}

// setWorktreeTargets tells terminal_start/agent_start how to find the host of a
// worktree (the same resolution the UI does), so SSH and dev-server worktrees
// work and nothing falls back to a local PTY. Call before serving.
func (s *mcpToolStack) setWorktreeTargets(t tools.WorktreeTargets) {
	s.PtyCfg.Targets = t
	s.Executor.WithPtyTools(s.PtyCfg)
}

// withMCPSessionClosed stops the terminals and agents of an MCP session when it
// ends for good (DELETE, user/admin close, expiry, close signal). Stopping runs
// off the request path: it may take a few seconds.
func withMCPSessionClosed(s *mcpToolStack) func(*mcpserver.Deps) {
	return func(d *mcpserver.Deps) {
		d.OnSessionClosed = func(row, reason string) { go s.Executor.CloseSession(row, reason) }
	}
}

// runMCPSessionReaper is the durable half of auto-stop: one replica per
// orca.mcp.session.closed event closes whatever infra-fleet still lists for that
// MCP session (crashed or redeployed replicas, sessions that expired while no
// replica held them). Blocks until ctx ends; run it in its own goroutine.
func runMCPSessionReaper(ctx context.Context, s *mcpToolStack, bus *commoneventbus.Consumer, logger *slog.Logger) {
	err := bus.Subscribe(ctx, "MCP", "api-gateway-mcp-reaper", "orca.mcp.session.closed", func(ctx context.Context, ev commoneventbus.Event) error {
		var p struct {
			SessionID string `json:"session_id"`
			UserID    string `json:"user_id"`
		}
		if json.Unmarshal(ev.Payload, &p) != nil || p.SessionID == "" || ev.TenantID == "" {
			return nil // malformed: redelivery would not help
		}
		rctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		return s.Executor.ReapSession(rctx, ev.TenantID, p.UserID, p.SessionID)
	})
	if err != nil && ctx.Err() == nil {
		logger.WarnContext(ctx, "mcp session reaper ended", slog.Any("error", err))
	}
}

// withMCPTools plugs the stack into mcpserver.Deps.
func withMCPTools(s *mcpToolStack) func(*mcpserver.Deps) {
	return func(d *mcpserver.Deps) {
		d.Catalog, d.Executor = s.Catalog, s.Executor
	}
}

// withMCPGuards plugs the governance hooks into the executor: kill-switch
// watching of running tools, prompt-injection framing of untrusted output and
// the never-dispatch fuse. WatchKill needs the real gate and kill guard.
func withMCPGuards(s *mcpToolStack, gate mcpserver.PolicyGate, guard *mcppolicy.KillGuard) {
	g := tools.Guards{WrapUntrusted: mcppolicy.WrapUntrusted, NeverDispatch: mcppolicy.NeverDispatch,
		SessionID: mcppolicy.SessionIDFromContext, ClientName: mcppolicy.ClientNameFromContext}
	if real, ok := gate.(*mcppolicy.Gate); ok && guard != nil {
		g.WatchKill = func(ctx context.Context, p mcpserver.Principal) (context.Context, context.CancelFunc) {
			return real.WatchKill(ctx, guard, p, 0)
		}
	}
	s.Executor.WithGuards(g)
}
