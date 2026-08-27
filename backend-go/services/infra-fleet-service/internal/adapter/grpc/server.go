// Package grpc implements the generated infrafleetv1.InfraFleetServiceServer
// interface by translating wire messages to/from usecase calls — no
// business logic here, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md's
// inbound-adapter contract.
package grpc

import (
	"context"
	"encoding/json"

	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
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

	listSshTargets      *usecase.ListSshTargets
	getSshState         *usecase.GetSshState
	establishConnection *usecase.EstablishConnection
	killWorkspacePort   *usecase.KillWorkspacePort
	// --- Terminal/PTY (TASK-185) ---
	spawnTerminalSession   *usecase.SpawnTerminalSession
	resizeTerminalSession  *usecase.ResizeTerminalSession
	killTerminalSession    *usecase.KillTerminalSession
	stopTerminalProcess    *usecase.StopTerminalProcess
	listTerminalSessions   *usecase.ListTerminalSessions
	waitTerminalSession    *usecase.WaitTerminalSession
	focusTerminalSession   *usecase.FocusTerminalSession
	getTerminalAgentStatus *usecase.GetTerminalAgentStatus
	inspectTerminalProcess *usecase.InspectTerminalProcess
	attachPty              *usecase.AttachPty
	listBrowserProfiles    *usecase.ListBrowserProfiles
	createBrowserProfile   *usecase.CreateBrowserProfile
	deleteBrowserProfile   *usecase.DeleteBrowserProfile

	// --- Emulator relay (TASK-048) / host capabilities relay (TASK-070) ---
	// Shipped-but-honestly-inert until agent/ gains device.*/host.capabilities
	// — see usecase.EmulatorRelay / usecase.GetHostCapabilities doc comments.
	emulatorRelay       *usecase.EmulatorRelay
	getHostCapabilities *usecase.GetHostCapabilities

	// --- Persistent agent tokens (BL-AWS-03, TASK-AWS-03-07) ---
	createAgentToken *usecase.CreateAgentToken
	listAgentTokens  *usecase.ListAgentTokens
	revokeAgentToken *usecase.RevokeAgentToken
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
	listSshTargets *usecase.ListSshTargets,
	getSshState *usecase.GetSshState,
	establishConnection *usecase.EstablishConnection,
	killWorkspacePort *usecase.KillWorkspacePort,
	spawnTerminalSession *usecase.SpawnTerminalSession,
	resizeTerminalSession *usecase.ResizeTerminalSession,
	killTerminalSession *usecase.KillTerminalSession,
	stopTerminalProcess *usecase.StopTerminalProcess,
	listTerminalSessions *usecase.ListTerminalSessions,
	waitTerminalSession *usecase.WaitTerminalSession,
	focusTerminalSession *usecase.FocusTerminalSession,
	getTerminalAgentStatus *usecase.GetTerminalAgentStatus,
	inspectTerminalProcess *usecase.InspectTerminalProcess,
	attachPty *usecase.AttachPty,
	listBrowserProfiles *usecase.ListBrowserProfiles,
	createBrowserProfile *usecase.CreateBrowserProfile,
	deleteBrowserProfile *usecase.DeleteBrowserProfile,
	emulatorRelay *usecase.EmulatorRelay,
	getHostCapabilities *usecase.GetHostCapabilities,
	createAgentToken *usecase.CreateAgentToken,
	listAgentTokens *usecase.ListAgentTokens,
	revokeAgentToken *usecase.RevokeAgentToken,
) *Server {
	return &Server{
		registerDevServer:      registerDevServer,
		resolveConnection:      resolveConnection,
		createSshTarget:        createSshTarget,
		getFleetHealth:         getFleetHealth,
		scanWorkspacePorts:     scanWorkspacePorts,
		listDevServers:         listDevServers,
		createConnection:       createConnection,
		relay:                  relay,
		listSshTargets:         listSshTargets,
		getSshState:            getSshState,
		establishConnection:    establishConnection,
		killWorkspacePort:      killWorkspacePort,
		spawnTerminalSession:   spawnTerminalSession,
		resizeTerminalSession:  resizeTerminalSession,
		killTerminalSession:    killTerminalSession,
		stopTerminalProcess:    stopTerminalProcess,
		listTerminalSessions:   listTerminalSessions,
		waitTerminalSession:    waitTerminalSession,
		focusTerminalSession:   focusTerminalSession,
		getTerminalAgentStatus: getTerminalAgentStatus,
		inspectTerminalProcess: inspectTerminalProcess,
		attachPty:              attachPty,
		listBrowserProfiles:    listBrowserProfiles,
		createBrowserProfile:   createBrowserProfile,
		deleteBrowserProfile:   deleteBrowserProfile,
		emulatorRelay:          emulatorRelay,
		getHostCapabilities:    getHostCapabilities,
		createAgentToken:       createAgentToken,
		listAgentTokens:        listAgentTokens,
		revokeAgentToken:       revokeAgentToken,
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
	out, err := s.resolveConnection.Execute(ctx, usecase.ResolveConnectionInput{
		ConnectionID: req.GetConnectionId(),
		DevServerID:  req.GetDevServerId(),
		WorktreeID:   req.GetWorktreeId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	resp := &infrafleetv1.ResolveConnectionResponse{Connected: out.Connected}
	if out.Connected {
		resp.DevServer = toProtoDevServer(out.DevServer)
		resp.RepoPath = out.RepoPath
		resp.WorktreeId = out.WorktreeID
		resp.ConnectionId = out.ConnectionID
		resp.NodeVersion = out.NodeVersion
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

func (s *Server) ListSshTargets(ctx context.Context, req *infrafleetv1.ListSshTargetsRequest) (*infrafleetv1.ListSshTargetsResponse, error) {
	targets, err := s.listSshTargets.Execute(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*infrafleetv1.SshTarget, 0, len(targets))
	for _, t := range targets {
		out = append(out, &infrafleetv1.SshTarget{
			Id: t.ID, TenantId: t.TenantID, Host: t.Host, User: t.UserName, VaultSshRole: t.VaultSSHRole,
		})
	}
	return &infrafleetv1.ListSshTargetsResponse{SshTargets: out}, nil
}

func (s *Server) GetSshState(ctx context.Context, req *infrafleetv1.GetSshStateRequest) (*infrafleetv1.GetSshStateResponse, error) {
	state, err := s.getSshState.Execute(ctx, usecase.SshStateInput{SshTargetID: req.GetSshTargetId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	resp := &infrafleetv1.GetSshStateResponse{Connected: state.Connected, ConnectionId: state.ConnectionID}
	if state.LastActivity != nil {
		resp.LastActivityUnixMs = state.LastActivity.UnixMilli()
	}
	return resp, nil
}

func (s *Server) EstablishConnection(ctx context.Context, req *infrafleetv1.EstablishConnectionRequest) (*infrafleetv1.Connection, error) {
	conn, err := s.establishConnection.Execute(ctx, usecase.EstablishConnectionInput{SshTargetID: req.GetSshTargetId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	resp := &infrafleetv1.Connection{Id: conn.ID, DevServerId: conn.DevServerID, Status: conn.Status}
	if conn.LastActivityAt != nil {
		resp.EstablishedAtUnixMs = conn.LastActivityAt.UnixMilli()
	}
	return resp, nil
}

func (s *Server) KillWorkspacePort(ctx context.Context, req *infrafleetv1.KillWorkspacePortRequest) (*infrafleetv1.KillWorkspacePortResponse, error) {
	ok, reason, err := s.killWorkspacePort.Execute(ctx, usecase.KillWorkspacePortInput{
		ConnectionID: req.GetConnectionId(),
		WorktreeID:   req.GetWorktreeId(),
		PID:          req.GetPid(),
		Port:         req.GetPort(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.KillWorkspacePortResponse{Ok: ok, Reason: reason}, nil
}

// CreateAgentToken/ListAgentTokens/RevokeAgentToken back BL-AWS-03's
// persistent, named, per-DevServer agent token admin surface — see
// specs/backend-go/bugs/logic-v1/solutions/SOL-AWS-03-agent-token-management.md.

func (s *Server) CreateAgentToken(ctx context.Context, req *infrafleetv1.CreateAgentTokenRequest) (*infrafleetv1.CreateAgentTokenResponse, error) {
	plaintext, tok, err := s.createAgentToken.Execute(ctx, req.GetDevServerId(), req.GetName())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.CreateAgentTokenResponse{
		Id: tok.ID, Token: plaintext, Name: tok.Name, CreatedAtUnixMs: tok.CreatedAt.UnixMilli(),
	}, nil
}

func (s *Server) ListAgentTokens(ctx context.Context, req *infrafleetv1.ListAgentTokensRequest) (*infrafleetv1.ListAgentTokensResponse, error) {
	summaries, err := s.listAgentTokens.Execute(ctx, req.GetDevServerId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*infrafleetv1.AgentTokenSummary, 0, len(summaries))
	for _, sum := range summaries {
		pb := &infrafleetv1.AgentTokenSummary{Id: sum.ID, Name: sum.Name, CreatedAtUnixMs: sum.CreatedAt.UnixMilli()}
		if sum.LastUsedAt != nil {
			ms := sum.LastUsedAt.UnixMilli()
			pb.LastUsedAtUnixMs = &ms
		}
		out = append(out, pb)
	}
	return &infrafleetv1.ListAgentTokensResponse{Tokens: out}, nil
}

func (s *Server) RevokeAgentToken(ctx context.Context, req *infrafleetv1.RevokeAgentTokenRequest) (*emptypb.Empty, error) {
	if err := s.revokeAgentToken.Execute(ctx, req.GetDevServerId(), req.GetId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

// ListBrowserProfiles backs the frontend's browser.profileList channel —
// see usecase.ListBrowserProfiles's doc comment (SOL-006 Group C).
func (s *Server) ListBrowserProfiles(ctx context.Context, req *infrafleetv1.ListBrowserProfilesRequest) (*infrafleetv1.ListBrowserProfilesResponse, error) {
	profiles, err := s.listBrowserProfiles.Execute(ctx, req.GetDevServerId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*infrafleetv1.BrowserProfile, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, toProtoBrowserProfile(p))
	}
	return &infrafleetv1.ListBrowserProfilesResponse{Profiles: out}, nil
}

// CreateBrowserProfile backs the frontend's browser.profileCreate channel —
// see usecase.CreateBrowserProfile's doc comment (SOL-006 Group C).
func (s *Server) CreateBrowserProfile(ctx context.Context, req *infrafleetv1.CreateBrowserProfileRequest) (*infrafleetv1.CreateBrowserProfileResponse, error) {
	profile, err := s.createBrowserProfile.Execute(ctx, usecase.CreateBrowserProfileInput{
		DevServerID:   req.GetDevServerId(),
		Name:          req.GetName(),
		SourceBrowser: req.GetSourceBrowser(),
		IsDefault:     req.GetIsDefault(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.CreateBrowserProfileResponse{Profile: toProtoBrowserProfile(profile)}, nil
}

// DeleteBrowserProfile backs the frontend's browser.profileDelete channel —
// see usecase.DeleteBrowserProfile's doc comment (SOL-006 Group C).
func (s *Server) DeleteBrowserProfile(ctx context.Context, req *infrafleetv1.DeleteBrowserProfileRequest) (*emptypb.Empty, error) {
	if err := s.deleteBrowserProfile.Execute(ctx, req.GetId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
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

// --- Terminal/PTY (TASK-185) ---

func (s *Server) SpawnTerminalSession(ctx context.Context, req *infrafleetv1.SpawnTerminalSessionRequest) (*infrafleetv1.SpawnTerminalSessionResponse, error) {
	session, err := s.spawnTerminalSession.Execute(ctx, usecase.SpawnTerminalSessionInput{
		ConnectionID: req.GetConnectionId(),
		Cwd:          req.GetCwd(),
		Shell:        req.GetShell(),
		Cols:         req.GetCols(),
		Rows:         req.GetRows(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.SpawnTerminalSessionResponse{Session: toProtoTerminalSession(session)}, nil
}

func (s *Server) ResizeTerminalSession(ctx context.Context, req *infrafleetv1.ResizeTerminalSessionRequest) (*emptypb.Empty, error) {
	if err := s.resizeTerminalSession.Execute(ctx, usecase.ResizeTerminalSessionInput{PtyID: req.GetPtyId(), Cols: req.GetCols(), Rows: req.GetRows()}); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) KillTerminalSession(ctx context.Context, req *infrafleetv1.KillTerminalSessionRequest) (*emptypb.Empty, error) {
	if err := s.killTerminalSession.Execute(ctx, req.GetPtyId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) StopTerminalProcess(ctx context.Context, req *infrafleetv1.StopTerminalProcessRequest) (*emptypb.Empty, error) {
	if err := s.stopTerminalProcess.Execute(ctx, req.GetPtyId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) ListTerminalSessions(ctx context.Context, req *infrafleetv1.ListTerminalSessionsRequest) (*infrafleetv1.ListTerminalSessionsResponse, error) {
	sessions, err := s.listTerminalSessions.Execute(ctx, usecase.ListTerminalSessionsInput{ConnectionID: req.GetConnectionId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*infrafleetv1.TerminalSession, 0, len(sessions))
	for _, session := range sessions {
		out = append(out, toProtoTerminalSession(session))
	}
	return &infrafleetv1.ListTerminalSessionsResponse{Sessions: out}, nil
}

func (s *Server) WaitTerminalSession(ctx context.Context, req *infrafleetv1.WaitTerminalSessionRequest) (*infrafleetv1.WaitTerminalSessionResponse, error) {
	result, err := s.waitTerminalSession.Execute(ctx, usecase.WaitTerminalSessionInput{PtyID: req.GetPtyId(), TimeoutMs: req.GetTimeoutMs()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.WaitTerminalSessionResponse{Exited: result.Exited, ExitCode: result.ExitCode, TimedOut: result.TimedOut}, nil
}

func (s *Server) FocusTerminalSession(ctx context.Context, req *infrafleetv1.FocusTerminalSessionRequest) (*emptypb.Empty, error) {
	if err := s.focusTerminalSession.Execute(ctx, req.GetPtyId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) GetTerminalAgentStatus(ctx context.Context, req *infrafleetv1.GetTerminalAgentStatusRequest) (*infrafleetv1.GetTerminalAgentStatusResponse, error) {
	result, err := s.getTerminalAgentStatus.Execute(ctx, req.GetPtyId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.GetTerminalAgentStatusResponse{
		AgentRunning:  result.AgentRunning,
		AgentKind:     result.AgentKind,
		ReadyForInput: result.ReadyForInput,
	}, nil
}

func (s *Server) InspectTerminalProcess(ctx context.Context, req *infrafleetv1.InspectTerminalProcessRequest) (*infrafleetv1.InspectTerminalProcessResponse, error) {
	result, err := s.inspectTerminalProcess.Execute(ctx, req.GetPtyId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.InspectTerminalProcessResponse{
		Known:   result.Known,
		Pid:     result.Pid,
		Command: result.Command,
		Cwd:     result.Cwd,
	}, nil
}

// AttachPty implements the bidirectional streaming RPC: pumps
// stream.Recv() into an inbound channel usecase.AttachPty.Execute consumes,
// and pumps its two returned channels (outbound, errCh) back into
// stream.Send()/the final returned error.
//
// Tenant extraction: grpcmw.ChainUnary only wires a UnaryServerInterceptor
// chain (see that function's doc comment) — there is no stream-interceptor
// counterpart registered in cmd/server/main.go, so a streaming RPC's ctx
// does NOT get tenant.WithTenantID applied automatically the way every
// unary handler's does. This handler works around that gap locally (mirrors
// grpcmw.TenantExtractionInterceptor's own metadata-read exactly) rather
// than editing the shared common/grpcmw package, which would widen this
// pass's blast radius beyond this one streaming RPC. FLAGGED as a known gap:
// a real stream interceptor in common/grpcmw would be the more correct fix
// if more streaming RPCs are added later.
func (s *Server) AttachPty(stream infrafleetv1.InfraFleetService_AttachPtyServer) error {
	ctx := withTenantFromStreamMetadata(stream.Context())

	inbound := make(chan usecase.PtyClientMessage)
	go pumpAttachPtyInbound(stream, inbound)

	outbound, errCh := s.attachPty.Execute(ctx, inbound)
	for {
		select {
		case msg, ok := <-outbound:
			if !ok {
				outbound = nil
				continue
			}
			if err := stream.Send(toProtoPtyServerFrame(msg)); err != nil {
				return err
			}
		case err, ok := <-errCh:
			if !ok {
				return nil
			}
			if err != nil {
				return apperrors.ToGRPCStatus(err)
			}
			return nil
		}
		if outbound == nil {
			// outbound closed — drain errCh for the final (possibly nil) error.
			if err := <-errCh; err != nil {
				return apperrors.ToGRPCStatus(err)
			}
			return nil
		}
	}
}

// pumpAttachPtyInbound reads stream.Recv() until it errors/EOFs, translating
// each PtyClientFrame into usecase.PtyClientMessage and pushing it onto
// inbound; closes inbound when the client stream ends so
// usecase.AttachPty.run's read loop observes !ok and returns.
func pumpAttachPtyInbound(stream infrafleetv1.InfraFleetService_AttachPtyServer, inbound chan<- usecase.PtyClientMessage) {
	defer close(inbound)
	for {
		frame, err := stream.Recv()
		if err != nil {
			return // io.EOF (client closed send side) or a real transport error — either way, stop
		}
		msg, ok := toUsecasePtyClientMessage(frame)
		if !ok {
			continue // frame carried no oneof variant — ignore rather than error the whole stream
		}
		select {
		case inbound <- msg:
		case <-stream.Context().Done():
			return
		}
	}
}

func withTenantFromStreamMetadata(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	if v := md.Get(grpcmw.MetadataTenantID); len(v) > 0 && v[0] != "" {
		ctx = tenant.WithTenantID(ctx, v[0])
	}
	if v := md.Get(grpcmw.MetadataUserID); len(v) > 0 && v[0] != "" {
		ctx = tenant.WithUserID(ctx, v[0])
	}
	return ctx
}

func toUsecasePtyClientMessage(frame *infrafleetv1.PtyClientFrame) (usecase.PtyClientMessage, bool) {
	switch f := frame.GetFrame().(type) {
	case *infrafleetv1.PtyClientFrame_Attach:
		return usecase.PtyClientMessage{Attach: &usecase.PtyAttachMessage{PtyID: f.Attach.GetPtyId()}}, true
	case *infrafleetv1.PtyClientFrame_Input:
		return usecase.PtyClientMessage{Input: f.Input.GetData()}, true
	case *infrafleetv1.PtyClientFrame_Resize:
		return usecase.PtyClientMessage{Resize: &usecase.PtyResizeMessage{Cols: f.Resize.GetCols(), Rows: f.Resize.GetRows()}}, true
	default:
		return usecase.PtyClientMessage{}, false
	}
}

func toProtoPtyServerFrame(msg usecase.PtyServerMessage) *infrafleetv1.PtyServerFrame {
	if msg.Exited {
		return &infrafleetv1.PtyServerFrame{Frame: &infrafleetv1.PtyServerFrame_Exited{Exited: &infrafleetv1.PtyExited{ExitCode: msg.ExitCode}}}
	}
	return &infrafleetv1.PtyServerFrame{Frame: &infrafleetv1.PtyServerFrame_Out{Out: &infrafleetv1.PtyOutput{Data: msg.Output}}}
}

func toProtoTerminalSession(session domain.TerminalSession) *infrafleetv1.TerminalSession {
	return &infrafleetv1.TerminalSession{
		PtyId:              session.PtyID,
		ConnectionId:       session.ConnectionID,
		Cwd:                session.Cwd,
		CreatedAtUnixMs:    session.CreatedAt.UnixMilli(),
		LastActiveAtUnixMs: session.LastActiveAt.UnixMilli(),
	}
}

func toProtoBrowserProfile(p domain.BrowserProfile) *infrafleetv1.BrowserProfile {
	return &infrafleetv1.BrowserProfile{
		Id:            p.ID,
		TenantId:      p.TenantID,
		DevServerId:   p.DevServerID,
		Name:          p.Name,
		SourceBrowser: p.SourceBrowser,
		IsDefault:     p.IsDefault,
		CreatedAt:     timestamppb.New(p.CreatedAt),
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
