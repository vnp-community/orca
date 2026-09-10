// Package grpc implements the generated infrafleetv1.InfraFleetServiceServer
// interface by translating wire messages to/from usecase calls — no
// business logic here, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md's
// inbound-adapter contract.
package grpc

import (
	"context"
	"encoding/json"
	"sync"
	"time"

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

<<<<<<< HEAD
	registerDevServer   *usecase.RegisterDevServer
	resolveConnection   *usecase.ResolveConnection
	createSshTarget     *usecase.CreateSshTarget
	getFleetHealth      *usecase.GetFleetHealth
	scanWorkspacePorts  *usecase.ScanWorkspacePorts
	listDevServers      *usecase.ListDevServers
	listDevServersByTag *usecase.ListDevServersByTag
	createConnection    *usecase.CreateConnection
	relay               *usecase.Relay
	relayStream         *usecase.RelayStream
=======
	registerDevServer  *usecase.RegisterDevServer
	resolveConnection  *usecase.ResolveConnection
	createSshTarget    *usecase.CreateSshTarget
	bulkProvisionFleet *usecase.BulkProvisionFleet
	applyTerraformPlan *usecase.ApplyTerraformPlan

	createFleetDefinition     *usecase.CreateFleetDefinition
	updateFleetDefinition     *usecase.UpdateFleetDefinition
	getFleetDefinition        *usecase.GetFleetDefinition
	listFleetDefinitions      *usecase.ListFleetDefinitions
	exportFleetDefinitionYaml *usecase.ExportFleetDefinitionYaml
	deployFleetDefinition     *usecase.DeployFleetDefinition

	getFleetHealth     *usecase.GetFleetHealth
	scanWorkspacePorts *usecase.ScanWorkspacePorts
	listDevServers     *usecase.ListDevServers
	createConnection   *usecase.CreateConnection
	relay              *usecase.Relay
>>>>>>> feat/team-rbac-implementation

	listSshTargets      *usecase.ListSshTargets
	getSshState         *usecase.GetSshState
	establishConnection *usecase.EstablishConnection
	killWorkspacePort   *usecase.KillWorkspacePort
	// teardownConnection backs the confirmed-logout explicit-close RPC
	// (BE-SOL-STORAGE-003 §5, TASK-BE-STORAGE-012).
	teardownConnection *usecase.TeardownConnection
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
	attachScreencast       *usecase.AttachScreencast
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

	// --- Auto port-forwarding (SOL-SSH-04) ---
	createPortForward *usecase.CreatePortForward
	listPortForwards  *usecase.ListPortForwards
	deletePortForward *usecase.DeletePortForward

	// --- Port-forward push notifications (TASK-SSH-04-08) --- shared with
	// PollWorkspacePorts as its usecase.PortForwardEventPublisher.
	portEvents *portevents.Broadcaster

	// --- Agent sessions (TASK-AG-01..04) ---
	startAgentSession  *usecase.StartAgentSession
	stopAgentSession   *usecase.StopAgentSession
	killAgentSession   *usecase.KillAgentSession
	resumeAgentSession *usecase.ResumeAgentSession
	switchAgentAccount *usecase.SwitchAgentAccount
	// --- Mobile prompt dispatch (SOL-MB-03) ---
	dispatchPrompt  *usecase.DispatchPrompt
	getQueuedPrompt *usecase.GetQueuedPrompt

	// liveStates is the SAME per-pod quiescence registry AttachPty/
	// GetTerminalAgentStatus share (TASK-MB-02-01) — read here only to
	// populate ListTerminalSessions/SpawnTerminalSession's
	// TerminalSession.LastOutputPreview (TASK-MB-04-02), never written.
	liveStates *sync.Map

	// --- CR-DS-006 Phase 2 / CR-DS-007 / CR-DS-008 (dev server access control) ---
	approveDevServer           *usecase.ApproveDevServer
	rejectDevServer            *usecase.RejectDevServer
	assignDevServerGroup       *usecase.AssignDevServerGroup
	createDevServerGroup       *usecase.CreateDevServerGroup
	listDevServerGroups        *usecase.ListDevServerGroups
	grantDevServerGroupAccess  *usecase.GrantDevServerGroupAccess
	revokeDevServerGroupAccess *usecase.RevokeDevServerGroupAccess
	listDevServerGroupGrants   *usecase.ListDevServerGroupGrants
	listDevServersForUser      *usecase.ListDevServersForUser
	createAccessRequest        *usecase.CreateAccessRequest
	listPendingAccessRequests  *usecase.ListPendingAccessRequests
	resolveAccessRequest       *usecase.ResolveAccessRequest

	relayByDevServer     *usecase.RelayByDevServer
	isDevServerConnected *usecase.IsDevServerConnected

	// --- Ephemeral VM (SOL-004 Group 1/2a, TASK-002/004) ---
	listEphemeralVmRuntimes *usecase.ListEphemeralVmRuntimes
	ephemeralVmRelay        *usecase.EphemeralVmRelay

	// getFleetConnectivitySummary backs CR-STORAGE-007's poll-driven health
	// summary (TASK-BE-STORAGE-006) — see usecase.GetFleetConnectivitySummary's
	// doc comment.
	getFleetConnectivitySummary *usecase.GetFleetConnectivitySummary

	// streamFileChanges backs BACKLOG-003's file-watch streaming RPC.
	streamFileChanges *usecase.StreamFileChanges
}

