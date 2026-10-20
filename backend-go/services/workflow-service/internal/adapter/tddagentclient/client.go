// Package tddagentclient drives tdd-agent runs on a project's dev server from
// workflow steps (action "tdd_agent.*", docs/integrate/TDD-WF-AGENT-001).
// Every call goes workflow-service -> infra-fleet Relay -> Dev Server Agent
// "mcp.callTool" -> tdd-agent MCP server (stdio) on that dev server. Long work
// never holds a call open: the run is advanced by a background process there
// (run_dispatch) and these handlers poll run_get.
package tddagentclient

import (
	"context"
	"fmt"
	"path"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tddagent"
	"github.com/stablyai/orca-go/common/tenant"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// MCPServer is the name of the tdd-agent server in the agent's mcp.callTool allowlist.
const MCPServer = tddagent.Server

// The wire side of the call (the mcp.callTool message, how a reply is classified) is shared with request-service in
// backend-go/common/tddagent; these names keep this package's own API and tests as they were.
type (
	// ToolError is a refusal or failure reported by the tool itself (tdd-agent's {error:{code,message}}).
	ToolError = tddagent.ToolError
	// BridgeError is a refusal of the dev server's MCP bridge (MCP_* codes): not retried.
	BridgeError = tddagent.BridgeError
)

// ErrTransport marks a failure to reach the dev server at all (relay or agent down): worth retrying.
var ErrTransport = tddagent.ErrTransport

// Relayer is the slice of infra-fleet's client this package uses.
type Relayer interface {
	Relay(ctx context.Context, in *infrafleetv1.RelayRequest, opts ...grpc.CallOption) (*infrafleetv1.RelayResponse, error)
}

// RepoLister is the slice of project-service's client this package uses.
type RepoLister interface {
	ListRepos(ctx context.Context, in *projectv1.ListReposRequest, opts ...grpc.CallOption) (*projectv1.ListReposResponse, error)
}

// Client calls tdd-agent tools on the dev server of a project.
type Client struct {
	relay    Relayer
	resolver usecase.ServerResolver
	repos    RepoLister
}

func NewClient(relay Relayer, resolver usecase.ServerResolver, repos RepoLister) *Client {
	return &Client{relay: relay, resolver: resolver, repos: repos}
}

// CallTool runs tool with args and decodes its JSON result into out (when non-nil).
// Errors: *ToolError, *BridgeError, or one wrapping ErrTransport.
func (c *Client) CallTool(ctx context.Context, target, projectDir, tool string, args map[string]any, out any) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return fmt.Errorf("tddagentclient: %w", err)
	}
	connectionID, err := c.resolver.Resolve(ctx, tenantID, target)
	if err != nil {
		return fmt.Errorf("%w: resolve %s: %v", ErrTransport, target, err)
	}
	relay := func(rctx context.Context, method, paramsJSON string) (string, error) {
		outCtx := metadata.AppendToOutgoingContext(rctx, grpcmw.MetadataTenantID, tenantID)
		resp, err := c.relay.Relay(outCtx, &infrafleetv1.RelayRequest{ConnectionId: connectionID, Method: method, ParamsJson: paramsJSON})
		if err != nil {
			return "", err
		}
		return resp.GetResultJson(), nil
	}
	return tddagent.Call(ctx, relay, projectDir, tool, args, out)
}

// ResolveProjectDir returns the tdd-agent project directory for a step: the
// requested one when it lies inside one of the project's repos, else (empty
// request) the project's first repo. A repo's URL is its path on the dev server.
func (c *Client) ResolveProjectDir(ctx context.Context, projectID, requested string) (string, error) {
	if projectID == "" {
		return "", &BridgeError{Code: "TDD_AGENT_NO_PROJECT", Message: "the workflow execution has no project"}
	}
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return "", fmt.Errorf("tddagentclient: %w", err)
	}
	userID, _ := tenant.UserID(ctx)
	outCtx := metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID, grpcmw.MetadataUserID, userID)
	resp, err := c.repos.ListRepos(outCtx, &projectv1.ListReposRequest{ProjectId: projectID})
	if err != nil {
		return "", fmt.Errorf("%w: list repos of project %s: %v", ErrTransport, projectID, err)
	}
	var roots []string
	for _, r := range resp.GetRepos() {
		if strings.HasPrefix(r.GetUrl(), "/") {
			roots = append(roots, path.Clean(r.GetUrl()))
		}
	}
	if len(roots) == 0 {
		return "", &BridgeError{Code: "TDD_AGENT_NO_REPO", Message: "the project has no repo on a dev server"}
	}
	if requested == "" {
		return roots[0], nil
	}
	clean := path.Clean(requested)
	for _, root := range roots {
		if clean == root || strings.HasPrefix(clean, root+"/") {
			return clean, nil
		}
	}
	return "", &BridgeError{Code: "TDD_AGENT_PROJECT_DIR_OUTSIDE_PROJECT", Message: fmt.Sprintf("%s is not inside a repo of the project", requested)}
}
