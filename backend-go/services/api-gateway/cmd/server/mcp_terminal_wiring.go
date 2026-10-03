package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpmetrics"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/tools"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// subjectTerminalIdleStopped is published when the janitor stops a PTY for
// inactivity (notification-service may turn it into a notification).
const subjectTerminalIdleStopped = "orca.mcp.terminal.idlestopped"

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

type terminalEventPublisher interface {
	Publish(ctx context.Context, subject string, event commoneventbus.Event) error
}

// idleStoppedNotifier publishes best effort: the PTY is already stopped, so a
// lost event only costs the user a notification.
func idleStoppedNotifier(pub terminalEventPublisher, logger *slog.Logger) func(tenantID, userID, mcpSessionID, ptyID string) {
	return func(tenantID, userID, mcpSessionID, ptyID string) {
		payload, _ := json.Marshal(map[string]string{"session_id": mcpSessionID, "user_id": userID, "pty_id": ptyID, "reason": "idle"})
		ev := commoneventbus.Event{ID: "idlestopped:" + ptyID, TenantID: tenantID, OccurredAt: time.Now().UTC(), Version: 1, Payload: payload}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := pub.Publish(ctx, subjectTerminalIdleStopped, ev); err != nil {
			logger.Warn("publishing mcp terminal idle-stop event failed", slog.Any("error", err))
		}
	}
}

// wireTerminalOperations connects the terminal/agent tools to metrics (ring
// overflow), the durable agent reaper and the idle-stop event. pub may be nil
// (NATS down). Call after setWorktreeTargets and before serving: it rebuilds the
// session registry from s.PtyCfg.
func (s *mcpToolStack) wireTerminalOperations(m *mcpmetrics.Metrics, fleet infrafleetv1.InfraFleetServiceClient, pub terminalEventPublisher, logger *slog.Logger) {
	if m != nil {
		s.PtyCfg.OnOutputDropped = m.TerminalDropped
	}
	if fleet != nil {
		s.PtyCfg.AgentLister = infraAgentLister{c: fleet}
	}
	if pub != nil {
		s.PtyCfg.OnIdleStopped = idleStoppedNotifier(pub, logger)
	}
	s.Executor.WithPtyTools(s.PtyCfg)
}
