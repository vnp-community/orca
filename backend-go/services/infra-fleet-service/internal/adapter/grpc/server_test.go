package grpc

import (
	"context"
	"errors"
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
// nil is safe and avoids constructing all ~30 of this service's usecases
// just to contract-test three handlers. The 4 trailing nils before the
// agent-token trio are the fleet-import/bulk-provision/detect/preflight
// usecases (BL-FLEET-01..04); the final 3 nils are the persistent
// agent-token usecases (BL-AWS-03) — both untouched by these tests, which
// construct their own *Server{...} literals directly (see below) when they
// need those fields instead.
func newTestServer(getAgentTerminalSession *usecase.GetAgentTerminalSession, sendTerminalInput *usecase.SendTerminalInput, getTerminalScrollback *usecase.GetTerminalScrollback) *Server {
	return New(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, // 1-13
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, // 14-24
		nil, nil, nil, nil, nil, // listBrowserProfiles..getHostCapabilities, 25-29
		nil, nil, nil, // scrollback-snapshot usecases (SOL-TM-03), 30-32, unused by this package's tests
		getAgentTerminalSession, sendTerminalInput, getTerminalScrollback, // 33-35
		nil, nil, nil, nil, // fleet import/bulk-provision/detect/preflight usecases, 36-39, unused here
		nil, nil, nil, // persistent agent-token usecases (BL-AWS-03), 40-42, unused here
		nil,           // teardown-connection usecase (BR-SSH-13), 43, unused here
		nil, nil, nil, // port-forward CRUD usecases (SOL-SSH-04), 44-46, unused here
		nil,           // port-forward event broadcaster (TASK-SSH-04-08), 47, unused here
		nil, nil, nil, nil, nil, // agent-session usecases (TASK-AG-01..04), 48-52, unused here
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
	uc := usecase.NewSendTerminalInput(sessions, resolver, agent)
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
	uc := usecase.NewGetTerminalScrollback(sessions, resolver, agent)
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
func (f *fakeDevServerRepo) UpdateProvisionResult(ctx context.Context, tenantID, id string, status domain.DevServerStatus, info usecase.HandshakeInfo, provisionedAt time.Time) error {
	return nil
}
func (f *fakeDevServerRepo) ListAllForPolling(ctx context.Context) ([]domain.DevServer, error) {
	return nil, nil
}
func (f *fakeDevServerRepo) ListByTag(ctx context.Context, tenantID, tag string) ([]domain.DevServer, error) {
	return nil, nil
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
	if resp.GetOutcomes()[0].GetHost() != "h1.example.com" || resp.GetOutcomes()[0].GetStatus() != string(domain.DevServerStatusHealthy) {
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
	}
}
