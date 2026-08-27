package grpcclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// InfraFleetTerminalStatusResolver implements usecase.TerminalStatusResolver
// by resolving a dev server's active connectionId (ResolveConnection, the
// same RPC channels_browser.go already uses) then listing its terminal
// sessions — a thin client, mirrors InfraFleetDevServerLister's shape.
type InfraFleetTerminalStatusResolver struct {
	conn   *grpc.ClientConn
	client infrafleetv1.InfraFleetServiceClient
}

func NewInfraFleetTerminalStatusResolver(addr string) (*InfraFleetTerminalStatusResolver, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpcclient: dial infra-fleet-service at %q: %w", addr, err)
	}
	return &InfraFleetTerminalStatusResolver{conn: conn, client: infrafleetv1.NewInfraFleetServiceClient(conn)}, nil
}

func (c *InfraFleetTerminalStatusResolver) Close() error { return c.conn.Close() }

// ListSessionsForDevServer resolves devServerID to its active connectionId
// (ResolveConnectionResponse carries the resolved id directly via
// GetConnectionId — NOT DevServer.GetId(), a different id space per
// channels_browser.go's TASK-025 comment) then lists that connection's
// terminal sessions. A dev server with no live connection returns (nil,
// nil) — not an error, just no sessions to correlate against.
func (c *InfraFleetTerminalStatusResolver) ListSessionsForDevServer(ctx context.Context, devServerID string) ([]*infrafleetv1.TerminalSession, error) {
	resolved, err := c.client.ResolveConnection(ctx, &infrafleetv1.ResolveConnectionRequest{DevServerId: devServerID})
	if err != nil {
		return nil, fmt.Errorf("grpcclient: resolve connection for dev server %q: %w", devServerID, err)
	}
	if !resolved.GetConnected() {
		return nil, nil // not currently connected — no error, just no sessions
	}
	resp, err := c.client.ListTerminalSessions(ctx, &infrafleetv1.ListTerminalSessionsRequest{ConnectionId: resolved.GetConnectionId()})
	if err != nil {
		return nil, fmt.Errorf("grpcclient: list terminal sessions: %w", err)
	}
	return resp.GetSessions(), nil
}

// GetAgentStatus fetches one ptyId's agent-status fields (AgentKind/
// AgentRunning/ReadyForInput) — TerminalSession itself doesn't carry these
// (they live on GetTerminalAgentStatusResponse, per-ptyId), so
// GetMobileWorktreeStatus issues one of these per matched session. See that
// usecase's doc comment for the extra-RPC-cost tradeoff.
func (c *InfraFleetTerminalStatusResolver) GetAgentStatus(ctx context.Context, ptyID string) (*infrafleetv1.GetTerminalAgentStatusResponse, error) {
	resp, err := c.client.GetTerminalAgentStatus(ctx, &infrafleetv1.GetTerminalAgentStatusRequest{PtyId: ptyID})
	if err != nil {
		return nil, fmt.Errorf("grpcclient: get terminal agent status for pty %q: %w", ptyID, err)
	}
	return resp, nil
}
