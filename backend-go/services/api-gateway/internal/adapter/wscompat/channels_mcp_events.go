package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"

	"google.golang.org/protobuf/types/known/emptypb"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

var (
	errMcpDisabled = errors.New("MCP_DISABLED: MCP is disabled on this server")
	errMcpNotFound = errors.New("MCP_NOT_FOUND: not found")
)

// mcpEventPushKey is the push channel name FE listens on (CONTRACT C5).
const mcpEventPushKey = "mcp.event"

// mcpEventBuffer: if the browser can't keep up the stream is dropped (events
// are hints; the client resyncs from mcp.approval.list and mcp.server.info).
const mcpEventBuffer = 64

func registerMcpEventsChannel(r *Registry, d McpChannelDeps) {
	r.RegisterStream("mcp.events.subscribe", func(ctx context.Context, id Identity, _ []json.RawMessage) (<-chan PushEvent, error) {
		if !d.Enabled {
			return nil, mcpChannelError(errMcpDisabled)
		}
		c, err := mcpClient(d)
		if err != nil {
			return nil, err
		}
		if id.UserID == "" {
			return nil, mcpChannelError(errMcpNotFound)
		}
		// Streams bypass Registry.Dispatch, so identity is attached here.
		sctx := gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID, Role: id.Role})
		stream, err := c.StreamEvents(sctx, &emptypb.Empty{})
		if err != nil {
			return nil, mcpChannelError(err)
		}
		out := make(chan PushEvent, mcpEventBuffer)
		go func() {
			defer close(out)
			for {
				ev, err := stream.Recv()
				if err != nil {
					if err != io.EOF && ctx.Err() == nil {
						slog.WarnContext(ctx, "mcp event stream ended", slog.Any("error", err))
					}
					return
				}
				payload, ok := toMcpEvent(ev)
				if !ok {
					continue
				}
				select {
				case out <- PushEvent{Channel: mcpEventPushKey, Args: []any{payload}}:
				case <-ctx.Done():
					return
				default: // slow consumer: end the stream so the client reconnects and resyncs
					return
				}
			}
		}()
		return out, nil
	})
}

// toMcpEvent maps the wire event to the CONTRACT McpEvent union; unknown
// types are dropped (additive evolution).
func toMcpEvent(e *mcpv1.McpEvent) (map[string]any, bool) {
	switch e.GetType() {
	case "approval.requested":
		if e.GetApproval() == nil {
			return nil, false
		}
		return map[string]any{"type": "approval.requested", "approval": toMcpApproval(e.GetApproval())}, true
	case "approval.resolved":
		return map[string]any{"type": "approval.resolved", "id": e.GetId(), "status": e.GetStatus()}, true
	case "grant.revoked":
		return map[string]any{"type": "grant.revoked", "grantId": e.GetGrantId()}, true
	case "session.closed":
		return map[string]any{"type": "session.closed", "sessionId": e.GetSessionId()}, true
	case "killswitch.changed":
		m := map[string]any{"type": "killswitch.changed", "active": e.GetActive()}
		if e.GetReason() != "" {
			m["reason"] = e.GetReason()
		}
		return m, true
	}
	return nil, false
}
