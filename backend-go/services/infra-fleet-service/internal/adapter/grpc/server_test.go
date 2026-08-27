package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

// newTestServer builds a Server with every usecase field nil except the
// three under test (CLI agent access, BUG-CLI-02) — each RPC handler this
// package's tests exercise only touches its own field, so leaving the rest
// nil is safe and avoids constructing all ~30 of this service's usecases
// just to contract-test three handlers.
func newTestServer(getAgentTerminalSession *usecase.GetAgentTerminalSession, sendTerminalInput *usecase.SendTerminalInput, getTerminalScrollback *usecase.GetTerminalScrollback) *Server {
	return New(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil, nil, nil, // scrollback-snapshot usecases (SOL-TM-03), unused by this package's tests
		getAgentTerminalSession, sendTerminalInput, getTerminalScrollback,
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
func (f *fakeDevServerAgentClient) Health(ctx context.Context, devServer domain.DevServer) (bool, error) {
	return true, nil
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
