package mcpserver

import (
	"context"
	"errors"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// serverInstructions is read by the LLM client; keep it short and free of
// tenant data.
const serverInstructions = "Orca MCP server. Tools act on the authenticated user's Orca workspace " +
	"within the scopes granted to this client. Mutating tools may require user approval."

const principalExtraKey = "orca.principal"

// engine wires the official SDK server to our ports. The SDK owns JSON-RPC,
// the initialize/initialized lifecycle, ping, version negotiation and the
// Streamable HTTP transport; the adapter only overrides what must come from
// us (tools/list via ToolCatalog with signed cursors, tools/call via
// ToolExecutor, logging/setLevel validation).
type engine struct {
	catalog  ToolCatalog
	executor ToolExecutor
	cursors  *CursorCodec
	pageSize int
	log      *slog.Logger
	ready    *readySessions
	host     *sessionHost // session registry, cross-replica signals (BE-MCP-SOL-004)

	// Resources/prompts are optional (BE-MCP-SOL-010/011): nil = capability
	// not declared and methods fall through to the SDK's empty defaults.
	resources ResourceProvider
	prompts   PromptProvider
	server    *mcp.Server
	subs      subscribedSessions
	reqRec    RequestRecorder // optional; set by NewHandler from Deps.Recorder

	// toolsListChanged advertises tools.listChanged; set only when something
	// feeds Handler.NotifyToolsChanged (Config.ToolsListChanged).
	toolsListChanged bool
}

func newSDKServer(e *engine, cfg Config, log *slog.Logger, getSessionID func() string) *mcp.Server {
	srv := mcp.NewServer(
		&mcp.Implementation{Name: "orca", Title: "Orca", Version: cfg.ServerVersion},
		&mcp.ServerOptions{
			Instructions: serverInstructions,
			Logger:       log,
			GetSessionID: getSessionID,
			// Declare only what is truly implemented: tools/list + tools/call
			// (listChanged only when Config.ToolsListChanged is wired) and logging/setLevel.
			Capabilities:              e.capabilities(),
			SupportedProtocolVersions: sdkProtocolVersions(),
			SubscribeHandler:          e.subscribeHandler(),
			UnsubscribeHandler:        e.unsubscribeHandler(),
		})
	srv.AddReceivingMiddleware(e.observed)
	e.server = srv
	return srv
}

func principalOf(req mcp.Request) (Principal, bool) {
	if x := req.GetExtra(); x != nil && x.TokenInfo != nil {
		p, ok := x.TokenInfo.Extra[principalExtraKey].(Principal)
		return p, ok
	}
	return Principal{}, false
}

func invalidParams(msg string) error {
	return &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: msg}
}

func (e *engine) middleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		if err := e.sessionHook(ctx, method, req); err != nil {
			return nil, err
		}
		ctx, untrack := e.trackRequest(ctx, req)
		defer untrack()
		// The SDK treats a session as initialized as soon as `initialize`
		// arrives; the spec requires `notifications/initialized` first.
		switch method {
		case "initialize", "ping", "notifications/cancelled":
		case "notifications/initialized":
			e.ready.mark(req.GetSession().ID())
		default:
			if !e.ready.has(req.GetSession().ID()) {
				return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidRequest, Message: "server not initialized"}
			}
		}
		switch method {
		case "tools/list":
			return e.listTools(ctx, req)
		case "tools/call":
			return e.callTool(ctx, req)
		case "resources/list":
			if e.resources != nil {
				return e.listResources(ctx, req)
			}
		case "resources/templates/list":
			if e.resources != nil {
				return e.listResourceTemplates(ctx, req)
			}
		case "resources/read":
			if e.resources != nil {
				return e.readResource(ctx, req)
			}
		case "prompts/list":
			if e.prompts != nil {
				return e.listPrompts(ctx, req)
			}
		case "prompts/get":
			if e.prompts != nil {
				return e.getPrompt(ctx, req)
			}
		case "logging/setLevel":
			if p, ok := req.GetParams().(*mcp.SetLoggingLevelParams); !ok || !ValidLogLevel(string(p.Level)) {
				return nil, invalidParams("invalid logging level")
			}
		}
		return next(ctx, method, req)
	}
}

func (e *engine) listTools(ctx context.Context, req mcp.Request) (mcp.Result, error) {
	p, ok := principalOf(req)
	params, _ := req.GetParams().(*mcp.ListToolsParams)
	if params == nil { // params are optional for list methods
		params = &mcp.ListToolsParams{}
	}
	if !ok {
		return nil, &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "internal error"}
	}
	tools, err := e.catalog.ListTools(ctx, p)
	if err != nil {
		e.log.ErrorContext(ctx, "mcp tools/list: catalog failed", slog.Any("error", err))
		return nil, &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "internal error"}
	}
	page, next, err := Paginate(e.cursors, "tools", req.GetSession().ID(), params.Cursor, tools, e.pageSize)
	if err != nil {
		return nil, err
	}
	return &mcp.ListToolsResult{Tools: page, NextCursor: next}, nil
}

func (e *engine) callTool(ctx context.Context, req mcp.Request) (mcp.Result, error) {
	p, ok := principalOf(req)
	params, _ := req.GetParams().(*mcp.CallToolParamsRaw)
	if !ok || params == nil {
		return nil, &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "internal error"}
	}
	if e.executor == nil {
		return nil, invalidParams("unknown tool: " + params.Name)
	}
	res, err := e.executor.CallTool(e.toolContext(ctx, req, p, params), p, params.Name, params.Arguments)
	switch {
	case errors.Is(err, ErrUnknownTool):
		return nil, invalidParams("unknown tool: " + params.Name)
	case err != nil && ctx.Err() != nil:
		return nil, ctx.Err()
	case err != nil:
		// Business failures are tool results (isError), not protocol errors.
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: MapToolError(err)}}}, nil
	case res == nil:
		return nil, &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "internal error"}
	}
	return res, nil
}
