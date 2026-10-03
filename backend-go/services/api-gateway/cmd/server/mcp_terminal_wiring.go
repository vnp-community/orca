package main

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpmetrics"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/tools"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// infraAgentLister finds the agents an MCP session started via infra-fleet's
// ListAgentSessions. An infra-fleet that predates the RPC answers
// Unimplemented: the reaper then only stops this replica's own agents.
type infraAgentLister struct {
	c infrafleetv1.InfraFleetServiceClient
}

func (l infraAgentLister) ListMcpAgentSessions(ctx context.Context, id wscompat.Identity, mcpSessionID string) ([]tools.AgentSessionRef, error) {
	ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
	resp, err := l.c.ListAgentSessions(ctx, &infrafleetv1.ListAgentSessionsRequest{
		OriginType: "mcp", OriginSessionId: mcpSessionID, ActiveOnly: true, Limit: 100,
	})
	if status.Code(err) == codes.Unimplemented {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]tools.AgentSessionRef, 0, len(resp.GetSessions()))
	for _, s := range resp.GetSessions() {
		out = append(out, tools.AgentSessionRef{SessionID: s.GetId(), Status: s.GetStatus()})
	}
	return out, nil
}

// wireTerminalOperations connects the terminal/agent tools to metrics (ring
// overflow) and the durable agent reaper. Call after setWorktreeTargets and
// before serving: it rebuilds the session registry from s.PtyCfg.
//
// The idle-stop notification is NOT published here: the janitor closes the PTY
// with reason "idle" and infra-fleet writes orca.infrafleet.terminal.closed to
// its outbox in the same transaction as the close, so the gateway (which has no
// database) cannot lose it. Crash safety: dying before the close RPC leaves the
// terminal open for the next stop path (session reaper, reason session_closed);
// dying after the RPC succeeded leaves the event already in the outbox.
func (s *mcpToolStack) wireTerminalOperations(m *mcpmetrics.Metrics, fleet infrafleetv1.InfraFleetServiceClient, logger *slog.Logger) {
	if m != nil {
		s.PtyCfg.OnOutputDropped = m.TerminalDropped
	}
	if fleet != nil {
		s.PtyCfg.AgentLister = infraAgentLister{c: fleet}
	}
	s.PtyCfg.OnIdleStopped = idleCloseObserver(m, logger)
	s.Executor.WithPtyTools(s.PtyCfg)
}

// idleCloseObserver counts the janitor's close RPC by result (m may be nil).
func idleCloseObserver(m *mcpmetrics.Metrics, logger *slog.Logger) func(tenantID, userID, mcpSessionID, ptyID, clientName string, closeErr error) {
	return func(tenantID, _, mcpSessionID, ptyID, _ string, closeErr error) {
		result := "closed"
		if closeErr != nil {
			result = "failed"
			logger.Warn("mcp idle terminal close failed; the session reaper will retry", slog.String("tenant", tenantID),
				slog.String("session", mcpSessionID), slog.String("pty", ptyID), slog.Any("error", closeErr))
		}
		if m != nil {
			m.IdleStopClose(result)
		}
	}
}
