package main

import (
	"context"
	"log/slog"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/prompts"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/resources"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

// mcpResourcePromptStack is resources/* and prompts/* over the real channel
// registry (BE-MCP-SOL-010/011).
type mcpResourcePromptStack struct {
	Resources *resources.Provider
	Prompts   *prompts.Provider
}

// buildMCPResourcePromptStack must run after the channels are registered.
// bus nil = no task event source, so resources/subscribe is not declared.
// client nil = built-in prompts only. gate nil = fail-closed default (reads
// still pass, they are risk=read). Env: MCP_RESOURCE_MAX_BYTES,
// MCP_RESOURCE_MAX_SUBS, MCP_RESOURCE_FILE_ENABLED (default false),
// MCP_SENSITIVE_PATH_EXTRA.
func buildMCPResourcePromptStack(reg *wscompat.Registry, gate mcpserver.PolicyGate, bus resources.EphemeralSubscriber,
	client mcpv1.McpServiceClient, logger *slog.Logger) *mcpResourcePromptStack {
	var events resources.TaskEventSource
	if bus != nil {
		events = resources.BusTaskEvents{Bus: bus}
	}
	res := resources.NewProvider(reg, gate, events, resources.ConfigFromEnv(), logger)
	var store prompts.Store
	if client != nil {
		store = prompts.GRPCStore{Client: client}
	}
	return &mcpResourcePromptStack{Resources: res, Prompts: prompts.NewProvider(store, res, logger)}
}

func withMCPResourcesPrompts(s *mcpResourcePromptStack) func(*mcpserver.Deps) {
	return func(d *mcpserver.Deps) {
		d.Resources, d.Prompts = s.Resources, s.Prompts
	}
}

// watchPromptChanges turns orca.mcp.prompt.changed (every replica sees it)
// into a cache drop and a prompts/list_changed notification. Blocks until ctx
// ends; run it in its own goroutine.
func (s *mcpResourcePromptStack) watchPromptChanges(ctx context.Context, bus resources.EphemeralSubscriber, h *mcpserver.Handler, logger *slog.Logger) {
	err := bus.SubscribeEphemeral(ctx, "MCP", "orca.mcp.prompt.changed", func(_ context.Context, ev commoneventbus.Event) error {
		s.Prompts.Invalidate(ev.TenantID)
		h.NotifyPromptsChanged()
		return nil
	})
	if err != nil && ctx.Err() == nil {
		logger.WarnContext(ctx, "mcp prompt change subscription ended", slog.Any("error", err))
	}
}
