package tools

import (
	"context"
	"encoding/json"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

// ComposeFunc implements a KindComposite tool. in is the validated tool input.
// The result is normalised (redacted, truncated) like a channel result.
type ComposeFunc func(ctx context.Context, env *CompositeEnv, in json.RawMessage) (any, error)

// CompositeEnv is what a composite tool may touch: the caller identity, the
// MCP-session-scoped PTY state and the channel registry (never gRPC directly,
// so the same handlers, SSH/dev-server routing and identity rules as the UI apply).
type CompositeEnv struct {
	e          *Executor
	Principal  mcpserver.Principal
	Identity   wscompat.Identity
	SessionID  string // MCP session row id, "" when the transport gave none
	ClientName string
}

func (e *Executor) compositeEnv(ctx context.Context, p mcpserver.Principal, id wscompat.Identity) *CompositeEnv {
	env := &CompositeEnv{e: e, Principal: p, Identity: id}
	if e.guards.SessionID != nil {
		env.SessionID = e.guards.SessionID(ctx)
	}
	if e.guards.ClientName != nil {
		env.ClientName = e.guards.ClientName(ctx)
	}
	return env
}

// Session returns the ToolSession owning this MCP session's PTYs.
func (env *CompositeEnv) Session(ctx context.Context) (*ToolSession, error) {
	return env.e.sessions.get(ctx, env.Principal, env.SessionID, env.ClientName)
}

// scoped decorates ctx so terminal.* / agent.* handlers find the session's
// stream registry and stamp the MCP origin on what they create.
func (env *CompositeEnv) scoped(ctx context.Context, ts *ToolSession) context.Context {
	ctx = wscompat.WithToolOrigin(ctx, wscompat.ToolOrigin{ClientName: ts.pty.clientName, MCPSessionID: ts.pty.id, UserID: ts.pty.userID})
	return ts.pty.streams.Context(ctx)
}

// Dispatch calls a unary channel with one object argument.
func (env *CompositeEnv) Dispatch(ctx context.Context, ts *ToolSession, channel string, payload any) (any, error) {
	return env.e.disp.Dispatch(env.scoped(ctx, ts), env.Identity, channel, mustArgs(payload))
}

// DispatchPlain calls a unary channel that needs no session scope.
func (env *CompositeEnv) DispatchPlain(ctx context.Context, channel string, payload any) (any, error) {
	return env.e.disp.Dispatch(ctx, env.Identity, channel, mustArgs(payload))
}

// DispatchStream calls a streamChannel; the caller must consume events.
func (env *CompositeEnv) DispatchStream(ctx context.Context, ts *ToolSession, channel string, payload any) (any, <-chan wscompat.PushEvent, error) {
	ack, events, ok, err := env.e.disp.DispatchStreamChannel(env.scoped(ctx, ts), env.Identity, channel, mustArgs(payload))
	if err == nil && !ok {
		return nil, nil, &ToolError{"MCP_INTERNAL", channel + " is not a stream channel"}
	}
	return ack, events, err
}

func decodeInput[T any](in json.RawMessage) (T, error) {
	var v T
	if len(in) > 0 && string(in) != "null" {
		if err := json.Unmarshal(in, &v); err != nil {
			return v, &ToolError{"INVALID_ARGUMENTS", "arguments do not match the tool schema"}
		}
	}
	return v, nil
}

// asMap converts any JSON-able value (channel views, protos aside) to a map.
func asMap(v any) map[string]any {
	b, err := json.Marshal(v)
	if err != nil {
		return map[string]any{}
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil || m == nil {
		return map[string]any{}
	}
	return m
}
