// Package tddagent is the service-independent half of driving tdd-agent on a project's dev server.
//
// Every call goes (an Orca service) -> infra-fleet relay -> Dev Server Agent "mcp.callTool" -> tdd-agent MCP server
// (stdio) on that dev server. This package owns what both callers share: the wire shape of mcp.callTool, how a
// reply is classified (tool refusal, bridge refusal, dev server unreachable) and how a run_get reply is read.
// It knows no proto, no service usecase and no repository: the caller supplies a Relay that sends one
// message to the right dev server, and decides how to find that server, how long to wait and when to retry.
//
// Used by workflow-service (blocking steps that poll a run) and request-service (event-driven test specs).
// The contract is docs/integrate/contracts/mcp-call-tool.{request,response}.schema.json.
package tddagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// Server is the name of the tdd-agent server in the agent's mcp.callTool allowlist.
const Server = "tdd-agent"

// CallToolMethod is the Dev Server Agent RPC that reaches an MCP server.
const CallToolMethod = "mcp.callTool"

// ToolError is a refusal or failure reported by the tool itself (tdd-agent's {error:{code,message}}).
type ToolError struct {
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Details json.RawMessage `json:"details,omitempty"`
}

func (e *ToolError) Error() string { return e.Code + ": " + e.Message }

// BridgeError is a refusal of the dev server's MCP bridge (MCP_* codes): not retried.
type BridgeError struct {
	Code    string
	Message string
}

func (e *BridgeError) Error() string { return e.Code + ": " + e.Message }

// ErrTransport marks a failure to reach the dev server at all (relay or agent down): worth retrying.
var ErrTransport = errors.New("tdd-agent: dev server unreachable")

// Relay sends one message to the Dev Server Agent that serves the project and returns its result JSON.
// An error from Relay means the dev server was not reached (it becomes ErrTransport).
type Relay func(ctx context.Context, method, paramsJSON string) (resultJSON string, err error)

// params is the params object of mcp.callTool.
type params struct {
	Server     string         `json:"server"`
	ProjectDir string         `json:"projectDir"`
	Tool       string         `json:"tool"`
	Arguments  map[string]any `json:"arguments"`
	TimeoutMs  int            `json:"timeoutMs,omitempty"`
}

// result is the result object of mcp.callTool.
type result struct {
	OK      bool            `json:"ok"`
	IsError bool            `json:"isError"`
	Result  json.RawMessage `json:"result"`
	Error   *ToolError      `json:"error"`
}

// Option changes one call.
type Option func(*params)

// WithTimeoutMs bounds the tool call on the agent's side (the agent applies its own default and ceiling when it is 0).
func WithTimeoutMs(ms int) Option { return func(p *params) { p.TimeoutMs = ms } }

// WithServer calls another MCP server of the agent's allowlist (for example test-script-agent); the default is tdd-agent.
// The wire shape and the error classes are the same for every server behind the bridge.
func WithServer(name string) Option { return func(p *params) { p.Server = name } }

// Call runs tool with args in projectDir and decodes its JSON result into out (when non-nil).
// Errors: *ToolError, *BridgeError, or one wrapping ErrTransport.
func Call(ctx context.Context, relay Relay, projectDir, tool string, args map[string]any, out any, opts ...Option) error {
	if args == nil {
		args = map[string]any{}
	}
	p := params{Server: Server, ProjectDir: projectDir, Tool: tool, Arguments: args}
	for _, o := range opts {
		o(&p)
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("tddagent: marshal %s: %w", tool, err)
	}
	resultJSON, err := relay(ctx, CallToolMethod, string(raw))
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrTransport, tool, err)
	}
	var res result
	if err := json.Unmarshal([]byte(resultJSON), &res); err != nil {
		return fmt.Errorf("tddagent: %s: agent without mcp.callTool or bad result: %w", tool, err)
	}
	if !res.OK {
		if res.Error == nil {
			return &BridgeError{Code: "MCP_BRIDGE_FAILED", Message: "bridge refused the call without a reason"}
		}
		// Server crashes and timeouts are transient on the dev server's side; refusals are not.
		if res.Error.Code == "MCP_SERVER_FAILED" || res.Error.Code == "MCP_TIMEOUT" {
			return fmt.Errorf("%w: %s: %s", ErrTransport, res.Error.Code, res.Error.Message)
		}
		return &BridgeError{Code: res.Error.Code, Message: res.Error.Message}
	}
	if res.IsError {
		if res.Error == nil {
			return &ToolError{Code: "TDD_AGENT_TOOL_ERROR", Message: "tool failed without a reason"}
		}
		return res.Error
	}
	if out != nil && len(res.Result) > 0 {
		if err := json.Unmarshal(res.Result, out); err != nil {
			return fmt.Errorf("tddagent: decode %s result: %w", tool, err)
		}
	}
	return nil
}
