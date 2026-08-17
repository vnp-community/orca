// Package grpc implements the generated infrafleetv1.InfraFleetServiceServer
// interface by translating wire messages to/from usecase calls — no
// business logic here, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md's
// inbound-adapter contract.
package grpc

import (
	"context"
	"encoding/json"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// Server implements infrafleetv1.UnimplementedInfraFleetServiceServer.
type Server struct {
	infrafleetv1.UnimplementedInfraFleetServiceServer

	registerDevServer  *usecase.RegisterDevServer
	resolveConnection  *usecase.ResolveConnection
	createSshTarget    *usecase.CreateSshTarget
	getFleetHealth     *usecase.GetFleetHealth
	scanWorkspacePorts *usecase.ScanWorkspacePorts
	listDevServers     *usecase.ListDevServers
	createConnection   *usecase.CreateConnection
	relay              *usecase.Relay
}

func New(
	registerDevServer *usecase.RegisterDevServer,
	resolveConnection *usecase.ResolveConnection,
	createSshTarget *usecase.CreateSshTarget,
	getFleetHealth *usecase.GetFleetHealth,
	scanWorkspacePorts *usecase.ScanWorkspacePorts,
	listDevServers *usecase.ListDevServers,
	createConnection *usecase.CreateConnection,
	relay *usecase.Relay,
) *Server {
	return &Server{
		registerDevServer:  registerDevServer,
		resolveConnection:  resolveConnection,
		createSshTarget:    createSshTarget,
		getFleetHealth:     getFleetHealth,
		scanWorkspacePorts: scanWorkspacePorts,
		listDevServers:     listDevServers,
		createConnection:   createConnection,
		relay:              relay,
	}
}

func (s *Server) RegisterDevServer(ctx context.Context, req *infrafleetv1.RegisterDevServerRequest) (*infrafleetv1.RegisterDevServerResponse, error) {
	devServer, err := s.registerDevServer.Execute(ctx, usecase.RegisterDevServerInput{
		Host:        req.GetHost(),
		Mode:        toDomainConnectionMode(req.GetMode()),
		SSHTargetID: req.GetSshTargetId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.RegisterDevServerResponse{DevServer: toProtoDevServer(devServer)}, nil
}

// ResolveConnection is THE core dispatch primitive every dependent service
// calls — see usecase.ResolveConnection's doc comment.
func (s *Server) ResolveConnection(ctx context.Context, req *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error) {
	out, err := s.resolveConnection.Execute(ctx, req.GetConnectionId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	resp := &infrafleetv1.ResolveConnectionResponse{Connected: out.Connected}
	if out.Connected {
		resp.DevServer = toProtoDevServer(out.DevServer)
		resp.RepoPath = out.RepoPath
		resp.WorktreeId = out.WorktreeID
	}
	return resp, nil
}

// ListDevServers backs the frontend's devServer.list channel (wired through
// api-gateway's wscompat) — see usecase.ListDevServers's doc comment.
func (s *Server) ListDevServers(ctx context.Context, req *infrafleetv1.ListDevServersRequest) (*infrafleetv1.ListDevServersResponse, error) {
	devServers, err := s.listDevServers.Execute(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*infrafleetv1.DevServer, 0, len(devServers))
	for _, ds := range devServers {
		out = append(out, toProtoDevServer(ds))
	}
	return &infrafleetv1.ListDevServersResponse{DevServers: out}, nil
}

// CreateConnection is the write path for infra.connections — see
// usecase.CreateConnection's doc comment.
func (s *Server) CreateConnection(ctx context.Context, req *infrafleetv1.CreateConnectionRequest) (*infrafleetv1.CreateConnectionResponse, error) {
	conn, err := s.createConnection.Execute(ctx, usecase.CreateConnectionInput{
		DevServerID: req.GetDevServerId(),
		RepoPath:    req.GetRepoPath(),
		WorktreeID:  req.GetWorktreeId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.CreateConnectionResponse{ConnectionId: conn.ID}, nil
}

// Relay is the generic connectionId+method+params passthrough — see
// usecase.Relay's doc comment for why this is one RPC rather than one per
// caller/method.
func (s *Server) Relay(ctx context.Context, req *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) {
	var params map[string]any
	if raw := req.GetParamsJson(); raw != "" {
		if err := json.Unmarshal([]byte(raw), &params); err != nil {
			return nil, apperrors.ToGRPCStatus(apperrors.New(apperrors.KindInvalidArgument, "INFRA_RELAY_BAD_PARAMS", "params_json must be a JSON object", err))
		}
	}

	result, err := s.relay.Execute(ctx, usecase.RelayInput{
		ConnectionID: req.GetConnectionId(),
		Method:       req.GetMethod(),
		Params:       params,
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(apperrors.New(apperrors.KindInternal, "INFRA_RELAY_ENCODE_FAILED", "failed to encode relay result", err))
	}
	return &infrafleetv1.RelayResponse{ResultJson: string(resultJSON)}, nil
}

func (s *Server) CreateSshTarget(ctx context.Context, req *infrafleetv1.CreateSshTargetRequest) (*infrafleetv1.CreateSshTargetResponse, error) {
	target, err := s.createSshTarget.Execute(ctx, usecase.CreateSshTargetInput{
		Host:         req.GetHost(),
		UserName:     req.GetUser(),
		VaultSSHRole: req.GetVaultSshRole(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.CreateSshTargetResponse{SshTargetId: target.ID}, nil
}

func (s *Server) GetFleetHealth(ctx context.Context, req *infrafleetv1.GetFleetHealthRequest) (*infrafleetv1.GetFleetHealthResponse, error) {
	statuses, err := s.getFleetHealth.Execute(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*infrafleetv1.DevServerHealth, 0, len(statuses))
	for _, h := range statuses {
		out = append(out, toProtoDevServerHealth(h))
	}
	return &infrafleetv1.GetFleetHealthResponse{Statuses: out}, nil
}

func (s *Server) ScanWorkspacePorts(ctx context.Context, req *infrafleetv1.ScanWorkspacePortsRequest) (*infrafleetv1.ScanWorkspacePortsResponse, error) {
	ports, err := s.scanWorkspacePorts.Execute(ctx, usecase.ScanWorkspacePortsInput{
		ConnectionID: req.GetConnectionId(),
		WorktreeID:   req.GetWorktreeId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.ScanWorkspacePortsResponse{OpenPorts: ports}, nil
}

func toDomainConnectionMode(m infrafleetv1.ConnectionMode) domain.ConnectionMode {
	switch m {
	case infrafleetv1.ConnectionMode_CONNECTION_MODE_RELAY_SSH:
		return domain.ConnectionModeRelaySSH
	case infrafleetv1.ConnectionMode_CONNECTION_MODE_RELAY_WEBSOCKET:
		return domain.ConnectionModeRelayWebSocket
	case infrafleetv1.ConnectionMode_CONNECTION_MODE_DIRECT_WEBSOCKET:
		return domain.ConnectionModeDirectWebSocket
	default:
		return ""
	}
}

func toProtoConnectionMode(m domain.ConnectionMode) infrafleetv1.ConnectionMode {
	switch m {
	case domain.ConnectionModeRelaySSH:
		return infrafleetv1.ConnectionMode_CONNECTION_MODE_RELAY_SSH
	case domain.ConnectionModeRelayWebSocket:
		return infrafleetv1.ConnectionMode_CONNECTION_MODE_RELAY_WEBSOCKET
	case domain.ConnectionModeDirectWebSocket:
		return infrafleetv1.ConnectionMode_CONNECTION_MODE_DIRECT_WEBSOCKET
	default:
		return infrafleetv1.ConnectionMode_CONNECTION_MODE_UNSPECIFIED
	}
}

func toProtoDevServer(ds domain.DevServer) *infrafleetv1.DevServer {
	return &infrafleetv1.DevServer{
		Id:          ds.ID,
		TenantId:    ds.TenantID,
		Host:        ds.Host,
		Mode:        toProtoConnectionMode(ds.Mode),
		SshTargetId: ds.SSHTargetID,
	}
}

func toProtoDevServerHealth(h domain.DevServerHealth) *infrafleetv1.DevServerHealth {
	return &infrafleetv1.DevServerHealth{
		DevServerId: h.DevServerID,
		Reachable:   h.Reachable,
		CpuPercent:  h.CPUPercent,
		RamPercent:  h.RAMPercent,
		DiskPercent: h.DiskPercent,
		LatencyMs:   h.LatencyMS,
	}
}