func New(
	registerDevServer *usecase.RegisterDevServer,
	resolveConnection *usecase.ResolveConnection,
	createSshTarget *usecase.CreateSshTarget,
	bulkProvisionFleet *usecase.BulkProvisionFleet,
	applyTerraformPlan *usecase.ApplyTerraformPlan,
	createFleetDefinition *usecase.CreateFleetDefinition,
	updateFleetDefinition *usecase.UpdateFleetDefinition,
	getFleetDefinition *usecase.GetFleetDefinition,
	listFleetDefinitions *usecase.ListFleetDefinitions,
	exportFleetDefinitionYaml *usecase.ExportFleetDefinitionYaml,
	deployFleetDefinition *usecase.DeployFleetDefinition,
	getFleetHealth *usecase.GetFleetHealth,
	scanWorkspacePorts *usecase.ScanWorkspacePorts,
	listDevServers *usecase.ListDevServers,
	listDevServersByTag *usecase.ListDevServersByTag,
	createConnection *usecase.CreateConnection,
	relay *usecase.Relay,
	relayStream *usecase.RelayStream,
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
	attachScreencast *usecase.AttachScreencast,
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
	startAgentSession *usecase.StartAgentSession,
	stopAgentSession *usecase.StopAgentSession,
	killAgentSession *usecase.KillAgentSession,
	resumeAgentSession *usecase.ResumeAgentSession,
	switchAgentAccount *usecase.SwitchAgentAccount,
	dispatchPrompt *usecase.DispatchPrompt,
	getQueuedPrompt *usecase.GetQueuedPrompt,
	liveStates *sync.Map,
	approveDevServer *usecase.ApproveDevServer,
	rejectDevServer *usecase.RejectDevServer,
	assignDevServerGroup *usecase.AssignDevServerGroup,
	createDevServerGroup *usecase.CreateDevServerGroup,
	listDevServerGroups *usecase.ListDevServerGroups,
	grantDevServerGroupAccess *usecase.GrantDevServerGroupAccess,
	revokeDevServerGroupAccess *usecase.RevokeDevServerGroupAccess,
	listDevServerGroupGrants *usecase.ListDevServerGroupGrants,
	listDevServersForUser *usecase.ListDevServersForUser,
	createAccessRequest *usecase.CreateAccessRequest,
	listPendingAccessRequests *usecase.ListPendingAccessRequests,
	resolveAccessRequest *usecase.ResolveAccessRequest,
	relayByDevServer *usecase.RelayByDevServer,
	isDevServerConnected *usecase.IsDevServerConnected,
	listEphemeralVmRuntimes *usecase.ListEphemeralVmRuntimes,
	ephemeralVmRelay *usecase.EphemeralVmRelay,
	getFleetConnectivitySummary *usecase.GetFleetConnectivitySummary,
	streamFileChanges *usecase.StreamFileChanges,
) *Server {
	return &Server{
<<<<<<< HEAD
		registerDevServer:      registerDevServer,
		resolveConnection:      resolveConnection,
		createSshTarget:        createSshTarget,
		getFleetHealth:         getFleetHealth,
		scanWorkspacePorts:     scanWorkspacePorts,
		listDevServers:         listDevServers,
		listDevServersByTag:    listDevServersByTag,
		createConnection:       createConnection,
		relay:                  relay,
		relayStream:            relayStream,
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

		startAgentSession:          startAgentSession,
		stopAgentSession:           stopAgentSession,
		killAgentSession:           killAgentSession,
		resumeAgentSession:         resumeAgentSession,
		switchAgentAccount:         switchAgentAccount,
		dispatchPrompt:             dispatchPrompt,
		getQueuedPrompt:            getQueuedPrompt,
		liveStates:                 liveStates,
=======
		registerDevServer:          registerDevServer,
		resolveConnection:          resolveConnection,
		createSshTarget:            createSshTarget,
		bulkProvisionFleet:         bulkProvisionFleet,
		applyTerraformPlan:         applyTerraformPlan,
		createFleetDefinition:      createFleetDefinition,
		updateFleetDefinition:      updateFleetDefinition,
		getFleetDefinition:         getFleetDefinition,
		listFleetDefinitions:       listFleetDefinitions,
		exportFleetDefinitionYaml:  exportFleetDefinitionYaml,
		deployFleetDefinition:      deployFleetDefinition,
		getFleetHealth:             getFleetHealth,
		scanWorkspacePorts:         scanWorkspacePorts,
		listDevServers:             listDevServers,
		createConnection:           createConnection,
		relay:                      relay,
		listSshTargets:             listSshTargets,
		getSshState:                getSshState,
		establishConnection:        establishConnection,
		killWorkspacePort:          killWorkspacePort,
		spawnTerminalSession:       spawnTerminalSession,
		resizeTerminalSession:      resizeTerminalSession,
		killTerminalSession:        killTerminalSession,
		stopTerminalProcess:        stopTerminalProcess,
		listTerminalSessions:       listTerminalSessions,
		waitTerminalSession:        waitTerminalSession,
		focusTerminalSession:       focusTerminalSession,
		getTerminalAgentStatus:     getTerminalAgentStatus,
		inspectTerminalProcess:     inspectTerminalProcess,
		attachPty:                  attachPty,
		attachScreencast:           attachScreencast,
		listBrowserProfiles:        listBrowserProfiles,
		createBrowserProfile:       createBrowserProfile,
		deleteBrowserProfile:       deleteBrowserProfile,
		emulatorRelay:              emulatorRelay,
		getHostCapabilities:        getHostCapabilities,
>>>>>>> feat/team-rbac-implementation
		approveDevServer:           approveDevServer,
		rejectDevServer:            rejectDevServer,
		assignDevServerGroup:       assignDevServerGroup,
		createDevServerGroup:       createDevServerGroup,
		listDevServerGroups:        listDevServerGroups,
		grantDevServerGroupAccess:  grantDevServerGroupAccess,
		revokeDevServerGroupAccess: revokeDevServerGroupAccess,
		listDevServerGroupGrants:   listDevServerGroupGrants,
		listDevServersForUser:      listDevServersForUser,
		createAccessRequest:        createAccessRequest,
		listPendingAccessRequests:  listPendingAccessRequests,
		resolveAccessRequest:       resolveAccessRequest,
		relayByDevServer:           relayByDevServer,
		isDevServerConnected:       isDevServerConnected,

		listEphemeralVmRuntimes: listEphemeralVmRuntimes,
		ephemeralVmRelay:        ephemeralVmRelay,

		getFleetConnectivitySummary: getFleetConnectivitySummary,

		streamFileChanges: streamFileChanges,
	}
}

