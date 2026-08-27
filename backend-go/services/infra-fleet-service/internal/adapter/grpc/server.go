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
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/portevents"
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

	// --- Terminal scrollback persistence (SOL-TM-03) ---
	saveTerminalScrollbackSnapshot    *usecase.SaveTerminalScrollbackSnapshot
	getTerminalScrollbackSnapshot     *usecase.GetTerminalScrollbackSnapshot
	deleteTerminalScrollbackSnapshots *usecase.DeleteTerminalScrollbackSnapshots

	// --- Emulator relay (TASK-048) / host capabilities relay (TASK-070) ---
	// Shipped-but-honestly-inert until agent/ gains device.*/host.capabilities
	// — see usecase.EmulatorRelay / usecase.GetHostCapabilities doc comments.
	emulatorRelay       *usecase.EmulatorRelay
	getHostCapabilities *usecase.GetHostCapabilities

	// --- CLI agent access (BUG-CLI-02) ---
	getAgentTerminalSession *usecase.GetAgentTerminalSession
	sendTerminalInput       *usecase.SendTerminalInput
	getTerminalScrollback   *usecase.GetTerminalScrollback

	importFleetInventory *usecase.ImportFleetInventory
	bulkProvisionFleet   *usecase.BulkProvisionFleet

	detectDevServerAgents   *usecase.DetectDevServerAgents
	checkDevServerPreflight *usecase.CheckDevServerPreflight

	// --- Persistent agent tokens (BL-AWS-03, TASK-AWS-03-07) ---
	createAgentToken *usecase.CreateAgentToken
	listAgentTokens  *usecase.ListAgentTokens
	revokeAgentToken *usecase.RevokeAgentToken

	// --- BR-SSH-13 reconnect cancellation ---
	teardownConnection *usecase.TeardownConnection

	// --- Auto port-forwarding (SOL-SSH-04) ---
	createPortForward *usecase.CreatePortForward
	listPortForwards  *usecase.ListPortForwards
	deletePortForward *usecase.DeletePortForward

	// --- Port-forward push notifications (TASK-SSH-04-08) --- shared with
	// PollWorkspacePorts as its usecase.PortForwardEventPublisher.
	portEvents *portevents.Broadcaster
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
	saveTerminalScrollbackSnapshot *usecase.SaveTerminalScrollbackSnapshot,
	getTerminalScrollbackSnapshot *usecase.GetTerminalScrollbackSnapshot,
	deleteTerminalScrollbackSnapshots *usecase.DeleteTerminalScrollbackSnapshots,
	getAgentTerminalSession *usecase.GetAgentTerminalSession,
	sendTerminalInput *usecase.SendTerminalInput,
	getTerminalScrollback *usecase.GetTerminalScrollback,
	importFleetInventory *usecase.ImportFleetInventory,
	bulkProvisionFleet *usecase.BulkProvisionFleet,
	detectDevServerAgents *usecase.DetectDevServerAgents,
	checkDevServerPreflight *usecase.CheckDevServerPreflight,
	createAgentToken *usecase.CreateAgentToken,
	listAgentTokens *usecase.ListAgentTokens,
	revokeAgentToken *usecase.RevokeAgentToken,
	teardownConnection *usecase.TeardownConnection,
	createPortForward *usecase.CreatePortForward,
	listPortForwards *usecase.ListPortForwards,
	deletePortForward *usecase.DeletePortForward,
	portEvents *portevents.Broadcaster,
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

		saveTerminalScrollbackSnapshot:    saveTerminalScrollbackSnapshot,
		getTerminalScrollbackSnapshot:     getTerminalScrollbackSnapshot,
		deleteTerminalScrollbackSnapshots: deleteTerminalScrollbackSnapshots,

		getAgentTerminalSession: getAgentTerminalSession,
		sendTerminalInput:       sendTerminalInput,
		getTerminalScrollback:   getTerminalScrollback,

		importFleetInventory: importFleetInventory,
		bulkProvisionFleet:   bulkProvisionFleet,

		detectDevServerAgents:   detectDevServerAgents,
		checkDevServerPreflight: checkDevServerPreflight,

		createAgentToken: createAgentToken,
		listAgentTokens:  listAgentTokens,
		revokeAgentToken: revokeAgentToken,

		teardownConnection: teardownConnection,
		createPortForward:  createPortForward,
		listPortForwards:   listPortForwards,
		deletePortForward:  deletePortForward,
		portEvents:         portEvents,
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
		Host:                  req.GetHost(),
		Port:                  int(req.GetPort()),
		UserName:              req.GetUser(),
		VaultSSHRole:          req.GetVaultSshRole(),
		KnownHostsFingerprint: req.GetKnownHostsFingerprint(),
		JumpHostTargetID:      req.GetJumpHostTargetId(),
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
	detected, err := s.scanWorkspacePorts.Execute(ctx, usecase.ScanWorkspacePortsInput{
		ConnectionID: req.GetConnectionId(),
		WorktreeID:   req.GetWorktreeId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*infrafleetv1.DetectedPortProto, 0, len(detected))
	for _, d := range detected {
		out = append(out, &infrafleetv1.DetectedPortProto{
			Port:        d.Port,
			Host:        d.Host,
			Pid:         d.PID,
			ProcessName: d.ProcessName,
		})
	}
	return &infrafleetv1.ScanWorkspacePortsResponse{Ports: out}, nil
}

