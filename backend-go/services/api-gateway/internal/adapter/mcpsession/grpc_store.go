// Package mcpsession adapts the gateway's session ports (mcpserver.SessionStore,
// StreamRegistry, ResumableEventStore) to mcp-service (gRPC) and NATS JetStream.
package mcpsession

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

// GRPCStore stores sessions in mcp-service. Identity travels as gRPC metadata
// built from the verified principal, never from request bodies.
type GRPCStore struct{ c mcpv1.McpServiceClient }

func NewGRPCStore(c mcpv1.McpServiceClient) *GRPCStore { return &GRPCStore{c: c} }

func idCtx(ctx context.Context, p mcpserver.Principal) context.Context {
	return gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: p.TenantID, UserID: p.UserID, Role: p.Role})
}

func mapErr(err error) error {
	switch status.Code(err) {
	case codes.NotFound, codes.PermissionDenied:
		return mcpserver.ErrSessionNotFound
	case codes.FailedPrecondition:
		if strings.Contains(status.Convert(err).Message(), "MCP_STREAM_LIMIT") {
			return mcpserver.ErrStreamLimit
		}
	}
	return err
}

func toRecord(s *mcpv1.McpSession) mcpserver.SessionRecord {
	return mcpserver.SessionRecord{
		ID: s.GetId(), TenantID: s.GetTenantId(), UserID: s.GetUserId(), ClientID: s.GetClientId(), ClientName: s.GetClientName(),
		ClientVersion: s.GetClientVersion(), GrantID: s.GetGrantId(), TokenID: s.GetTokenId(), ProtocolVersion: s.GetProtocolVersion(),
		Capabilities: s.GetCapabilitiesJson(), LogLevel: s.GetLogLevel(), State: s.GetState(),
	}
}

func (g *GRPCStore) Create(ctx context.Context, p mcpserver.Principal, hash []byte, ns mcpserver.NewSession) (mcpserver.SessionRecord, error) {
	s, err := g.c.CreateSession(idCtx(ctx, p), &mcpv1.CreateSessionRequest{
		SecretHash: hash, ClientId: p.ClientID, ClientName: ns.ClientName, ClientVersion: ns.ClientVersion, GrantId: p.GrantID,
		TokenId: p.TokenID, ProtocolVersion: ns.ProtocolVersion, CapabilitiesJson: ns.Capabilities,
	})
	if err != nil {
		return mcpserver.SessionRecord{}, mapErr(err)
	}
	return toRecord(s), nil
}

func (g *GRPCStore) Lookup(ctx context.Context, p mcpserver.Principal, hash []byte) (mcpserver.SessionRecord, error) {
	s, err := g.c.GetSessionBySecret(idCtx(ctx, p), &mcpv1.GetSessionBySecretRequest{SecretHash: hash})
	if err != nil {
		return mcpserver.SessionRecord{}, mapErr(err)
	}
	return toRecord(s), nil
}

func (g *GRPCStore) Touch(ctx context.Context, p mcpserver.Principal, id string, t mcpserver.SessionTouch) (string, error) {
	r, err := g.c.TouchSession(idCtx(ctx, p), &mcpv1.TouchSessionRequest{SessionId: id, Ready: t.Ready, ToolCallsDelta: t.ToolCallsDelta, LogLevel: t.LogLevel})
	if err != nil {
		return "", mapErr(err)
	}
	return r.GetState(), nil
}

func (g *GRPCStore) Close(ctx context.Context, p mcpserver.Principal, id, reason string) error {
	_, err := g.c.CloseSession(idCtx(ctx, p), &mcpv1.CloseSessionRequest{SessionId: id, Reason: reason})
	return mapErr(err)
}

func (g *GRPCStore) OpenStream(ctx context.Context, p mcpserver.Principal, id string, maxUser, maxTenant int) (string, error) {
	r, err := g.c.OpenStream(idCtx(ctx, p), &mcpv1.OpenStreamRequest{SessionId: id, Kind: "get", ReplicaId: "gateway", MaxPerUser: int32(maxUser), MaxPerTenant: int32(maxTenant)})
	if err != nil {
		return "", mapErr(err)
	}
	return r.GetStreamId(), nil
}

func (g *GRPCStore) HeartbeatStream(ctx context.Context, p mcpserver.Principal, streamID string) error {
	_, err := g.c.HeartbeatStream(idCtx(ctx, p), &mcpv1.HeartbeatStreamRequest{StreamId: streamID})
	return mapErr(err)
}

func (g *GRPCStore) CloseStream(ctx context.Context, p mcpserver.Principal, streamID string) error {
	_, err := g.c.CloseStream(idCtx(ctx, p), &mcpv1.CloseStreamRequest{StreamId: streamID})
	return mapErr(err)
}

var (
	_ mcpserver.SessionStore   = (*GRPCStore)(nil)
	_ mcpserver.StreamRegistry = (*GRPCStore)(nil)
)
