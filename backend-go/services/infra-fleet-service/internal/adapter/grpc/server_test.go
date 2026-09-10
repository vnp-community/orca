package grpc

import (
	"context"
	"errors"
<<<<<<< HEAD
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/tenant"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

// newTestServer builds a Server with every usecase field nil except the
// three under test (CLI agent access, BUG-CLI-02) — each RPC handler this
// package's tests exercise only touches its own field, so leaving the rest
// nil is safe and avoids constructing all ~70 of this service's usecases
// just to contract-test three handlers. Positional args, not a struct
// literal — see New's own parameter list (server.go) for what each nil
// group corresponds to; this test constructs its own *Server{...} literals
// directly (see below) when it needs other fields instead.
func newTestServer(getAgentTerminalSession *usecase.GetAgentTerminalSession, sendTerminalInput *usecase.SendTerminalInput, getTerminalScrollback *usecase.GetTerminalScrollback) *Server {
	return New(
		nil, nil, nil, nil, nil, // 1-5
		nil, nil, nil, nil, nil, // 6-10
		nil, nil, nil, nil, // 11-14
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, // 15-24
		nil, nil, nil, nil, // 25-28
		nil, nil, // emulatorRelay, getHostCapabilities, 29-30
		nil, nil, nil, // scrollback-snapshot usecases (SOL-TM-03), 31-33, unused by this package's tests
		getAgentTerminalSession, sendTerminalInput, getTerminalScrollback, // 34-36
		nil, nil, nil, nil, // fleet import/bulk-provision/detect/preflight usecases, 37-40, unused here
		nil, nil, nil, // persistent agent-token usecases (BL-AWS-03), 41-43, unused here
		nil,           // teardown-connection usecase (BR-SSH-13), 44, unused here
		nil, nil, nil, // port-forward CRUD usecases (SOL-SSH-04), 45-47, unused here
		nil,                     // port-forward event broadcaster (TASK-SSH-04-08), 48, unused here
		nil, nil, nil, nil, nil, // agent-session usecases (TASK-AG-01..04), 49-53, unused here
		nil, nil, // mobile prompt-dispatch usecases (SOL-MB-03), 54-55, unused here
		nil,                     // liveStates registry (TASK-MB-02-01), 56, unused here
		nil, nil, nil, nil, nil, // dev-server access control usecases (CR-DS-006/007/008), 57-61, unused here
		nil, nil, nil, nil, // 62-65
		nil, nil, nil, // 66-68
		nil, nil, // relayByDevServer, isDevServerConnected, 69-70
		nil, nil, nil, nil, // ephemeral-VM + fleet-connectivity + file-changes usecases, 71-74, unused here
	)
}

// fakeConnectionResolver is a minimal usecase.ConnectionResolver stub —
// this package's own fake, since the real fakes in internal/usecase are
// unexported to that package.
type fakeConnectionResolver struct {
	connected  bool
	devServer  domain.DevServer
	connection domain.Connection
	err        error
}

func (f *fakeConnectionResolver) ResolveConnection(ctx context.Context, tenantID, connectionID string) (bool, domain.DevServer, domain.Connection, error) {
	return f.connected, f.devServer, f.connection, f.err
}
func (f *fakeConnectionResolver) ResolveConnectionByDevServer(ctx context.Context, tenantID, devServerID string) (bool, domain.DevServer, domain.Connection, error) {
	return f.connected, f.devServer, f.connection, f.err
}
func (f *fakeConnectionResolver) ResolveConnectionByWorktree(ctx context.Context, tenantID, worktreeID string) (bool, domain.DevServer, domain.Connection, error) {
	return f.connected, f.devServer, f.connection, f.err
}

// fakeTerminalSessionRepository is a minimal usecase.TerminalSessionRepository stub.
type fakeTerminalSessionRepository struct {
	sessions map[string]domain.TerminalSession
	listErr  error
}

func (f *fakeTerminalSessionRepository) Create(ctx context.Context, session domain.TerminalSession) (domain.TerminalSession, error) {
	return session, nil
}
func (f *fakeTerminalSessionRepository) Get(ctx context.Context, tenantID, ptyID string) (bool, domain.TerminalSession, error) {
	s, ok := f.sessions[ptyID]
	return ok, s, nil
}
func (f *fakeTerminalSessionRepository) List(ctx context.Context, tenantID, connectionID string) ([]domain.TerminalSession, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []domain.TerminalSession
	for _, s := range f.sessions {
		out = append(out, s)
	}
	return out, nil
}
func (f *fakeTerminalSessionRepository) Touch(ctx context.Context, tenantID, ptyID string, now time.Time) error {
	return nil
}
func (f *fakeTerminalSessionRepository) Close(ctx context.Context, tenantID, ptyID string, closedAt time.Time) error {
	return nil
}
func (f *fakeTerminalSessionRepository) CloseAllForConnection(ctx context.Context, tenantID, connectionID string, closedAt time.Time) error {
	return nil
}

// fakeDevServerAgentClient is a minimal usecase.DevServerAgentClient stub.
type fakeDevServerAgentClient struct {
	writePtyErr error

	streamPtyEvents chan usecase.PtyEvent
	streamPtyErr    error
}

func (f *fakeDevServerAgentClient) Exec(ctx context.Context, devServer domain.DevServer, method string, params map[string]any) (map[string]any, error) {
	return nil, nil
}
func (f *fakeDevServerAgentClient) ExecStream(ctx context.Context, devServer domain.DevServer, method string, params map[string]any) (<-chan map[string]any, func(), error) {
	ch := make(chan map[string]any)
	close(ch)
	return ch, func() {}, nil
}
func (f *fakeDevServerAgentClient) Health(ctx context.Context, devServer domain.DevServer) (bool, error) {
	return true, nil
}
func (f *fakeDevServerAgentClient) LastHandshakeInfo(devServerID string) (usecase.HandshakeInfo, bool) {
	return usecase.HandshakeInfo{}, false
}
func (f *fakeDevServerAgentClient) SpawnPty(ctx context.Context, devServer domain.DevServer, in usecase.SpawnPtyInput) (usecase.SpawnPtyResult, error) {
	return usecase.SpawnPtyResult{}, nil
}
func (f *fakeDevServerAgentClient) WritePty(ctx context.Context, devServer domain.DevServer, ptyID string, data []byte) error {
	return f.writePtyErr
}
func (f *fakeDevServerAgentClient) ResizePty(ctx context.Context, devServer domain.DevServer, ptyID string, cols, rows int32) error {
	return nil
}
func (f *fakeDevServerAgentClient) KillPty(ctx context.Context, devServer domain.DevServer, ptyID string, graceful bool) error {
	return nil
}
func (f *fakeDevServerAgentClient) SendSignal(ctx context.Context, devServer domain.DevServer, ptyID string, signal string) error {
	return nil
}
func (f *fakeDevServerAgentClient) StreamPty(ctx context.Context, devServer domain.DevServer, ptyID string) (<-chan usecase.PtyEvent, func(), error) {
	if f.streamPtyErr != nil {
		return nil, nil, f.streamPtyErr
	}
	events := f.streamPtyEvents
	if events == nil {
		events = make(chan usecase.PtyEvent)
	}
	return events, func() {}, nil
}
func (f *fakeDevServerAgentClient) AgentStatus(ctx context.Context, devServer domain.DevServer, ptyID string) (usecase.AgentStatusResult, error) {
	return usecase.AgentStatusResult{}, nil
}
func (f *fakeDevServerAgentClient) InspectProcess(ctx context.Context, devServer domain.DevServer, ptyID string) (usecase.InspectProcessResult, error) {
	return usecase.InspectProcessResult{}, nil
}
func (f *fakeDevServerAgentClient) CancelReconnect(devServerID string) {}
func (f *fakeDevServerAgentClient) DialHiddenSshTarget(ctx context.Context, devServer domain.DevServer, runtimeID string, target domain.EphemeralVmSshTarget) (string, string, error) {
	return "", "", nil
}
func (f *fakeDevServerAgentClient) SpawnAgent(ctx context.Context, devServer domain.DevServer, in usecase.SpawnAgentInput) (usecase.SpawnAgentResult, error) {
	return usecase.SpawnAgentResult{}, nil
}
func (f *fakeDevServerAgentClient) KillAgent(ctx context.Context, devServer domain.DevServer, ptyID, signal string) error {
	return nil
}
func (f *fakeDevServerAgentClient) SendAgentInput(ctx context.Context, devServer domain.DevServer, ptyID string, data []byte) error {
	return nil
}
func (f *fakeDevServerAgentClient) StreamAgentHooks(ctx context.Context, devServer domain.DevServer) (<-chan usecase.AgentHookEvent, func(), error) {
	return nil, nil, nil
}
func (f *fakeDevServerAgentClient) IsConnected(devServerID string) bool { return false }
func (f *fakeDevServerAgentClient) StreamScreencast(ctx context.Context, devServer domain.DevServer, params usecase.ScreencastParams) (<-chan usecase.ScreencastEvent, func(), error) {
	return nil, nil, nil
}
func (f *fakeDevServerAgentClient) StreamFileChanges(ctx context.Context, devServer domain.DevServer, path string) (<-chan usecase.FileChangeEvent, func(), error) {
	return nil, nil, nil
}
func (f *fakeDevServerAgentClient) StreamVmProvision(ctx context.Context, devServer domain.DevServer, params usecase.VmProvisionParams) (<-chan usecase.VmProvisionEvent, func(), error) {
	return nil, nil, nil
}
func (f *fakeDevServerAgentClient) ReadCredentialFile(ctx context.Context, devServer domain.DevServer, path string) (string, error) {
	return "", nil
}

func withTenant(ctx context.Context, tenantID string) context.Context {
	return tenant.WithTenantID(ctx, tenantID)
}

func TestServer_GetAgentTerminalSession_NotFound_ReturnsFoundFalseWithoutSession(t *testing.T) {
	resolver := &fakeConnectionResolver{connected: false}
	sessions := &fakeTerminalSessionRepository{}
	uc := usecase.NewGetAgentTerminalSession(resolver, sessions)
	s := newTestServer(uc, nil, nil)

	resp, err := s.GetAgentTerminalSession(withTenant(context.Background(), "tenant-1"), &infrafleetv1.GetAgentTerminalSessionRequest{WorktreeId: "wt-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetFound() {
		t.Error("expected Found=false")
	}
	if resp.Session != nil {
		t.Errorf("expected Session to be unset when Found=false, got %+v", resp.Session)
	}
}

func TestServer_SendTerminalInput_Success_ReturnsEmpty(t *testing.T) {
	resolver := &fakeConnectionResolver{connected: true, devServer: domain.DevServer{ID: "ds-1"}}
	sessions := &fakeTerminalSessionRepository{sessions: map[string]domain.TerminalSession{
		"pty-1": {PtyID: "pty-1", TenantID: "tenant-1", ConnectionID: "conn-1"},
	}}
	agent := &fakeDevServerAgentClient{}
	uc := usecase.NewSendTerminalInput(sessions, resolver, &fakeDevServerRepo{}, agent)
	s := newTestServer(nil, uc, nil)

	resp, err := s.SendTerminalInput(withTenant(context.Background(), "tenant-1"), &infrafleetv1.SendTerminalInputRequest{PtyId: "pty-1", Data: []byte("hi")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected a non-nil emptypb.Empty response")
	}
}

func TestServer_GetTerminalScrollback_RoundTripsTextAndTruncated(t *testing.T) {
	resolver := &fakeConnectionResolver{connected: true, devServer: domain.DevServer{ID: "ds-1"}}
	sessions := &fakeTerminalSessionRepository{sessions: map[string]domain.TerminalSession{
		"pty-1": {PtyID: "pty-1", TenantID: "tenant-1", ConnectionID: "conn-1"},
	}}
	events := make(chan usecase.PtyEvent, 1)
	events <- usecase.PtyEvent{PtyID: "pty-1", Data: []byte("scrollback text")}
	agent := &fakeDevServerAgentClient{streamPtyEvents: events}
	uc := usecase.NewGetTerminalScrollback(sessions, resolver, &fakeDevServerRepo{}, agent)
	s := newTestServer(nil, nil, uc)

	resp, err := s.GetTerminalScrollback(withTenant(context.Background(), "tenant-1"), &infrafleetv1.GetTerminalScrollbackRequest{PtyId: "pty-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetText() != "scrollback text" {
		t.Errorf("expected text=%q, got %q", "scrollback text", resp.GetText())
	}
	if resp.GetTruncated() {
		t.Error("expected truncated=false")
	}
}

// fakeSshTargetRepo is a minimal usecase.SshTargetRepository fake for this
// package's gRPC-level marshaling tests — only Upsert/GetByHostUser matter
// for ImportFleetInventory, the rest satisfy the interface unused.
type fakeSshTargetRepo struct {
	upsertErr error
	updated   bool
	targets   []domain.SshTarget
}

func (f *fakeSshTargetRepo) Create(ctx context.Context, target domain.SshTarget) (domain.SshTarget, error) {
	return target, nil
}
func (f *fakeSshTargetRepo) Get(ctx context.Context, tenantID, id string) (domain.SshTarget, error) {
	return domain.SshTarget{}, nil
}
func (f *fakeSshTargetRepo) List(ctx context.Context, tenantID string) ([]domain.SshTarget, error) {
	return f.targets, nil
}
func (f *fakeSshTargetRepo) Upsert(ctx context.Context, target domain.SshTarget) (domain.SshTarget, bool, error) {
	if f.upsertErr != nil {
		return domain.SshTarget{}, false, f.upsertErr
	}
	return target, f.updated, nil
}
func (f *fakeSshTargetRepo) GetByHostUser(ctx context.Context, tenantID, host, userName string) (domain.SshTarget, bool, error) {
	return domain.SshTarget{}, false, nil
}

func TestServer_ImportFleetInventory_RequestToResponseMarshaling(t *testing.T) {
	repo := &fakeSshTargetRepo{}
	s := &Server{importFleetInventory: usecase.NewImportFleetInventory(repo)}

	ctx := tenant.WithTenantID(context.Background(), "t1")
	resp, err := s.ImportFleetInventory(ctx, &infrafleetv1.ImportFleetInventoryRequest{
		Servers: []*infrafleetv1.FleetServerInput{
			{Host: "10.0.0.1", User: "deploy", VaultSshRole: "role-1", Project: "team-a", Tags: []string{"prod"}},
			{Host: "10.0.0.1", User: "", VaultSshRole: "role-1"}, // invalid: empty user
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetImported() != 1 || resp.GetSkipped() != 1 || len(resp.GetErrors()) != 1 {
		t.Errorf("expected imported=1 skipped=1 with 1 error, got %+v", resp)
	}
	if resp.GetErrors()[0].GetHost() != "10.0.0.1" {
		t.Errorf("expected error to identify the offending host, got %+v", resp.GetErrors()[0])
	}
}

func TestServer_ImportFleetInventory_UsecaseErrorMapsToGRPCStatus(t *testing.T) {
	s := &Server{importFleetInventory: usecase.NewImportFleetInventory(&fakeSshTargetRepo{})}

	// No tenant in context -> usecase returns apperrors.KindUnauthenticated,
	// which apperrors.ToGRPCStatus must map to a non-nil gRPC status error.
	_, err := s.ImportFleetInventory(context.Background(), &infrafleetv1.ImportFleetInventoryRequest{
		Servers: []*infrafleetv1.FleetServerInput{{Host: "10.0.0.1", User: "deploy", VaultSshRole: "role-1"}},
	})
	if err == nil {
		t.Fatal("expected an error when no tenant is present in the request context")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected a gRPC status error, got %v", err)
	}
	if st.Code() != codes.Unauthenticated {
		t.Errorf("expected codes.Unauthenticated, got %v", st.Code())
	}
}

func TestServer_ImportFleetInventory_UpsertErrorSurfacesAsSkipped(t *testing.T) {
	repo := &fakeSshTargetRepo{upsertErr: errors.New("db unavailable")}
	s := &Server{importFleetInventory: usecase.NewImportFleetInventory(repo)}

	ctx := tenant.WithTenantID(context.Background(), "t1")
	resp, err := s.ImportFleetInventory(ctx, &infrafleetv1.ImportFleetInventoryRequest{
		Servers: []*infrafleetv1.FleetServerInput{{Host: "10.0.0.1", User: "deploy", VaultSshRole: "role-1"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetSkipped() != 1 || len(resp.GetErrors()) != 1 {
		t.Errorf("expected skipped=1 with 1 error, got %+v", resp)
	}
}

// fakeDevServerRepo is a minimal usecase.DevServerRepository fake for
// BulkProvisionFleet's gRPC-level marshaling test.
type fakeDevServerRepo struct{}

func (f *fakeDevServerRepo) Register(ctx context.Context, ds domain.DevServer) (domain.DevServer, error) {
	return ds, nil
}
func (f *fakeDevServerRepo) Get(ctx context.Context, tenantID, id string) (domain.DevServer, error) {
	return domain.DevServer{}, nil
}
func (f *fakeDevServerRepo) List(ctx context.Context, tenantID string) ([]domain.DevServer, error) {
	return nil, nil
}
func (f *fakeDevServerRepo) FindBySshTarget(ctx context.Context, tenantID, sshTargetID string) (domain.DevServer, bool, error) {
	return domain.DevServer{}, false, nil
}
func (f *fakeDevServerRepo) UpdateProvisionResult(ctx context.Context, tenantID, id string, status domain.DevServerHealthStatus, info usecase.HandshakeInfo, provisionedAt time.Time) error {
	return nil
}
func (f *fakeDevServerRepo) ListAllForPolling(ctx context.Context) ([]domain.DevServer, error) {
	return nil, nil
}
func (f *fakeDevServerRepo) ListByTag(ctx context.Context, tenantID, tag string) ([]domain.DevServer, error) {
	return nil, nil
}
func (f *fakeDevServerRepo) FindByHostAndMode(ctx context.Context, tenantID, host string, mode domain.ConnectionMode) (domain.DevServer, bool, error) {
	return domain.DevServer{}, false, nil
}
func (f *fakeDevServerRepo) UpdateApprovalStatus(ctx context.Context, tenantID, devServerID string, status domain.DevServerStatus) (domain.DevServer, error) {
	return domain.DevServer{}, nil
}
func (f *fakeDevServerRepo) AssignGroup(ctx context.Context, tenantID, devServerID, groupID string) (domain.DevServer, error) {
	return domain.DevServer{}, nil
}

// fakeBulkProvisioner is a minimal usecase.Provisioner fake.
type fakeBulkProvisioner struct{}

func (f *fakeBulkProvisioner) Provision(ctx context.Context, devServer domain.DevServer) (usecase.HandshakeInfo, bool, error) {
	return usecase.HandshakeInfo{Platform: "linux"}, true, nil
}

func TestServer_BulkProvisionFleet_RequestToResponseMarshaling(t *testing.T) {
	sshRepo := &fakeSshTargetRepo{targets: []domain.SshTarget{
		{ID: "ssht-1", TenantID: "t1", Host: "h1.example.com", UserName: "deploy"},
	}}
	s := &Server{bulkProvisionFleet: usecase.NewBulkProvisionFleet(sshRepo, &fakeDevServerRepo{}, &fakeBulkProvisioner{})}

	ctx := tenant.WithTenantID(context.Background(), "t1")
	resp, err := s.BulkProvisionFleet(ctx, &infrafleetv1.BulkProvisionFleetRequest{Concurrency: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetSuccess() != 1 || len(resp.GetOutcomes()) != 1 {
		t.Errorf("expected success=1 with 1 outcome, got %+v", resp)
	}
	if resp.GetOutcomes()[0].GetHost() != "h1.example.com" || resp.GetOutcomes()[0].GetStatus() != string(domain.DevServerHealthHealthy) {
		t.Errorf("unexpected outcome: %+v", resp.GetOutcomes()[0])
	}
}

func TestServer_BulkProvisionFleet_UsecaseErrorMapsToGRPCStatus(t *testing.T) {
	s := &Server{bulkProvisionFleet: usecase.NewBulkProvisionFleet(&fakeSshTargetRepo{}, &fakeDevServerRepo{}, &fakeBulkProvisioner{})}

	_, err := s.BulkProvisionFleet(context.Background(), &infrafleetv1.BulkProvisionFleetRequest{})
	if err == nil {
		t.Fatal("expected an error when no tenant is present in the request context")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected a gRPC status error, got %v", err)
	}
	if st.Code() != codes.Unauthenticated {
		t.Errorf("expected codes.Unauthenticated, got %v", st.Code())
	}
}

// fakeDevServerAgent is a minimal usecase.DevServerAgentClient fake for
// this package's DetectDevServerAgents/CheckDevServerPreflight
// gRPC-level marshaling tests — only Exec matters for either usecase, the
// rest satisfy the interface unused.
type fakeDevServerAgent struct {
	execResult map[string]any
	execErr    error
}

func (f *fakeDevServerAgent) Exec(ctx context.Context, devServer domain.DevServer, method string, params map[string]any) (map[string]any, error) {
	if f.execErr != nil {
		return nil, f.execErr
	}
	return f.execResult, nil
}
func (f *fakeDevServerAgent) ExecStream(ctx context.Context, devServer domain.DevServer, method string, params map[string]any) (<-chan map[string]any, func(), error) {
	ch := make(chan map[string]any)
	close(ch)
	return ch, func() {}, nil
}
func (f *fakeDevServerAgent) Health(ctx context.Context, devServer domain.DevServer) (bool, error) {
	return true, nil
}
func (f *fakeDevServerAgent) LastHandshakeInfo(devServerID string) (usecase.HandshakeInfo, bool) {
	return usecase.HandshakeInfo{}, false
}
func (f *fakeDevServerAgent) SpawnPty(ctx context.Context, devServer domain.DevServer, in usecase.SpawnPtyInput) (usecase.SpawnPtyResult, error) {
	return usecase.SpawnPtyResult{}, nil
}
func (f *fakeDevServerAgent) WritePty(ctx context.Context, devServer domain.DevServer, ptyID string, data []byte) error {
	return nil
}
func (f *fakeDevServerAgent) ResizePty(ctx context.Context, devServer domain.DevServer, ptyID string, cols, rows int32) error {
	return nil
}
func (f *fakeDevServerAgent) KillPty(ctx context.Context, devServer domain.DevServer, ptyID string, graceful bool) error {
	return nil
}
func (f *fakeDevServerAgent) SendSignal(ctx context.Context, devServer domain.DevServer, ptyID string, signal string) error {
	return nil
}
func (f *fakeDevServerAgent) StreamPty(ctx context.Context, devServer domain.DevServer, ptyID string) (<-chan usecase.PtyEvent, func(), error) {
	return nil, func() {}, nil
}
func (f *fakeDevServerAgent) AgentStatus(ctx context.Context, devServer domain.DevServer, ptyID string) (usecase.AgentStatusResult, error) {
	return usecase.AgentStatusResult{}, nil
}
func (f *fakeDevServerAgent) InspectProcess(ctx context.Context, devServer domain.DevServer, ptyID string) (usecase.InspectProcessResult, error) {
	return usecase.InspectProcessResult{}, nil
}
func (f *fakeDevServerAgent) CancelReconnect(devServerID string) {}
func (f *fakeDevServerAgent) SpawnAgent(ctx context.Context, devServer domain.DevServer, in usecase.SpawnAgentInput) (usecase.SpawnAgentResult, error) {
	return usecase.SpawnAgentResult{}, nil
}
func (f *fakeDevServerAgent) KillAgent(ctx context.Context, devServer domain.DevServer, ptyID, signal string) error {
	return nil
}
func (f *fakeDevServerAgent) SendAgentInput(ctx context.Context, devServer domain.DevServer, ptyID string, data []byte) error {
	return nil
}
func (f *fakeDevServerAgent) StreamAgentHooks(ctx context.Context, devServer domain.DevServer) (<-chan usecase.AgentHookEvent, func(), error) {
	return nil, nil, nil
}
func (f *fakeDevServerAgent) IsConnected(devServerID string) bool { return false }
func (f *fakeDevServerAgent) StreamScreencast(ctx context.Context, devServer domain.DevServer, params usecase.ScreencastParams) (<-chan usecase.ScreencastEvent, func(), error) {
	return nil, nil, nil
}
func (f *fakeDevServerAgent) StreamFileChanges(ctx context.Context, devServer domain.DevServer, path string) (<-chan usecase.FileChangeEvent, func(), error) {
	return nil, nil, nil
}
func (f *fakeDevServerAgent) StreamVmProvision(ctx context.Context, devServer domain.DevServer, params usecase.VmProvisionParams) (<-chan usecase.VmProvisionEvent, func(), error) {
	return nil, nil, nil
}
func (f *fakeDevServerAgent) DialHiddenSshTarget(ctx context.Context, devServer domain.DevServer, runtimeID string, target domain.EphemeralVmSshTarget) (string, string, error) {
	return "", "", nil
}
func (f *fakeDevServerAgent) ReadCredentialFile(ctx context.Context, devServer domain.DevServer, path string) (string, error) {
	return "", nil
}

func TestServer_DetectDevServerAgents_RequestToResponseMarshaling(t *testing.T) {
	agent := &fakeDevServerAgent{execResult: map[string]any{"agents": []any{"claude"}, "platform": "linux"}}
	s := &Server{detectDevServerAgents: usecase.NewDetectDevServerAgents(&fakeDevServerRepo{}, agent)}

	ctx := tenant.WithTenantID(context.Background(), "t1")
	resp, err := s.DetectDevServerAgents(ctx, &infrafleetv1.DetectDevServerAgentsRequest{DevServerId: "ds1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetAgents()) != 1 || resp.GetAgents()[0] != "claude" || resp.GetPlatform() != "linux" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestServer_DetectDevServerAgents_NoTenantMapsToGRPCStatus(t *testing.T) {
	agent := &fakeDevServerAgent{execResult: map[string]any{}}
	s := &Server{detectDevServerAgents: usecase.NewDetectDevServerAgents(&fakeDevServerRepo{}, agent)}

	_, err := s.DetectDevServerAgents(context.Background(), &infrafleetv1.DetectDevServerAgentsRequest{DevServerId: "ds1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is present in the request context")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected a gRPC status error, got %v", err)
	}
	if st.Code() != codes.Unauthenticated {
		t.Errorf("expected codes.Unauthenticated, got %v", st.Code())
	}
}

func TestServer_CheckDevServerPreflight_RequestToResponseMarshaling(t *testing.T) {
	agent := &fakeDevServerAgent{execResult: map[string]any{
		"stdout": "GIT:git version 2.39.2\nNODE:v22.3.0\nDISK:10485760\nGH:gh version 2.40.0\nPORT:FREE\n",
	}}
	s := &Server{checkDevServerPreflight: usecase.NewCheckDevServerPreflight(&fakeDevServerRepo{}, agent)}

	ctx := tenant.WithTenantID(context.Background(), "t1")
	resp, err := s.CheckDevServerPreflight(ctx, &infrafleetv1.CheckDevServerPreflightRequest{DevServerId: "ds1", ProbePort: 3000})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.GetGit().GetMeetsMin() || !resp.GetNode().GetMeetsMin() {
		t.Errorf("expected git/node MeetsMin=true, got %+v", resp)
	}
	if !resp.GetDisk().GetMeetsMin() || resp.GetDisk().GetFreeGb() != 10 {
		t.Errorf("expected disk MeetsMin=true with 10GB free, got %+v", resp.GetDisk())
	}
	if !resp.GetPort().GetAvailable() || resp.GetPort().GetPort() != 3000 {
		t.Errorf("expected port available=true port=3000, got %+v", resp.GetPort())
	}
	if !resp.GetGh().GetInstalled() {
		t.Errorf("expected gh installed=true, got %+v", resp.GetGh())
	}
}

func TestServer_CheckDevServerPreflight_NoTenantMapsToGRPCStatus(t *testing.T) {
	agent := &fakeDevServerAgent{execResult: map[string]any{}}
	s := &Server{checkDevServerPreflight: usecase.NewCheckDevServerPreflight(&fakeDevServerRepo{}, agent)}

	_, err := s.CheckDevServerPreflight(context.Background(), &infrafleetv1.CheckDevServerPreflightRequest{DevServerId: "ds1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is present in the request context")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected a gRPC status error, got %v", err)
	}
	if st.Code() != codes.Unauthenticated {
		t.Errorf("expected codes.Unauthenticated, got %v", st.Code())
=======
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// bulkFakeSshTargetRepositoryForServerTest is an in-memory
// usecase.SshTargetRepository — this package's own copy of the same fake
// shape usecase/bulk_provision_fleet_test.go uses (can't import a _test.go
// symbol across packages). Mutex-protected: BulkProvisionFleet.Execute
// calls Create/Delete from N concurrent per-server goroutines (its bounded
// concurrency semaphore) — confirmed by `go test -race` (this was a real,
// unguarded race in an earlier version of this fake).
type bulkFakeSshTargetRepositoryForServerTest struct {
	mu         sync.Mutex
	created    []domain.SshTarget
	deletedIDs []string
	errForHost map[string]error
}

func (f *bulkFakeSshTargetRepositoryForServerTest) Create(ctx context.Context, target domain.SshTarget) (domain.SshTarget, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err, ok := f.errForHost[target.Host]; ok {
		return domain.SshTarget{}, err
	}
	f.created = append(f.created, target)
	return target, nil
}
func (f *bulkFakeSshTargetRepositoryForServerTest) Get(ctx context.Context, tenantID, id string) (domain.SshTarget, error) {
	return domain.SshTarget{}, nil
}
func (f *bulkFakeSshTargetRepositoryForServerTest) List(ctx context.Context, tenantID string) ([]domain.SshTarget, error) {
	return nil, nil
}
func (f *bulkFakeSshTargetRepositoryForServerTest) Delete(ctx context.Context, tenantID, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletedIDs = append(f.deletedIDs, id)
	return nil
}

// bulkFakeDevServerRepositoryForServerTest is an in-memory
// usecase.DevServerRepository, same rationale (and same mutex fix) as above.
type bulkFakeDevServerRepositoryForServerTest struct {
	mu         sync.Mutex
	registered []domain.DevServer
	errForHost map[string]error
}

func (f *bulkFakeDevServerRepositoryForServerTest) Register(ctx context.Context, ds domain.DevServer) (domain.DevServer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err, ok := f.errForHost[ds.Host]; ok {
		return domain.DevServer{}, err
	}
	f.registered = append(f.registered, ds)
	return ds, nil
}
func (f *bulkFakeDevServerRepositoryForServerTest) Get(ctx context.Context, tenantID, id string) (domain.DevServer, error) {
	return domain.DevServer{}, nil
}
func (f *bulkFakeDevServerRepositoryForServerTest) List(ctx context.Context, tenantID string) ([]domain.DevServer, error) {
	return nil, nil
}
func (f *bulkFakeDevServerRepositoryForServerTest) FindBySshTarget(ctx context.Context, tenantID, sshTargetID string) (domain.DevServer, bool, error) {
	return domain.DevServer{}, false, nil
}
func (f *bulkFakeDevServerRepositoryForServerTest) FindByHostAndMode(ctx context.Context, tenantID, host string, mode domain.ConnectionMode) (domain.DevServer, bool, error) {
	return domain.DevServer{}, false, nil
}
func (f *bulkFakeDevServerRepositoryForServerTest) UpdateApprovalStatus(ctx context.Context, tenantID, devServerID string, status domain.DevServerStatus) (domain.DevServer, error) {
	return domain.DevServer{}, nil
}
func (f *bulkFakeDevServerRepositoryForServerTest) AssignGroup(ctx context.Context, tenantID, devServerID, groupID string) (domain.DevServer, error) {
	return domain.DevServer{}, nil
}

// fakeBulkProvisionFleetStream implements
// infrafleetv1.InfraFleetService_BulkProvisionFleetServer (a
// grpc.ServerStreamingServer[BulkProvisionFleetEvent] alias) — records every
// Send call, optionally failing on a configured call index.
type fakeBulkProvisionFleetStream struct {
	ctx        context.Context
	sent       []*infrafleetv1.BulkProvisionFleetEvent
	failOnSend int // 1-based index of the Send call to fail; 0 = never fail
}

func (f *fakeBulkProvisionFleetStream) Send(e *infrafleetv1.BulkProvisionFleetEvent) error {
	f.sent = append(f.sent, e)
	if f.failOnSend != 0 && len(f.sent) == f.failOnSend {
		return errors.New("stream send failed")
	}
	return nil
}
func (f *fakeBulkProvisionFleetStream) Context() context.Context     { return f.ctx }
func (f *fakeBulkProvisionFleetStream) SetHeader(metadata.MD) error  { return nil }
func (f *fakeBulkProvisionFleetStream) SendHeader(metadata.MD) error { return nil }
func (f *fakeBulkProvisionFleetStream) SetTrailer(metadata.MD)       {}
func (f *fakeBulkProvisionFleetStream) SendMsg(m interface{}) error  { return nil }
func (f *fakeBulkProvisionFleetStream) RecvMsg(m interface{}) error  { return nil }

func newBulkProvisionFleetTestServer(t *testing.T, sshRepo *bulkFakeSshTargetRepositoryForServerTest, devRepo *bulkFakeDevServerRepositoryForServerTest) *Server {
	t.Helper()
	bulkProvisionFleetUC := usecase.NewBulkProvisionFleet(
		usecase.NewCreateSshTarget(sshRepo),
		usecase.NewRegisterDevServer(devRepo),
		usecase.NewDeleteSshTarget(sshRepo),
	)
	// Only bulkProvisionFleet is populated — the rest of Server's usecase
	// fields stay nil, safe since this test only calls BulkProvisionFleet.
	return New(
		nil, nil, nil, bulkProvisionFleetUC, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
}

func withTestTenant(ctx context.Context) context.Context {
	return tenant.WithTenantID(ctx, "tenant-1")
}

func TestServer_BulkProvisionFleet_StreamsEventPerServer(t *testing.T) {
	sshRepo := &bulkFakeSshTargetRepositoryForServerTest{}
	devRepo := &bulkFakeDevServerRepositoryForServerTest{}
	s := newBulkProvisionFleetTestServer(t, sshRepo, devRepo)

	stream := &fakeBulkProvisionFleetStream{ctx: withTestTenant(context.Background())}
	req := &infrafleetv1.BulkProvisionFleetRequest{
		Servers: []*infrafleetv1.FleetSpecServerProto{
			{Host: "h1", UserName: "orca", VaultSshRole: "role"},
			{Host: "h2", UserName: "orca", VaultSshRole: "role"},
		},
	}

	if err := s.BulkProvisionFleet(req, stream); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stream.sent) != 2 {
		t.Fatalf("expected 2 events sent, got %d", len(stream.sent))
	}
	for _, e := range stream.sent {
		if e.GetStatus() != infrafleetv1.BulkProvisionFleetEvent_SUCCEEDED {
			t.Errorf("host %q: expected SUCCEEDED, got %v", e.GetHost(), e.GetStatus())
		}
	}
}

func TestServer_BulkProvisionFleet_MapsFailedStatusCorrectly(t *testing.T) {
	sshRepo := &bulkFakeSshTargetRepositoryForServerTest{}
	devRepo := &bulkFakeDevServerRepositoryForServerTest{errForHost: map[string]error{"h1": errors.New("boom")}}
	s := newBulkProvisionFleetTestServer(t, sshRepo, devRepo)

	stream := &fakeBulkProvisionFleetStream{ctx: withTestTenant(context.Background())}
	req := &infrafleetv1.BulkProvisionFleetRequest{
		Servers: []*infrafleetv1.FleetSpecServerProto{{Host: "h1", UserName: "orca", VaultSshRole: "role"}},
	}

	if err := s.BulkProvisionFleet(req, stream); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stream.sent) != 1 {
		t.Fatalf("expected 1 event sent, got %d", len(stream.sent))
	}
	got := stream.sent[0]
	if got.GetStatus() != infrafleetv1.BulkProvisionFleetEvent_FAILED {
		t.Errorf("expected FAILED, got %v", got.GetStatus())
	}
	if got.GetError() == "" {
		t.Error("expected a non-empty Error field")
	}
}

// TestServer_BulkProvisionFleet_MissingTenant_AllServersFailedNoTopLevelError
// documents actual usecase.BulkProvisionFleet.Execute behavior: it NEVER
// returns a non-nil top-level error (see bulk_provision_fleet.go's Execute —
// always `return result, nil`) — a per-server failure (like a missing
// tenant) surfaces as a FAILED event, not a returned error. The task doc's
// original TestServer_BulkProvisionFleet_UsecaseErrorReturnsGRPCStatus
// assumed Execute could fail generally; it cannot with the real
// implementation, so this test exercises the actually-reachable behavior
// instead, and TestServer_BulkProvisionFleet_StreamSendFails_ReturnsGRPCStatus
// below covers the handler's other general-error branch that IS reachable
// (stream.Send failing).
func TestServer_BulkProvisionFleet_MissingTenant_AllServersFailedNoTopLevelError(t *testing.T) {
	s := newBulkProvisionFleetTestServer(t, &bulkFakeSshTargetRepositoryForServerTest{}, &bulkFakeDevServerRepositoryForServerTest{})

	stream := &fakeBulkProvisionFleetStream{ctx: context.Background()} // no tenant in context
	req := &infrafleetv1.BulkProvisionFleetRequest{
		Servers: []*infrafleetv1.FleetSpecServerProto{{Host: "h1", UserName: "orca", VaultSshRole: "role"}},
	}

	err := s.BulkProvisionFleet(req, stream)
	if err != nil {
		t.Fatalf("expected no top-level RPC error (failure surfaces per-server), got: %v", err)
	}
	if len(stream.sent) != 1 || stream.sent[0].GetStatus() != infrafleetv1.BulkProvisionFleetEvent_FAILED {
		t.Fatalf("expected 1 FAILED event, got %+v", stream.sent)
	}
}

// TestServer_BulkProvisionFleet_StreamSendFails_ReturnsGRPCStatus covers the
// handler's general (non-per-server) error path — a stream.Send failure
// (e.g. the client disconnected mid-stream) is wrapped via apperrors and
// returned as a gRPC status, the only way BulkProvisionFleet's handler can
// currently fail generally given Execute never returns a non-nil error.
func TestServer_BulkProvisionFleet_StreamSendFails_ReturnsGRPCStatus(t *testing.T) {
	s := newBulkProvisionFleetTestServer(t, &bulkFakeSshTargetRepositoryForServerTest{}, &bulkFakeDevServerRepositoryForServerTest{})

	stream := &fakeBulkProvisionFleetStream{ctx: withTestTenant(context.Background()), failOnSend: 1}
	req := &infrafleetv1.BulkProvisionFleetRequest{
		Servers: []*infrafleetv1.FleetSpecServerProto{{Host: "h1", UserName: "orca", VaultSshRole: "role"}},
	}

	err := s.BulkProvisionFleet(req, stream)
	if err == nil {
		t.Fatal("expected a gRPC status error when stream.Send fails")
	}
}

// --- ApplyTerraformPlan (TASK-BE-FLEET-008) ---

// fakeApplyTerraformPlanStream implements
// infrafleetv1.InfraFleetService_ApplyTerraformPlanServer.
type fakeApplyTerraformPlanStream struct {
	ctx     context.Context
	sent    []*infrafleetv1.ApplyTerraformPlanEvent
	sendErr error
}

func (f *fakeApplyTerraformPlanStream) Send(e *infrafleetv1.ApplyTerraformPlanEvent) error {
	f.sent = append(f.sent, e)
	return f.sendErr
}
func (f *fakeApplyTerraformPlanStream) Context() context.Context     { return f.ctx }
func (f *fakeApplyTerraformPlanStream) SetHeader(metadata.MD) error  { return nil }
func (f *fakeApplyTerraformPlanStream) SendHeader(metadata.MD) error { return nil }
func (f *fakeApplyTerraformPlanStream) SetTrailer(metadata.MD)       {}
func (f *fakeApplyTerraformPlanStream) SendMsg(m interface{}) error  { return nil }
func (f *fakeApplyTerraformPlanStream) RecvMsg(m interface{}) error  { return nil }

// fakeTerraformRunnerForServerTest is an in-memory usecase.TerraformRunner.
type fakeTerraformRunnerForServerTest struct {
	outputJSON string
	applyErr   error
}

func (f *fakeTerraformRunnerForServerTest) Apply(ctx context.Context, controlDevServer domain.DevServer, workingDir, varsFile string) (string, error) {
	if f.applyErr != nil {
		return "", f.applyErr
	}
	return f.outputJSON, nil
}

func newApplyTerraformPlanTestServer(t *testing.T, devRepo *bulkFakeDevServerRepositoryForServerTest, runner *fakeTerraformRunnerForServerTest) *Server {
	t.Helper()
	applyTerraformPlanUC := usecase.NewApplyTerraformPlan(devRepo, runner)
	return New(
		nil, nil, nil, nil, applyTerraformPlanUC, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
}

func TestServer_ApplyTerraformPlan_SendsResultEvent(t *testing.T) {
	devRepo := &bulkFakeDevServerRepositoryForServerTest{}
	// Get() on this fake always returns a zero-value DevServer with no
	// error, so ControlDevServerID resolution always "succeeds" here.
	runner := &fakeTerraformRunnerForServerTest{outputJSON: `{"instance_hosts":{"value":["h1"]}}`}
	s := newApplyTerraformPlanTestServer(t, devRepo, runner)

	stream := &fakeApplyTerraformPlanStream{ctx: withTestTenant(context.Background())}
	req := &infrafleetv1.ApplyTerraformPlanRequest{ControlDevServerId: "d1", WorkingDir: "/infra"}

	if err := s.ApplyTerraformPlan(req, stream); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stream.sent) != 1 {
		t.Fatalf("expected 1 event sent, got %d", len(stream.sent))
	}
	got := stream.sent[0]
	if got.GetType() != "result" || got.GetOutputJson() != `{"instance_hosts":{"value":["h1"]}}` {
		t.Errorf("unexpected event: %+v", got)
	}
}

func TestServer_ApplyTerraformPlan_UsecaseErrorReturnsGRPCStatus(t *testing.T) {
	devRepo := &bulkFakeDevServerRepositoryForServerTest{}
	runner := &fakeTerraformRunnerForServerTest{applyErr: errors.New("terraform apply failed")}
	s := newApplyTerraformPlanTestServer(t, devRepo, runner)

	stream := &fakeApplyTerraformPlanStream{ctx: withTestTenant(context.Background())}
	req := &infrafleetv1.ApplyTerraformPlanRequest{ControlDevServerId: "d1", WorkingDir: "/infra"}

	err := s.ApplyTerraformPlan(req, stream)
	if err == nil {
		t.Fatal("expected a gRPC status error when the runner fails")
	}
	if len(stream.sent) != 0 {
		t.Errorf("expected no events sent, got %d", len(stream.sent))
	}
}

// --- FleetDefinition CRUD (TASK-BE-FLEET-012) ---

func newFleetDefinitionTestServer(t *testing.T, repo *fakeFleetDefinitionRepositoryForServerTest) *Server {
	t.Helper()
	return New(
		nil, nil, nil, nil, nil,
		usecase.NewCreateFleetDefinition(repo),
		usecase.NewUpdateFleetDefinition(repo),
		usecase.NewGetFleetDefinition(repo),
		usecase.NewListFleetDefinitions(repo),
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
}

// fakeFleetDefinitionRepositoryForServerTest is an in-memory
// usecase.FleetDefinitionRepository — this package's own copy (can't reuse
// usecase's _test.go fake across packages).
type fakeFleetDefinitionRepositoryForServerTest struct {
	byID      map[string]domain.FleetDefinition
	createErr error
	updateErr error
}

func fleetDefKeyForServerTest(tenantID, id string) string { return tenantID + "/" + id }

func (f *fakeFleetDefinitionRepositoryForServerTest) Create(ctx context.Context, def domain.FleetDefinition) (domain.FleetDefinition, error) {
	if f.createErr != nil {
		return domain.FleetDefinition{}, f.createErr
	}
	if f.byID == nil {
		f.byID = map[string]domain.FleetDefinition{}
	}
	f.byID[fleetDefKeyForServerTest(def.TenantID, def.ID)] = def
	return def, nil
}
func (f *fakeFleetDefinitionRepositoryForServerTest) Update(ctx context.Context, def domain.FleetDefinition) (domain.FleetDefinition, error) {
	if f.updateErr != nil {
		return domain.FleetDefinition{}, f.updateErr
	}
	f.byID[fleetDefKeyForServerTest(def.TenantID, def.ID)] = def
	return def, nil
}
func (f *fakeFleetDefinitionRepositoryForServerTest) Get(ctx context.Context, tenantID, id string) (domain.FleetDefinition, error) {
	def, ok := f.byID[fleetDefKeyForServerTest(tenantID, id)]
	if !ok {
		return domain.FleetDefinition{}, domain.ErrFleetDefinitionNotFound
	}
	return def, nil
}
func (f *fakeFleetDefinitionRepositoryForServerTest) List(ctx context.Context, tenantID string) ([]domain.FleetDefinition, error) {
	var out []domain.FleetDefinition
	for _, def := range f.byID {
		if def.TenantID == tenantID {
			out = append(out, def)
		}
	}
	return out, nil
}

func TestServer_CreateFleetDefinition_ReturnsFleetDefinitionProto(t *testing.T) {
	repo := &fakeFleetDefinitionRepositoryForServerTest{}
	s := newFleetDefinitionTestServer(t, repo)

	ctx := withTestTenant(context.Background())
	ctx = tenant.WithUserID(ctx, "user-1")
	req := &infrafleetv1.CreateFleetDefinitionRequest{
		Name:      "fleet-1",
		Servers:   []*infrafleetv1.FleetSpecServerProto{{Host: "h1", UserName: "orca", VaultSshRole: "role"}},
		Provision: &infrafleetv1.ProvisionConfigProto{Iac: "terraform", WorkingDir: "/infra"},
	}

	got, err := s.CreateFleetDefinition(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetName() != "fleet-1" || got.GetVersion() != 1 {
		t.Errorf("unexpected response: %+v", got)
	}
	if len(got.GetServers()) != 1 || got.GetServers()[0].GetHost() != "h1" {
		t.Errorf("unexpected servers: %+v", got.GetServers())
	}
	if got.GetProvision().GetWorkingDir() != "/infra" {
		t.Errorf("unexpected provision: %+v", got.GetProvision())
	}
}

func TestServer_GetFleetDefinition_UsecaseErrorReturnsGRPCStatus(t *testing.T) {
	repo := &fakeFleetDefinitionRepositoryForServerTest{}
	s := newFleetDefinitionTestServer(t, repo)

	ctx := withTestTenant(context.Background())
	_, err := s.GetFleetDefinition(ctx, &infrafleetv1.GetFleetDefinitionRequest{Id: "unknown"})
	if err == nil {
		t.Fatal("expected a gRPC status error for an unknown fleet definition")
	}
}

func TestServer_ListFleetDefinitions_ReturnsAllForTenant(t *testing.T) {
	repo := &fakeFleetDefinitionRepositoryForServerTest{byID: map[string]domain.FleetDefinition{
		fleetDefKeyForServerTest("tenant-1", "d1"): {ID: "d1", TenantID: "tenant-1", Name: "fleet-1"},
	}}
	s := newFleetDefinitionTestServer(t, repo)

	ctx := withTestTenant(context.Background())
	got, err := s.ListFleetDefinitions(ctx, &infrafleetv1.ListFleetDefinitionsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.GetDefinitions()) != 1 || got.GetDefinitions()[0].GetName() != "fleet-1" {
		t.Errorf("unexpected response: %+v", got)
	}
}

func TestServer_UpdateFleetDefinition_IncrementsVersion(t *testing.T) {
	repo := &fakeFleetDefinitionRepositoryForServerTest{byID: map[string]domain.FleetDefinition{
		fleetDefKeyForServerTest("tenant-1", "d1"): {
			ID: "d1", TenantID: "tenant-1", Name: "fleet-1", Version: 1,
			Servers: []domain.FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role"}},
		},
	}}
	s := newFleetDefinitionTestServer(t, repo)

	ctx := withTestTenant(context.Background())
	req := &infrafleetv1.UpdateFleetDefinitionRequest{
		Id:      "d1",
		Servers: []*infrafleetv1.FleetSpecServerProto{{Host: "h2", UserName: "orca", VaultSshRole: "role"}},
	}
	got, err := s.UpdateFleetDefinition(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetVersion() != 2 {
		t.Errorf("expected version 2, got %d", got.GetVersion())
	}
}

// --- ExportFleetDefinitionYaml (TASK-BE-FLEET-013) ---

func newExportFleetDefinitionYamlTestServer(t *testing.T, repo *fakeFleetDefinitionRepositoryForServerTest) *Server {
	t.Helper()
	return New(
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
		usecase.NewExportFleetDefinitionYaml(repo),
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
}

func TestServer_ExportFleetDefinitionYaml_ReturnsYamlContent(t *testing.T) {
	repo := &fakeFleetDefinitionRepositoryForServerTest{byID: map[string]domain.FleetDefinition{
		fleetDefKeyForServerTest("tenant-1", "d1"): {
			ID: "d1", TenantID: "tenant-1", Name: "fleet-1",
			Servers: []domain.FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role"}},
		},
	}}
	s := newExportFleetDefinitionYamlTestServer(t, repo)

	ctx := withTestTenant(context.Background())
	got, err := s.ExportFleetDefinitionYaml(ctx, &infrafleetv1.ExportFleetDefinitionYamlRequest{Id: "d1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetYamlContent() == "" || !strings.Contains(got.GetYamlContent(), "host: h1") {
		t.Errorf("unexpected yaml content: %q", got.GetYamlContent())
	}
}

func TestServer_ExportFleetDefinitionYaml_NotFoundReturnsGRPCStatus(t *testing.T) {
	s := newExportFleetDefinitionYamlTestServer(t, &fakeFleetDefinitionRepositoryForServerTest{})
	ctx := withTestTenant(context.Background())
	_, err := s.ExportFleetDefinitionYaml(ctx, &infrafleetv1.ExportFleetDefinitionYamlRequest{Id: "unknown"})
	if err == nil {
		t.Fatal("expected a gRPC status error for an unknown fleet definition")
	}
}

// --- DeployFleetDefinition (TASK-BE-FLEET-014) ---

func newDeployFleetDefinitionTestServer(
	t *testing.T,
	fleetRepo *fakeFleetDefinitionRepositoryForServerTest,
	controlDevRepo *bulkFakeDevServerRepositoryForServerTest,
	runner *fakeTerraformRunnerForServerTest,
	sshRepo *bulkFakeSshTargetRepositoryForServerTest,
	devRepo *bulkFakeDevServerRepositoryForServerTest,
) *Server {
	t.Helper()
	applyTerraformPlan := usecase.NewApplyTerraformPlan(controlDevRepo, runner)
	bulkProvisionFleet := usecase.NewBulkProvisionFleet(
		usecase.NewCreateSshTarget(sshRepo), usecase.NewRegisterDevServer(devRepo), usecase.NewDeleteSshTarget(sshRepo),
	)
	deployFleetDefinition := usecase.NewDeployFleetDefinition(fleetRepo, applyTerraformPlan, bulkProvisionFleet)
	return New(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		deployFleetDefinition,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
}

func TestServer_DeployFleetDefinition_ProvisionNil_StreamsBulkProvisionFleetEvent(t *testing.T) {
	fleetRepo := &fakeFleetDefinitionRepositoryForServerTest{byID: map[string]domain.FleetDefinition{
		fleetDefKeyForServerTest("tenant-1", "d1"): {
			ID: "d1", TenantID: "tenant-1", Name: "fleet-1",
			Servers: []domain.FleetSpecServer{{Host: "h1", UserName: "orca", VaultSSHRole: "role"}},
		},
	}}
	s := newDeployFleetDefinitionTestServer(t, fleetRepo, &bulkFakeDevServerRepositoryForServerTest{}, &fakeTerraformRunnerForServerTest{}, &bulkFakeSshTargetRepositoryForServerTest{}, &bulkFakeDevServerRepositoryForServerTest{})

	stream := &fakeBulkProvisionFleetStream{ctx: withTestTenant(context.Background())}
	req := &infrafleetv1.DeployFleetDefinitionRequest{FleetDefinitionId: "d1"}

	if err := s.DeployFleetDefinition(req, stream); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stream.sent) != 1 || stream.sent[0].GetHost() != "h1" || stream.sent[0].GetStatus() != infrafleetv1.BulkProvisionFleetEvent_SUCCEEDED {
		t.Errorf("unexpected events: %+v", stream.sent)
	}
}

func TestServer_DeployFleetDefinition_UsecaseErrorReturnsGRPCStatus(t *testing.T) {
	s := newDeployFleetDefinitionTestServer(t, &fakeFleetDefinitionRepositoryForServerTest{}, &bulkFakeDevServerRepositoryForServerTest{}, &fakeTerraformRunnerForServerTest{}, &bulkFakeSshTargetRepositoryForServerTest{}, &bulkFakeDevServerRepositoryForServerTest{})

	stream := &fakeBulkProvisionFleetStream{ctx: withTestTenant(context.Background())}
	req := &infrafleetv1.DeployFleetDefinitionRequest{FleetDefinitionId: "unknown"}

	err := s.DeployFleetDefinition(req, stream)
	if err == nil {
		t.Fatal("expected a gRPC status error for an unknown fleet definition")
	}
	if len(stream.sent) != 0 {
		t.Errorf("expected no events sent, got %d", len(stream.sent))
>>>>>>> feat/team-rbac-implementation
	}
}