func (s *Server) ListSshTargets(ctx context.Context, req *infrafleetv1.ListSshTargetsRequest) (*infrafleetv1.ListSshTargetsResponse, error) {
	targets, err := s.listSshTargets.Execute(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*infrafleetv1.SshTarget, 0, len(targets))
	for _, t := range targets {
		out = append(out, &infrafleetv1.SshTarget{
			Id:                    t.ID,
			TenantId:              t.TenantID,
			Host:                  t.Host,
			Port:                  int32(t.Port),
			User:                  t.UserName,
			VaultSshRole:          t.VaultSSHRole,
			KnownHostsFingerprint: t.KnownHostsFingerprint,
			JumpHostTargetId:      t.JumpHostTargetID,
		})
	}
	return &infrafleetv1.ListSshTargetsResponse{SshTargets: out}, nil
}

// ImportFleetInventory is BL-FLEET-01's batch YAML-import entry point —
// see usecase.ImportFleetInventory's doc comment for the upsert semantics.
func (s *Server) ImportFleetInventory(ctx context.Context, req *infrafleetv1.ImportFleetInventoryRequest) (*infrafleetv1.ImportFleetInventoryResponse, error) {
	servers := make([]usecase.FleetServerInput, 0, len(req.GetServers()))
	for _, sv := range req.GetServers() {
		servers = append(servers, usecase.FleetServerInput{
			Host: sv.GetHost(), UserName: sv.GetUser(), VaultSSHRole: sv.GetVaultSshRole(),
			Project: sv.GetProject(), Tags: sv.GetTags(),
		})
	}
	result, err := s.importFleetInventory.Execute(ctx, usecase.ImportFleetInventoryInput{Servers: servers, DryRun: req.GetDryRun()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	resp := &infrafleetv1.ImportFleetInventoryResponse{
		Imported: int32(result.Imported), Updated: int32(result.Updated), Skipped: int32(result.Skipped),
	}
	for _, e := range result.Errors {
		resp.Errors = append(resp.Errors, &infrafleetv1.ImportFleetInventoryError{Host: e.Host, User: e.UserName, Reason: e.Reason})
	}
	return resp, nil
}

// BulkProvisionFleet is BL-FLEET-02's fan-out batch-provision entry point —
// see usecase.BulkProvisionFleet's doc comment.
func (s *Server) BulkProvisionFleet(ctx context.Context, req *infrafleetv1.BulkProvisionFleetRequest) (*infrafleetv1.BulkProvisionFleetResponse, error) {
	result, err := s.bulkProvisionFleet.Execute(ctx, usecase.BulkProvisionFleetInput{
		Project: req.GetProject(), Concurrency: int(req.GetConcurrency()),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	resp := &infrafleetv1.BulkProvisionFleetResponse{
		Success: int32(result.Success), Failed: int32(result.Failed), Skipped: int32(result.Skipped),
	}
	for _, o := range result.Outcomes {
		resp.Outcomes = append(resp.Outcomes, &infrafleetv1.ProvisionOutcome{
			DevServerId: o.DevServerID, Host: o.Host, Status: o.Status, Error: o.Error,
		})
	}
	return resp, nil
}

// DetectDevServerAgents closes BL-FLEET-04 Step 3 — see
// usecase.DetectDevServerAgents's doc comment.
func (s *Server) DetectDevServerAgents(ctx context.Context, req *infrafleetv1.DetectDevServerAgentsRequest) (*infrafleetv1.DetectDevServerAgentsResponse, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err))
	}
	result, err := s.detectDevServerAgents.Execute(ctx, tenantID, req.GetDevServerId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.DetectDevServerAgentsResponse{Agents: result.Agents, Platform: result.Platform}, nil
}

// CheckDevServerPreflight closes BL-FLEET-04 Step 4 — see
// usecase.CheckDevServerPreflight's doc comment.
func (s *Server) CheckDevServerPreflight(ctx context.Context, req *infrafleetv1.CheckDevServerPreflightRequest) (*infrafleetv1.CheckDevServerPreflightResponse, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err))
	}
	result, err := s.checkDevServerPreflight.Execute(ctx, tenantID, req.GetDevServerId(), req.GetProbePort())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.CheckDevServerPreflightResponse{
		Git:  &infrafleetv1.CheckResult{Installed: result.Git.Installed, Version: result.Git.Version, MeetsMin: result.Git.MeetsMin},
		Node: &infrafleetv1.CheckResult{Installed: result.Node.Installed, Version: result.Node.Version, MeetsMin: result.Node.MeetsMin},
		Disk: &infrafleetv1.DiskCheckResult{FreeGb: result.Disk.FreeGB, MeetsMin: result.Disk.MeetsMin},
		Port: &infrafleetv1.PortCheckResult{Port: result.Port.Port, Available: result.Port.Available},
		Gh:   &infrafleetv1.CheckResult{Installed: result.GH.Installed, Version: result.GH.Version, MeetsMin: result.GH.MeetsMin},
	}, nil
}