// TeardownConnection backs BE-SOL-STORAGE-003 §5's confirmed-logout
// explicit-close path — tenant scoping comes from the authenticated
// context, per infrafleet.proto's TeardownConnectionRequest doc comment.
func (s *Server) TeardownConnection(ctx context.Context, req *infrafleetv1.TeardownConnectionRequest) (*emptypb.Empty, error) {
	if err := s.teardownConnection.Execute(ctx, req.GetConnectionId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

// GetFleetConnectivitySummary backs CR-STORAGE-007's poll-driven health
// summary — see usecase.GetFleetConnectivitySummary's doc comment.
// tenant/user scoping comes from the authenticated identity in ctx, never
// from req (which is deliberately empty, see infrafleet.proto's
// GetFleetConnectivitySummaryRequest doc comment).
func (s *Server) GetFleetConnectivitySummary(ctx context.Context, req *infrafleetv1.GetFleetConnectivitySummaryRequest) (*infrafleetv1.GetFleetConnectivitySummaryResponse, error) {
	conns, err := s.getFleetConnectivitySummary.Execute(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*infrafleetv1.ConnectionHealthEntry, 0, len(conns))
	for _, conn := range conns {
		out = append(out, toProtoConnectionHealthEntry(conn))
	}
	return &infrafleetv1.GetFleetConnectivitySummaryResponse{Connections: out}, nil
}

// toProtoConnectionHealthEntry maps a domain.Connection to the wire shape
// GetFleetConnectivitySummary returns — LastActivityAt/DegradedSince stay
// unset (nil) on the proto message when the domain field is nil, never a
// fabricated zero timestamp (see infrafleet.proto's ConnectionHealthEntry
// doc comment: "unset if never active" / "unset unless status == degraded").
func toProtoConnectionHealthEntry(conn domain.Connection) *infrafleetv1.ConnectionHealthEntry {
	entry := &infrafleetv1.ConnectionHealthEntry{
		ConnectionId: conn.ID,
		DevServerId:  conn.DevServerID,
		Status:       conn.Status,
	}
	if conn.LastActivityAt != nil {
		entry.LastActivityAt = timestamppb.New(*conn.LastActivityAt)
	}
	if conn.DegradedSince != nil {
		entry.DegradedSince = timestamppb.New(*conn.DegradedSince)
	}
	return entry
}

func (s *Server) RegisterDevServer(ctx context.Context, req *infrafleetv1.RegisterDevServerRequest) (*infrafleetv1.RegisterDevServerResponse, error) {
	devServer, err := s.registerDevServer.Execute(ctx, usecase.RegisterDevServerInput{
		Host:        req.GetHost(),
		Mode:        toDomainConnectionMode(req.GetMode()),
		SSHTargetID: req.GetSshTargetId(),
		Tags:        req.GetTags(),
		Kind:        toDomainAgentKind(req.GetKind()),
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
		resp.HiddenTargetId = out.HiddenTargetID
	}
	return resp, nil
}

// ListDevServers backs the frontend's devServer.list channel (wired through
// api-gateway's wscompat) — see usecase.ListDevServers's doc comment.
func (s *Server) ListDevServers(ctx context.Context, req *infrafleetv1.ListDevServersRequest) (*infrafleetv1.ListDevServersResponse, error) {
	devServers, err := s.listDevServers.Execute(ctx, toDomainAgentKind(req.GetKind()))
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*infrafleetv1.DevServer, 0, len(devServers))
	for _, ds := range devServers {
		out = append(out, toProtoDevServer(ds))
	}
	return &infrafleetv1.ListDevServersResponse{DevServers: out}, nil
}

// ListDevServersByTag backs workflow-service's "fleet:tag:<tag>"
// dispatch-target shape — see usecase.ListDevServersByTag's doc comment.
func (s *Server) ListDevServersByTag(ctx context.Context, req *infrafleetv1.ListDevServersByTagRequest) (*infrafleetv1.ListDevServersByTagResponse, error) {
	devServers, err := s.listDevServersByTag.Execute(ctx, usecase.ListDevServersByTagInput{
		Tag:         req.GetTag(),
		HealthyOnly: req.GetHealthyOnly(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*infrafleetv1.DevServer, 0, len(devServers))
	for _, ds := range devServers {
		out = append(out, toProtoDevServer(ds))
	}
	return &infrafleetv1.ListDevServersByTagResponse{DevServers: out}, nil
}

// --- CR-DS-006 Phase 2 / CR-DS-007 / CR-DS-008 (dev server access control) ---

func (s *Server) ApproveDevServer(ctx context.Context, req *infrafleetv1.ApproveDevServerRequest) (*infrafleetv1.ApproveDevServerResponse, error) {
	ds, err := s.approveDevServer.Execute(ctx, req.GetDevServerId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.ApproveDevServerResponse{DevServer: toProtoDevServer(ds)}, nil
}

func (s *Server) RejectDevServer(ctx context.Context, req *infrafleetv1.RejectDevServerRequest) (*infrafleetv1.RejectDevServerResponse, error) {
	ds, err := s.rejectDevServer.Execute(ctx, usecase.RejectDevServerInput{
		DevServerID: req.GetDevServerId(),
		Reason:      req.GetReason(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.RejectDevServerResponse{DevServer: toProtoDevServer(ds)}, nil
}

func (s *Server) AssignDevServerGroup(ctx context.Context, req *infrafleetv1.AssignDevServerGroupRequest) (*infrafleetv1.AssignDevServerGroupResponse, error) {
	ds, err := s.assignDevServerGroup.Execute(ctx, usecase.AssignDevServerGroupInput{
		DevServerID: req.GetDevServerId(),
		GroupID:     req.GetGroupId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.AssignDevServerGroupResponse{DevServer: toProtoDevServer(ds)}, nil
}

func (s *Server) CreateDevServerGroup(ctx context.Context, req *infrafleetv1.CreateDevServerGroupRequest) (*infrafleetv1.CreateDevServerGroupResponse, error) {
	group, err := s.createDevServerGroup.Execute(ctx, usecase.CreateDevServerGroupInput{
		Name:          req.GetName(),
		ParentGroupID: req.GetParentGroupId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.CreateDevServerGroupResponse{Group: toProtoDevServerGroup(group)}, nil
}

func (s *Server) ListDevServerGroups(ctx context.Context, req *infrafleetv1.ListDevServerGroupsRequest) (*infrafleetv1.ListDevServerGroupsResponse, error) {
	groups, err := s.listDevServerGroups.Execute(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*infrafleetv1.DevServerGroup, 0, len(groups))
	for _, g := range groups {
		out = append(out, toProtoDevServerGroup(g))
	}
	return &infrafleetv1.ListDevServerGroupsResponse{Groups: out}, nil
}

func (s *Server) GrantDevServerGroupAccess(ctx context.Context, req *infrafleetv1.GrantDevServerGroupAccessRequest) (*infrafleetv1.GrantDevServerGroupAccessResponse, error) {
	grant, err := s.grantDevServerGroupAccess.Execute(ctx, usecase.GrantDevServerGroupAccessInput{
		DevServerGroupID: req.GetDevServerGroupId(),
		GranteeKind:      toDomainGranteeKind(req.GetGranteeKind()),
		GranteeID:        req.GetGranteeId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.GrantDevServerGroupAccessResponse{Grant: toProtoGrant(grant)}, nil
}

func (s *Server) RevokeDevServerGroupAccess(ctx context.Context, req *infrafleetv1.RevokeDevServerGroupAccessRequest) (*infrafleetv1.RevokeDevServerGroupAccessResponse, error) {
	if err := s.revokeDevServerGroupAccess.Execute(ctx, req.GetGrantId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.RevokeDevServerGroupAccessResponse{}, nil
}

func (s *Server) ListDevServerGroupGrants(ctx context.Context, req *infrafleetv1.ListDevServerGroupGrantsRequest) (*infrafleetv1.ListDevServerGroupGrantsResponse, error) {
	grants, err := s.listDevServerGroupGrants.Execute(ctx, req.GetDevServerGroupId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*infrafleetv1.DevServerGroupGrant, 0, len(grants))
	for _, g := range grants {
		out = append(out, toProtoGrant(g))
	}
	return &infrafleetv1.ListDevServerGroupGrantsResponse{Grants: out}, nil
}

func (s *Server) ListDevServersForUser(ctx context.Context, req *infrafleetv1.ListDevServersForUserRequest) (*infrafleetv1.ListDevServersForUserResponse, error) {
	devServers, err := s.listDevServersForUser.Execute(ctx, usecase.ListDevServersForUserInput{
		DepartmentID: req.GetDepartmentId(),
		TeamIDs:      req.GetTeamIds(),
		Kind:         toDomainAgentKind(req.GetKind()),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*infrafleetv1.DevServer, 0, len(devServers))
	for _, ds := range devServers {
		out = append(out, toProtoDevServer(ds))
	}
	return &infrafleetv1.ListDevServersForUserResponse{DevServers: out}, nil
}

func (s *Server) CreateAccessRequest(ctx context.Context, req *infrafleetv1.CreateAccessRequestRequest) (*infrafleetv1.CreateAccessRequestResponse, error) {
	out, err := s.createAccessRequest.Execute(ctx, usecase.CreateAccessRequestInput{
		DevServerGroupID: req.GetDevServerGroupId(),
		Message:          req.GetMessage(),
		GranteeKind:      toDomainGranteeKind(req.GetGranteeKind()),
		GranteeID:        req.GetGranteeId(),
		NowUnixMs:        nowUnixMs(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.CreateAccessRequestResponse{Request: toProtoAccessRequest(out)}, nil
}

func (s *Server) ListPendingAccessRequests(ctx context.Context, req *infrafleetv1.ListPendingAccessRequestsRequest) (*infrafleetv1.ListPendingAccessRequestsResponse, error) {
	reqs, err := s.listPendingAccessRequests.Execute(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*infrafleetv1.DevServerAccessRequest, 0, len(reqs))
	for _, r := range reqs {
		out = append(out, toProtoAccessRequest(r))
	}
	return &infrafleetv1.ListPendingAccessRequestsResponse{Requests: out}, nil
}

func (s *Server) ResolveAccessRequest(ctx context.Context, req *infrafleetv1.ResolveAccessRequestRequest) (*infrafleetv1.ResolveAccessRequestResponse, error) {
	out, err := s.resolveAccessRequest.Execute(ctx, usecase.ResolveAccessRequestInput{
		RequestID: req.GetRequestId(),
		Approve:   req.GetApprove(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	resp := &infrafleetv1.ResolveAccessRequestResponse{Request: toProtoAccessRequest(out.Request)}
	if req.GetApprove() {
		resp.Grant = toProtoGrant(out.Grant)
	}
	return resp, nil
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

// RelayStream is Relay's server-streaming counterpart — see
// usecase.RelayStream's doc comment.
func (s *Server) RelayStream(req *infrafleetv1.RelayStreamRequest, stream infrafleetv1.InfraFleetService_RelayStreamServer) error {
	var params map[string]any
	if raw := req.GetParamsJson(); raw != "" {
		if err := json.Unmarshal([]byte(raw), &params); err != nil {
			return apperrors.ToGRPCStatus(apperrors.New(apperrors.KindInvalidArgument, "INFRA_RELAY_STREAM_BAD_PARAMS", "params_json must be a JSON object", err))
		}
	}

	err := s.relayStream.Execute(stream.Context(), usecase.RelayStreamInput{
		ConnectionID: req.GetConnectionId(),
		Method:       req.GetMethod(),
		Params:       params,
	}, func(frame map[string]any) error {
		frameJSON, err := json.Marshal(frame)
		if err != nil {
			return apperrors.New(apperrors.KindInternal, "INFRA_RELAY_STREAM_ENCODE_FAILED", "failed to encode relay stream frame", err)
		}
		return stream.Send(&infrafleetv1.RelayStreamFrame{FrameJson: string(frameJSON)})
	})
	if err != nil {
		return apperrors.ToGRPCStatus(err)
	}
	return nil
}

func (s *Server) RelayByDevServer(ctx context.Context, req *infrafleetv1.RelayByDevServerRequest) (*infrafleetv1.RelayResponse, error) {
	var params map[string]any
	if raw := req.GetParamsJson(); raw != "" {
		if err := json.Unmarshal([]byte(raw), &params); err != nil {
			return nil, apperrors.ToGRPCStatus(apperrors.New(apperrors.KindInvalidArgument, "INFRA_RELAY_BAD_PARAMS", "params_json must be a JSON object", err))
		}
	}

	result, err := s.relayByDevServer.Execute(ctx, usecase.RelayByDevServerInput{
		DevServerID: req.GetDevServerId(),
		Method:      req.GetMethod(),
		Params:      params,
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

// StreamFileChanges is BACKLOG-003's streaming counterpart to Relay/
// RelayByDevServer above — same dual connection_id/dev_server_id
// addressing, but server-streaming (grpc.ServerStreamingServer's generated
// method shape takes (req, stream); ctx comes from stream.Context(), same
// as StreamVmProvision, not a separate parameter).
func (s *Server) StreamFileChanges(req *infrafleetv1.StreamFileChangesRequest, stream infrafleetv1.InfraFleetService_StreamFileChangesServer) error {
	events, unsubscribe, err := s.streamFileChanges.Execute(stream.Context(), usecase.StreamFileChangesInput{
		ConnectionID: req.GetConnectionId(),
		DevServerID:  req.GetDevServerId(),
		Path:         req.GetPath(),
	})
	if err != nil {
		return apperrors.ToGRPCStatus(err)
	}
	defer unsubscribe()
	for event := range events {
		if err := stream.Send(toProtoFileChangeEvent(event)); err != nil {
			return err
		}
	}
	return nil
}

func toProtoFileChangeEvent(e usecase.FileChangeEvent) *infrafleetv1.FileChangeEvent {
	return &infrafleetv1.FileChangeEvent{
		Kind:            e.Kind,
		AbsolutePath:    e.Path,
		OldAbsolutePath: e.OldPath,
		IsDirectory:     e.IsDirectory,
	}
}

func (s *Server) IsDevServerConnected(ctx context.Context, req *infrafleetv1.IsDevServerConnectedRequest) (*infrafleetv1.IsDevServerConnectedResponse, error) {
	connected, err := s.isDevServerConnected.Execute(ctx, req.GetDevServerId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.IsDevServerConnectedResponse{Connected: connected}, nil
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

// BulkProvisionFleet fans usecase.BulkProvisionFleet's per-server callback
// out into one BulkProvisionFleetEvent per stream.Send — CR-FLEET-001.
// sendErr latches the first stream.Send failure so later emit() calls
// become no-ops instead of calling Send on an already-broken stream.
func (s *Server) BulkProvisionFleet(req *infrafleetv1.BulkProvisionFleetRequest, stream infrafleetv1.InfraFleetService_BulkProvisionFleetServer) error {
	servers := make([]domain.FleetSpecServer, 0, len(req.GetServers()))
	for _, sp := range req.GetServers() {
		servers = append(servers, domain.FleetSpecServer{
			Host:         sp.GetHost(),
			UserName:     sp.GetUserName(),
			VaultSSHRole: sp.GetVaultSshRole(),
			Kind:         toDomainAgentKind(sp.GetKind()),
		})
	}
	spec := usecase.FleetSpec{Servers: servers}

	// BulkProvisionFleet.Execute's emit callback fires from N concurrent
	// per-server goroutines (its whole point — bounded concurrency), so
	// sendErr and stream.Send() itself both need this mutex: gRPC streams
	// are not safe for concurrent Send calls from multiple goroutines, and
	// reading/writing a plain `var sendErr error` from N goroutines races.
	// Confirmed via `go test -race` — this was a live bug in the version
	// TASK-BE-FLEET-003 originally shipped.
	var mu sync.Mutex
	var sendErr error
	_, err := s.bulkProvisionFleet.Execute(stream.Context(), spec, int(req.GetConcurrency()), func(r usecase.BulkProvisionServerResult) {
		mu.Lock()
		defer mu.Unlock()
		if sendErr != nil {
			return // already failed once — stream may be closed, don't retry
		}
		status := infrafleetv1.BulkProvisionFleetEvent_SUCCEEDED
		if r.Status == "FAILED" {
			status = infrafleetv1.BulkProvisionFleetEvent_FAILED
		}
		sendErr = stream.Send(&infrafleetv1.BulkProvisionFleetEvent{
			Host: r.Host, Status: status, DevServerId: r.DevServerID, Error: r.Error,
		})
	})
	if err != nil {
		return apperrors.ToGRPCStatus(err)
	}
	if sendErr != nil {
		return apperrors.ToGRPCStatus(apperrors.New(apperrors.KindInternal, "INFRA_BULK_PROVISION_STREAM_SEND_FAILED", "failed to stream event", sendErr))
	}
	return nil
}

// ApplyTerraformPlan sends exactly one terminal "result" event after
// usecase.ApplyTerraformPlan.Execute returns — see infrafleet.proto's
// ApplyTerraformPlanEvent doc comment for why (no per-chunk emit callback
// exists at the usecase layer yet, a known MVP limitation, not a silent
// gap). Still server-streaming on the wire so a future streaming usecase
// signature doesn't require a breaking RPC/proto change.
func (s *Server) ApplyTerraformPlan(req *infrafleetv1.ApplyTerraformPlanRequest, stream infrafleetv1.InfraFleetService_ApplyTerraformPlanServer) error {
	result, err := s.applyTerraformPlan.Execute(stream.Context(), usecase.ApplyTerraformPlanInput{
		ControlDevServerID: req.GetControlDevServerId(),
		WorkingDir:         req.GetWorkingDir(),
		VarsFile:           req.GetVarsFile(),
	})
	if err != nil {
		return apperrors.ToGRPCStatus(err)
	}
	if err := stream.Send(&infrafleetv1.ApplyTerraformPlanEvent{
		Type:       "result",
		OutputJson: result.OutputJSON,
	}); err != nil {
		return apperrors.ToGRPCStatus(apperrors.New(apperrors.KindInternal, "INFRA_APPLY_TERRAFORM_PLAN_STREAM_SEND_FAILED", "failed to stream event", err))
	}
	return nil
}

// --- FleetDefinition CRUD (TASK-BE-FLEET-012, CR-FLEET-003) ---

func (s *Server) CreateFleetDefinition(ctx context.Context, req *infrafleetv1.CreateFleetDefinitionRequest) (*infrafleetv1.FleetDefinitionProto, error) {
	def, err := s.createFleetDefinition.Execute(ctx, usecase.CreateFleetDefinitionInput{
		Name:      req.GetName(),
		Servers:   toDomainFleetSpecServers(req.GetServers()),
		Provision: toDomainProvisionConfig(req.GetProvision()),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoFleetDefinition(def), nil
}

func (s *Server) UpdateFleetDefinition(ctx context.Context, req *infrafleetv1.UpdateFleetDefinitionRequest) (*infrafleetv1.FleetDefinitionProto, error) {
	def, err := s.updateFleetDefinition.Execute(ctx, usecase.UpdateFleetDefinitionInput{
		ID:        req.GetId(),
		Servers:   toDomainFleetSpecServers(req.GetServers()),
		Provision: toDomainProvisionConfig(req.GetProvision()),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoFleetDefinition(def), nil
}

func (s *Server) GetFleetDefinition(ctx context.Context, req *infrafleetv1.GetFleetDefinitionRequest) (*infrafleetv1.FleetDefinitionProto, error) {
	def, err := s.getFleetDefinition.Execute(ctx, req.GetId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoFleetDefinition(def), nil
}

func (s *Server) ListFleetDefinitions(ctx context.Context, req *infrafleetv1.ListFleetDefinitionsRequest) (*infrafleetv1.ListFleetDefinitionsResponse, error) {
	defs, err := s.listFleetDefinitions.Execute(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*infrafleetv1.FleetDefinitionProto, 0, len(defs))
	for _, def := range defs {
		out = append(out, toProtoFleetDefinition(def))
	}
	return &infrafleetv1.ListFleetDefinitionsResponse{Definitions: out}, nil
}

func toDomainFleetSpecServers(protoServers []*infrafleetv1.FleetSpecServerProto) []domain.FleetSpecServer {
	servers := make([]domain.FleetSpecServer, 0, len(protoServers))
	for _, sp := range protoServers {
		servers = append(servers, domain.FleetSpecServer{
			Host:         sp.GetHost(),
			UserName:     sp.GetUserName(),
			VaultSSHRole: sp.GetVaultSshRole(),
			Kind:         toDomainAgentKind(sp.GetKind()),
		})
	}
	return servers
}

func toProtoFleetSpecServers(servers []domain.FleetSpecServer) []*infrafleetv1.FleetSpecServerProto {
	out := make([]*infrafleetv1.FleetSpecServerProto, 0, len(servers))
	for _, sv := range servers {
		out = append(out, &infrafleetv1.FleetSpecServerProto{
			Host:         sv.Host,
			UserName:     sv.UserName,
			VaultSshRole: sv.VaultSSHRole,
			Kind:         toProtoAgentKind(sv.Kind),
		})
	}
	return out
}

// toDomainProvisionConfig returns nil for a nil proto message — mirrors
// domain.FleetDefinition.Provision's "nil means register-only, no infra
// provisioned" convention (see domain/fleet_definition.go's doc comment).
func toDomainProvisionConfig(p *infrafleetv1.ProvisionConfigProto) *domain.ProvisionConfig {
	if p == nil {
		return nil
	}
	return &domain.ProvisionConfig{IaC: p.GetIac(), WorkingDir: p.GetWorkingDir(), VarsFile: p.GetVarsFile()}
}

func toProtoProvisionConfig(p *domain.ProvisionConfig) *infrafleetv1.ProvisionConfigProto {
	if p == nil {
		return nil
	}
	return &infrafleetv1.ProvisionConfigProto{Iac: p.IaC, WorkingDir: p.WorkingDir, VarsFile: p.VarsFile}
}

// toProtoFleetDefinition formats CreatedAt/UpdatedAt as RFC3339 strings —
// infrafleet.proto's FleetDefinitionProto deliberately uses `string` rather
// than google.protobuf.Timestamp for these 2 fields (mirroring this file's
// existing string-timestamp fields elsewhere), a zero time.Time formats as
// "0001-01-01T00:00:00Z" which callers should treat as "unset", same as
// other zero-value timestamp fields in this service.
func toProtoFleetDefinition(def domain.FleetDefinition) *infrafleetv1.FleetDefinitionProto {
	return &infrafleetv1.FleetDefinitionProto{
		Id:        def.ID,
		Name:      def.Name,
		Version:   int32(def.Version),
		Servers:   toProtoFleetSpecServers(def.Servers),
		Provision: toProtoProvisionConfig(def.Provision),
		CreatedBy: def.CreatedBy,
		CreatedAt: def.CreatedAt.Format(time.RFC3339),
		UpdatedAt: def.UpdatedAt.Format(time.RFC3339),
	}
}

func (s *Server) ExportFleetDefinitionYaml(ctx context.Context, req *infrafleetv1.ExportFleetDefinitionYamlRequest) (*infrafleetv1.ExportFleetDefinitionYamlResponse, error) {
	yamlContent, err := s.exportFleetDefinitionYaml.Execute(ctx, req.GetId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.ExportFleetDefinitionYamlResponse{YamlContent: yamlContent}, nil
}

// DeployFleetDefinition mirrors BulkProvisionFleet's stream-event pattern
// exactly (same sendErr-latching approach) — see that handler's doc comment.
func (s *Server) DeployFleetDefinition(req *infrafleetv1.DeployFleetDefinitionRequest, stream infrafleetv1.InfraFleetService_DeployFleetDefinitionServer) error {
	// Same concurrency hazard as BulkProvisionFleet's handler above (this
	// usecase calls straight through to BulkProvisionFleet.Execute's
	// concurrent emit) — same mutex fix, see that handler's comment.
	var mu sync.Mutex
	var sendErr error
	_, err := s.deployFleetDefinition.Execute(stream.Context(), usecase.DeployFleetDefinitionInput{
		FleetDefinitionID:  req.GetFleetDefinitionId(),
		ControlDevServerID: req.GetControlDevServerId(),
	}, func(r usecase.BulkProvisionServerResult) {
		mu.Lock()
		defer mu.Unlock()
		if sendErr != nil {
			return
		}
		status := infrafleetv1.BulkProvisionFleetEvent_SUCCEEDED
		if r.Status == "FAILED" {
			status = infrafleetv1.BulkProvisionFleetEvent_FAILED
		}
		sendErr = stream.Send(&infrafleetv1.BulkProvisionFleetEvent{
			Host: r.Host, Status: status, DevServerId: r.DevServerID, Error: r.Error,
		})
	})
	if err != nil {
		return apperrors.ToGRPCStatus(err)
	}
	if sendErr != nil {
		return apperrors.ToGRPCStatus(apperrors.New(apperrors.KindInternal, "INFRA_DEPLOY_FLEET_DEFINITION_STREAM_SEND_FAILED", "failed to stream event", sendErr))
	}
	return nil
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

// toDomainAgentKind maps AGENT_KIND_UNSPECIFIED to "" (not
// domain.AgentKindDevServer) — callers that need the back-compat default
// apply it themselves (see usecase.RegisterDevServer.Execute), since a list
// filter needs to tell "unspecified = no filter" apart from an explicit
// dev-server-kind filter, which register's default-to-dev-server behavior
// would otherwise mask.
func toDomainAgentKind(k infrafleetv1.AgentKind) domain.AgentKind {
	switch k {
	case infrafleetv1.AgentKind_AGENT_KIND_DEV_SERVER:
		return domain.AgentKindDevServer
	case infrafleetv1.AgentKind_AGENT_KIND_MOBILE_EMULATOR:
		return domain.AgentKindMobileEmulator
	default:
		return ""
	}
}

func toProtoAgentKind(k domain.AgentKind) infrafleetv1.AgentKind {
	switch k {
	case domain.AgentKindDevServer:
		return infrafleetv1.AgentKind_AGENT_KIND_DEV_SERVER
	case domain.AgentKindMobileEmulator:
		return infrafleetv1.AgentKind_AGENT_KIND_MOBILE_EMULATOR
	default:
		return infrafleetv1.AgentKind_AGENT_KIND_UNSPECIFIED
	}
}

// nowUnixMs stamps DevServerAccessRequest.CreatedAtUnixMs at creation time —
// the one place in this adapter that reads wall-clock time directly (every
// other timestamp on the wire round-trips a domain.Time value instead).
func nowUnixMs() int64 {
	return time.Now().UnixMilli()
}

func toProtoDevServer(ds domain.DevServer) *infrafleetv1.DevServer {
	return &infrafleetv1.DevServer{
		Id:             ds.ID,
		TenantId:       ds.TenantID,
		Host:           ds.Host,
		Mode:           toProtoConnectionMode(ds.Mode),
		SshTargetId:    ds.SSHTargetID,
		ApprovalStatus: string(ds.Status),
		GroupId:        ds.GroupID,
		Kind:           toProtoAgentKind(ds.Kind),
		HealthStatus:   string(ds.HealthStatus),
		Platform:       ds.Platform,
		Arch:           ds.Arch,
		NodeVersion:    ds.NodeVersion,
		AgentVersion:   ds.AgentVersion,
		Tags:           ds.Tags,
	}
}

func toProtoDevServerGroup(g domain.DevServerGroup) *infrafleetv1.DevServerGroup {
	return &infrafleetv1.DevServerGroup{
		Id:            g.ID,
		TenantId:      g.TenantID,
		Name:          g.Name,
		ParentGroupId: g.ParentGroupID,
	}
}

func toDomainGranteeKind(k infrafleetv1.DevServerGroupGranteeKind) domain.GranteeKind {
	switch k {
	case infrafleetv1.DevServerGroupGranteeKind_DEV_SERVER_GROUP_GRANTEE_KIND_DEPARTMENT:
		return domain.GranteeKindDepartment
	case infrafleetv1.DevServerGroupGranteeKind_DEV_SERVER_GROUP_GRANTEE_KIND_TEAM:
		return domain.GranteeKindTeam
	default:
		return ""
	}
}

func toProtoGranteeKind(k domain.GranteeKind) infrafleetv1.DevServerGroupGranteeKind {
	switch k {
	case domain.GranteeKindDepartment:
		return infrafleetv1.DevServerGroupGranteeKind_DEV_SERVER_GROUP_GRANTEE_KIND_DEPARTMENT
	case domain.GranteeKindTeam:
		return infrafleetv1.DevServerGroupGranteeKind_DEV_SERVER_GROUP_GRANTEE_KIND_TEAM
	default:
		return infrafleetv1.DevServerGroupGranteeKind_DEV_SERVER_GROUP_GRANTEE_KIND_UNSPECIFIED
	}
}

func toProtoGrant(g domain.DevServerGroupGrant) *infrafleetv1.DevServerGroupGrant {
	return &infrafleetv1.DevServerGroupGrant{
		Id:               g.ID,
		TenantId:         g.TenantID,
		DevServerGroupId: g.DevServerGroupID,
		GranteeKind:      toProtoGranteeKind(g.GranteeKind),
		GranteeId:        g.GranteeID,
	}
}

func toDomainAccessRequestStatus(s domain.AccessRequestStatus) infrafleetv1.DevServerAccessRequestStatus {
	switch s {
	case domain.AccessRequestStatusPending:
		return infrafleetv1.DevServerAccessRequestStatus_DEV_SERVER_ACCESS_REQUEST_STATUS_PENDING
	case domain.AccessRequestStatusApproved:
		return infrafleetv1.DevServerAccessRequestStatus_DEV_SERVER_ACCESS_REQUEST_STATUS_APPROVED
	case domain.AccessRequestStatusRejected:
		return infrafleetv1.DevServerAccessRequestStatus_DEV_SERVER_ACCESS_REQUEST_STATUS_REJECTED
	default:
		return infrafleetv1.DevServerAccessRequestStatus_DEV_SERVER_ACCESS_REQUEST_STATUS_UNSPECIFIED
	}
}

func toProtoAccessRequest(r domain.DevServerAccessRequest) *infrafleetv1.DevServerAccessRequest {
	return &infrafleetv1.DevServerAccessRequest{
		Id:               r.ID,
		TenantId:         r.TenantID,
		UserId:           r.UserID,
		DevServerGroupId: r.DevServerGroupID,
		Status:           toDomainAccessRequestStatus(r.Status),
		Message:          r.Message,
		GranteeKind:      toProtoGranteeKind(r.GranteeKind),
		GranteeId:        r.GranteeID,
		CreatedAtUnixMs:  r.CreatedAtUnixMs,
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
	return &infrafleetv1.SpawnTerminalSessionResponse{Session: s.toProtoTerminalSession(session)}, nil
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
		out = append(out, s.toProtoTerminalSession(session))
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
		AgentRunning:      result.AgentRunning,
		AgentKind:         result.AgentKind,
		ReadyForInput:     result.ReadyForInput,
		LastOutputPreview: result.LastOutputPreview,
	}, nil
}

func (s *Server) GetAgentTerminalSession(ctx context.Context, req *infrafleetv1.GetAgentTerminalSessionRequest) (*infrafleetv1.GetAgentTerminalSessionResponse, error) {
	session, found, err := s.getAgentTerminalSession.Execute(ctx, req.GetWorktreeId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	resp := &infrafleetv1.GetAgentTerminalSessionResponse{Found: found}
	if found {
		resp.Session = s.toProtoTerminalSession(session)
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

// --- Mobile prompt dispatch (SOL-MB-03) ------------------------------------

// DispatchPrompt is the ONE decision point BR-MB-09/10/12 all reduce to —
// see usecase.DispatchPrompt's doc comment.
func (s *Server) DispatchPrompt(ctx context.Context, req *infrafleetv1.DispatchPromptRequest) (*infrafleetv1.DispatchPromptResponse, error) {
	result, err := s.dispatchPrompt.Execute(ctx, usecase.DispatchPromptInput{
		PtyID: req.GetPtyId(), Prompt: req.GetPrompt(), Overwrite: req.GetOverwrite(), DeviceID: req.GetDispatchedByDeviceId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.DispatchPromptResponse{
		Outcome:                     infrafleetv1.DispatchPromptResponse_Outcome(infrafleetv1.DispatchPromptResponse_Outcome_value[result.Outcome]),
		ExistingQueuedPromptPreview: result.ExistingPreview,
	}, nil
}

func (s *Server) GetQueuedPrompt(ctx context.Context, req *infrafleetv1.GetQueuedPromptRequest) (*infrafleetv1.GetQueuedPromptResponse, error) {
	has, prompt, queuedAt, err := s.getQueuedPrompt.Execute(ctx, req.GetPtyId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.GetQueuedPromptResponse{HasQueuedPrompt: has, Prompt: prompt, QueuedAtUnixMs: queuedAt}, nil
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

// AttachScreencast mirrors AttachPty's shape exactly (same tenant-extraction
// workaround, same pump-inbound/pump-outbound structure) — see AttachPty's
// doc comment for why the manual withTenantFromStreamMetadata call is
// needed here too.
func (s *Server) AttachScreencast(stream infrafleetv1.InfraFleetService_AttachScreencastServer) error {
	ctx := withTenantFromStreamMetadata(stream.Context())

	inbound := make(chan usecase.ScreencastClientMessage)
	go pumpAttachScreencastInbound(stream, inbound)

	outbound, errCh := s.attachScreencast.Execute(ctx, inbound)
	for {
		select {
		case msg, ok := <-outbound:
			if !ok {
				outbound = nil
				continue
			}
			if err := stream.Send(toProtoScreencastServerFrame(msg)); err != nil {
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
			if err := <-errCh; err != nil {
				return apperrors.ToGRPCStatus(err)
			}
			return nil
		}
	}
}

func pumpAttachScreencastInbound(stream infrafleetv1.InfraFleetService_AttachScreencastServer, inbound chan<- usecase.ScreencastClientMessage) {
	defer close(inbound)
	for {
		frame, err := stream.Recv()
		if err != nil {
			return
		}
		msg, ok := toUsecaseScreencastClientMessage(frame)
		if !ok {
			continue
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

func toUsecaseScreencastClientMessage(frame *infrafleetv1.ScreencastClientFrame) (usecase.ScreencastClientMessage, bool) {
	switch f := frame.GetFrame().(type) {
	case *infrafleetv1.ScreencastClientFrame_Start:
		start := f.Start
		params := usecase.ScreencastParams{
			WorktreeID: start.GetWorktreeId(), Page: start.GetPage(), Format: start.GetFormat(),
			Quality: start.GetQuality(), MaxWidth: start.GetMaxWidth(), MaxHeight: start.GetMaxHeight(),
			Mobile: start.GetMobile(), EveryNthFrame: start.GetEveryNthFrame(), MinFrameIntervalMs: start.GetMinFrameIntervalMs(),
		}
		if start.ViewportWidth != nil {
			v := start.GetViewportWidth()
			params.ViewportWidth = &v
		}
		if start.ViewportHeight != nil {
			v := start.GetViewportHeight()
			params.ViewportHeight = &v
		}
		if start.DeviceScaleFactor != nil {
			v := start.GetDeviceScaleFactor()
			params.DeviceScaleFactor = &v
		}
		return usecase.ScreencastClientMessage{Start: &usecase.ScreencastStartMessage{Params: params}}, true
	case *infrafleetv1.ScreencastClientFrame_Stop:
		return usecase.ScreencastClientMessage{Stop: true}, true
	default:
		return usecase.ScreencastClientMessage{}, false
	}
}

func toProtoScreencastServerFrame(ev usecase.ScreencastEvent) *infrafleetv1.ScreencastServerFrame {
	switch {
	case ev.Ready:
		return &infrafleetv1.ScreencastServerFrame{Frame: &infrafleetv1.ScreencastServerFrame_Ready{Ready: &infrafleetv1.ScreencastReady{
			SubscriptionId: ev.SubscriptionID, BrowserPageId: ev.BrowserPageID, Format: ev.Format,
		}}}
	case ev.Ended:
		return &infrafleetv1.ScreencastServerFrame{Frame: &infrafleetv1.ScreencastServerFrame_Ended{Ended: &infrafleetv1.ScreencastEnded{}}}
	case ev.ErrorMsg != "":
		return &infrafleetv1.ScreencastServerFrame{Frame: &infrafleetv1.ScreencastServerFrame_Error{Error: &infrafleetv1.ScreencastError{Message: ev.ErrorMsg}}}
	default:
		return &infrafleetv1.ScreencastServerFrame{Frame: &infrafleetv1.ScreencastServerFrame_FrameData{FrameData: &infrafleetv1.ScreencastFrame{Data: ev.Frame}}}
	}
}

// toProtoTerminalSession is a method (not a free function) because
// LastOutputPreview (TASK-MB-04-02) is read from the server's shared
// liveStates registry, keyed by PtyID — empty when no live entry exists
// (cross-pod case, or a freshly spawned session with no output yet), not an
// error.
func (s *Server) toProtoTerminalSession(session domain.TerminalSession) *infrafleetv1.TerminalSession {
	return &infrafleetv1.TerminalSession{
		PtyId:              session.PtyID,
		ConnectionId:       session.ConnectionID,
		Cwd:                session.Cwd,
		CreatedAtUnixMs:    session.CreatedAt.UnixMilli(),
		LastActiveAtUnixMs: session.LastActiveAt.UnixMilli(),
		LastOutputPreview:  usecase.LastOutputPreview(s.liveStates, session.PtyID),
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

// --- Agent sessions (TASK-AG-01..04) ---

func (s *Server) StartAgentSession(ctx context.Context, req *infrafleetv1.StartAgentSessionRequest) (*infrafleetv1.AgentSession, error) {
	session, err := s.startAgentSession.Execute(ctx, usecase.StartAgentSessionInput{
		ConnectionID: req.GetConnectionId(),
		WorktreeID:   req.GetWorktreeId(),
		UserID:       req.GetUserId(),
		Cwd:          req.GetCwd(),
		ModelID:      req.GetModelId(),
		AccountID:    req.GetAccountId(),
		TrustPreset:  req.GetTrustPreset(),
		Cols:         req.GetCols(),
		Rows:         req.GetRows(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoAgentSession(session), nil
}

func (s *Server) StopAgentSession(ctx context.Context, req *infrafleetv1.StopAgentSessionRequest) (*emptypb.Empty, error) {
	if err := s.stopAgentSession.Execute(ctx, req.GetSessionId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) KillAgentSession(ctx context.Context, req *infrafleetv1.KillAgentSessionRequest) (*emptypb.Empty, error) {
	if err := s.killAgentSession.Execute(ctx, req.GetSessionId(), req.GetSignal()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) ResumeAgentSession(ctx context.Context, req *infrafleetv1.ResumeAgentSessionRequest) (*infrafleetv1.AgentSession, error) {
	session, err := s.resumeAgentSession.Execute(ctx, usecase.ResumeAgentSessionInput{
		ConnectionID: req.GetConnectionId(),
		WorktreeID:   req.GetWorktreeId(),
		UserID:       req.GetUserId(),
		Cwd:          req.GetCwd(),
		Cols:         req.GetCols(),
		Rows:         req.GetRows(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoAgentSession(session), nil
}

func (s *Server) SwitchAgentAccount(ctx context.Context, req *infrafleetv1.SwitchAgentAccountRequest) (*infrafleetv1.AgentSession, error) {
	session, err := s.switchAgentAccount.Execute(ctx, usecase.SwitchAgentAccountInput{
		ConnectionID: req.GetConnectionId(),
		WorktreeID:   req.GetWorktreeId(),
		UserID:       req.GetUserId(),
		ProjectID:    req.GetProjectId(),
		Cwd:          req.GetCwd(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoAgentSession(session), nil
}

func toProtoAgentSession(s domain.AgentSession) *infrafleetv1.AgentSession {
	return &infrafleetv1.AgentSession{
		Id: s.ID, PtyId: s.PtyID, ConnectionId: s.ConnectionID, WorktreeId: s.WorktreeID, DevServerId: s.DevServerID,
		UserId: s.UserID, ModelId: s.ModelID, AccountId: s.AccountID, Status: string(s.Status),
		StartedAtUnixMs: s.StartedAt.UnixMilli(), LastActiveAtUnixMs: s.LastActiveAt.UnixMilli(),
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
