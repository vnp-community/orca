package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/prompts"
)

// BE-MCP-SOL-011 admin channels: mcp.admin.prompt.list / upsert / delete.
// Admin-only (checked here AND in mcp-service), exactly one object in args[0]
// (C10), errors through mcpChannelError (C4). Built-in prompts are shipped in
// the gateway binary and are read-only; custom prompts live in mcp-service.

// McpPromptArgument / McpPrompt mirror CONTRACT section 1.
type McpPromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type McpPrompt struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Version     int                 `json:"version"`
	UpdatedAt   string              `json:"updatedAt"`
	Arguments   []McpPromptArgument `json:"arguments"`
	Template    string              `json:"template"`
	Builtin     bool                `json:"builtin"`
}

const builtinIDPrefix = "builtin:"

// builtinUpdatedAt is the process start: built-ins change only with a release.
var builtinUpdatedAt = time.Now().UTC().Format(time.RFC3339)

func registerMcpPromptChannels(r *Registry, d McpChannelDeps) {
	r.Register("mcp.admin.prompt.list", mcpHandler(d, true, mcpPromptList(d)))
	r.Register("mcp.admin.prompt.upsert", mcpHandler(d, true, mcpPromptUpsert(d)))
	r.Register("mcp.admin.prompt.delete", mcpHandler(d, true, mcpPromptDelete(d)))
}

var errBuiltinReadonly = errors.New("MCP_PROMPT_BUILTIN_READONLY: built-in prompts cannot be changed")

func toMcpPrompt(p *mcpv1.CustomPrompt) McpPrompt {
	args := make([]McpPromptArgument, 0, len(p.GetArguments()))
	for _, a := range p.GetArguments() {
		args = append(args, McpPromptArgument{Name: a.GetName(), Description: a.GetDescription(), Required: a.GetRequired()})
	}
	return McpPrompt{ID: p.GetId(), Name: p.GetName(), Description: p.GetDescription(), Version: int(p.GetVersion()),
		UpdatedAt: tsString(p.GetUpdatedAt()), Arguments: args, Template: p.GetTemplate()}
}

func builtinPrompts() []McpPrompt {
	out := []McpPrompt{}
	for _, b := range prompts.Builtins() {
		args := make([]McpPromptArgument, 0, len(b.Arguments))
		for _, a := range b.Arguments {
			args = append(args, McpPromptArgument{Name: a.Name, Description: a.Description, Required: a.Required})
		}
		out = append(out, McpPrompt{ID: builtinIDPrefix + b.Name, Name: b.Name, Description: b.Description, Version: b.Version,
			UpdatedAt: builtinUpdatedAt, Arguments: args, Template: b.Template, Builtin: true})
	}
	return out
}

func mcpPromptList(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		c, err := mcpClient(d)
		if err != nil {
			return nil, err
		}
		ctx, cancel := mcpCtx(ctx, id)
		defer cancel()
		resp, err := c.ListPrompts(ctx, &mcpv1.ListPromptsRequest{})
		if err != nil {
			return nil, err
		}
		out := builtinPrompts() // built-ins first, then custom by name (the service sorts)
		for _, p := range resp.GetPrompts() {
			out = append(out, toMcpPrompt(p))
		}
		return out, nil
	}
}

func mcpPromptUpsert(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := mcpArgs[McpPrompt](args)
		if err != nil {
			return nil, err
		}
		if in.Builtin || strings.HasPrefix(in.ID, builtinIDPrefix) {
			return nil, errBuiltinReadonly
		}
		c, err := mcpClient(d)
		if err != nil {
			return nil, err
		}
		pa := make([]*mcpv1.PromptArgument, 0, len(in.Arguments))
		for _, a := range in.Arguments {
			pa = append(pa, &mcpv1.PromptArgument{Name: a.Name, Description: a.Description, Required: a.Required})
		}
		ctx, cancel := mcpCtx(ctx, id)
		defer cancel()
		// updatedAt/builtin from the client are ignored; version is the lock.
		resp, err := c.UpsertPrompt(ctx, &mcpv1.UpsertPromptRequest{Prompt: &mcpv1.CustomPrompt{
			Id: in.ID, Name: in.Name, Description: in.Description, Arguments: pa, Template: in.Template, Version: int32(in.Version),
		}})
		if err != nil {
			return nil, err
		}
		return toMcpPrompt(resp), nil
	}
}

func mcpPromptDelete(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := mcpArgs[struct {
			PromptID string `json:"promptId"`
		}](args)
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(in.PromptID, builtinIDPrefix) {
			return nil, errBuiltinReadonly
		}
		c, err := mcpClient(d)
		if err != nil {
			return nil, err
		}
		ctx, cancel := mcpCtx(ctx, id)
		defer cancel()
		if _, err := c.DeletePrompt(ctx, &mcpv1.DeletePromptRequest{PromptId: in.PromptID}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	}
}