func (s *Server) GetSshState(ctx context.Context, req *infrafleetv1.GetSshStateRequest) (*infrafleetv1.GetSshStateResponse, error) {
	state, err := s.getSshState.Execute(ctx, usecase.SshStateInput{SshTargetID: req.GetSshTargetId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	resp := &infrafleetv1.GetSshStateResponse{Connected: state.Connected, ConnectionId: state.ConnectionID, Status: state.Status}
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

func (s *Server) TeardownConnection(ctx context.Context, req *infrafleetv1.TeardownConnectionRequest) (*emptypb.Empty, error) {
	if err := s.teardownConnection.Execute(ctx, usecase.TeardownConnectionInput{ConnectionID: req.GetConnectionId()}); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
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

func (s *Server) CreatePortForward(ctx context.Context, req *infrafleetv1.CreatePortForwardRequest) (*infrafleetv1.PortForward, error) {
	pf, err := s.createPortForward.Execute(ctx, usecase.CreatePortForwardInput{
		ConnectionID: req.GetConnectionId(),
		RemotePort:   int(req.GetRemotePort()),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoPortForward(pf), nil
}

func (s *Server) ListPortForwards(ctx context.Context, req *infrafleetv1.ListPortForwardsRequest) (*infrafleetv1.ListPortForwardsResponse, error) {
	forwards, err := s.listPortForwards.Execute(ctx, usecase.ListPortForwardsInput{ConnectionID: req.GetConnectionId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*infrafleetv1.PortForward, 0, len(forwards))
	for _, pf := range forwards {
		out = append(out, toProtoPortForward(pf))
	}
	return &infrafleetv1.ListPortForwardsResponse{PortForwards: out}, nil
}

func (s *Server) DeletePortForward(ctx context.Context, req *infrafleetv1.DeletePortForwardRequest) (*emptypb.Empty, error) {
	if err := s.deletePortForward.Execute(ctx, usecase.DeletePortForwardInput{ID: req.GetId()}); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func toProtoPortForward(pf domain.PortForward) *infrafleetv1.PortForward {
	return &infrafleetv1.PortForward{
		Id:           pf.ID,
		ConnectionId: pf.ConnectionID,
		LocalPort:    int32(pf.LocalPort),
		RemotePort:   int32(pf.RemotePort),
		ProcessName:  pf.ProcessName,
		Status:       string(pf.Status),
	}
}

// StreamPortForwardEvents pushes portevents.Broadcaster's per-connectionId
// port_opened/port_closed events to the caller for as long as the stream
// stays open — BR-SSH-15's live-push requirement (TASK-SSH-04-08), the same
// "open a stream, forward each item" shape AttachPty already uses.
func (s *Server) StreamPortForwardEvents(req *infrafleetv1.StreamPortForwardEventsRequest, stream infrafleetv1.InfraFleetService_StreamPortForwardEventsServer) error {
	events, unsubscribe := s.portEvents.Subscribe(req.GetConnectionId())
	defer unsubscribe()
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return nil
			}
			if err := stream.Send(&infrafleetv1.PortForwardEvent{
				Kind:    ev.Kind,
				Forward: toProtoPortForward(ev.Forward),
			}); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return nil
		}
	}
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
		Id:           ds.ID,
		TenantId:     ds.TenantID,
		Host:         ds.Host,
		Mode:         toProtoConnectionMode(ds.Mode),
		SshTargetId:  ds.SSHTargetID,
		Status:       string(ds.Status),
		Platform:     ds.Platform,
		Arch:         ds.Arch,
		NodeVersion:  ds.NodeVersion,
		AgentVersion: ds.AgentVersion,
	}
}

// --- Terminal/PTY (TASK-185) ---

func (s *Server) SpawnTerminalSession(ctx context.Context, req *infrafleetv1.SpawnTerminalSessionRequest) (*infrafleetv1.SpawnTerminalSessionResponse, error) {
	session, err := s.spawnTerminalSession.Execute(ctx, usecase.SpawnTerminalSessionInput{
		ConnectionID:     req.GetConnectionId(),
		Cwd:              req.GetCwd(),
		Shell:            req.GetShell(),
		Cols:             req.GetCols(),
		Rows:             req.GetRows(),
		ShellIntegration: req.GetShellIntegration(),
		Command:          req.GetCommand(),
		UserID:           req.GetUserId(),
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

func (s *Server) GetAgentTerminalSession(ctx context.Context, req *infrafleetv1.GetAgentTerminalSessionRequest) (*infrafleetv1.GetAgentTerminalSessionResponse, error) {
	session, found, err := s.getAgentTerminalSession.Execute(ctx, req.GetWorktreeId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	resp := &infrafleetv1.GetAgentTerminalSessionResponse{Found: found}
	if found {
		resp.Session = toProtoTerminalSession(session)
	}
	return resp, nil
}

func (s *Server) SendTerminalInput(ctx context.Context, req *infrafleetv1.SendTerminalInputRequest) (*emptypb.Empty, error) {
	if err := s.sendTerminalInput.Execute(ctx, req.GetPtyId(), req.GetData()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) GetTerminalScrollback(ctx context.Context, req *infrafleetv1.GetTerminalScrollbackRequest) (*infrafleetv1.GetTerminalScrollbackResponse, error) {
	result, err := s.getTerminalScrollback.Execute(ctx, req.GetPtyId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.GetTerminalScrollbackResponse{Text: result.Text, Truncated: result.Truncated}, nil
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

// --- Terminal scrollback persistence (SOL-TM-03) ---

func (s *Server) SaveTerminalScrollbackSnapshot(ctx context.Context, req *infrafleetv1.SaveTerminalScrollbackSnapshotRequest) (*emptypb.Empty, error) {
	err := s.saveTerminalScrollbackSnapshot.Execute(ctx, usecase.SaveTerminalScrollbackSnapshotInput{
		WorktreeID: req.GetWorktreeId(), PaneKey: req.GetPaneKey(),
		Cols: req.GetCols(), Rows: req.GetRows(), Data: req.GetData(), LastTitle: req.GetLastTitle(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) GetTerminalScrollbackSnapshot(ctx context.Context, req *infrafleetv1.GetTerminalScrollbackSnapshotRequest) (*infrafleetv1.GetTerminalScrollbackSnapshotResponse, error) {
	result, err := s.getTerminalScrollbackSnapshot.Execute(ctx, req.GetWorktreeId(), req.GetPaneKey())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.GetTerminalScrollbackSnapshotResponse{
		Found: result.Found, Cols: result.Cols, Rows: result.Rows, Data: result.Data,
		LastTitle: result.LastTitle, UpdatedAtUnixMs: result.UpdatedAt.UnixMilli(),
	}, nil
}

func (s *Server) DeleteTerminalScrollbackSnapshots(ctx context.Context, req *infrafleetv1.DeleteTerminalScrollbackSnapshotsRequest) (*emptypb.Empty, error) {
	if err := s.deleteTerminalScrollbackSnapshots.Execute(ctx, req.GetWorktreeId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
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
