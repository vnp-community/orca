package mcpserver

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Resources and prompts (BE-MCP-SOL-010/011) are plugged in through these
// ports; implementations live in mcpserver/resources and mcpserver/prompts so
// this package never imports them (and never the wscompat registry).

var (
	// ErrResourceNotFound is the ONE answer for "malformed", "does not exist",
	// "not yours" and "denied by policy": callers must not be able to tell
	// them apart.
	ErrResourceNotFound = errors.New("mcpserver: resource not found")
	// ErrSubscriptionUnsupported: the resource exists but is not subscribable.
	ErrSubscriptionUnsupported = errors.New("mcpserver: subscription not supported for this resource")
	// ErrSubscriptionLimit: the session reached its subscription quota.
	ErrSubscriptionLimit = errors.New("mcpserver: too many subscriptions")
)

// ResourceNotifier is how a ResourceProvider reaches MCP sessions. The SDK can
// only broadcast resources/updated to every session subscribed to a URI, so
// a session that lost its right to a resource is closed instead.
type ResourceNotifier interface {
	// ResourceUpdated sends notifications/resources/updated{uri} (URI only).
	ResourceUpdated(uri string)
	CloseSession(sessionID string)
}

// ResourceProvider serves resources/* (BE-MCP-SOL-010). Every method that
// takes a Principal must authorize with the same PolicyGate as a read tool.
type ResourceProvider interface {
	ListResources(ctx context.Context, p Principal) ([]*mcp.Resource, error)
	ListTemplates(ctx context.Context, p Principal) ([]*mcp.ResourceTemplate, error)
	// ReadResource returns ErrResourceNotFound for every kind of "no".
	ReadResource(ctx context.Context, p Principal, sessionID, uri string) (*mcp.ReadResourceResult, error)
	// Subscribe authorizes uri like a read, then arms its event source.
	Subscribe(ctx context.Context, p Principal, sessionID, uri string) error
	Unsubscribe(sessionID, uri string)
	// EndSession drops every subscription of a finished session.
	EndSession(sessionID string)
	// Bind is called once by the handler before serving.
	Bind(n ResourceNotifier)
	// Subscribable reports whether subscribe/listChanged are truly implemented
	// (capabilities are declared from this, never assumed).
	Subscribable() bool
}

// PromptProvider serves prompts/* (BE-MCP-SOL-011). Argument problems are
// returned as *jsonrpc.Error with code -32602.
type PromptProvider interface {
	ListPrompts(ctx context.Context, p Principal, locale string) ([]*mcp.Prompt, error)
	GetPrompt(ctx context.Context, p Principal, sessionID, name string, args map[string]string, locale string) (*mcp.GetPromptResult, error)
}
