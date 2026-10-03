package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// fakeTerminalCloser mirrors the real stores: only the NULL -> closed
// transition enqueues, a repeat close does not.
type fakeTerminalCloser struct {
	mu       sync.Mutex
	sessions *fakeTerminalSessionRepository
	events   []domain.OutboxEvent
}

func (f *fakeTerminalCloser) CloseWithEvent(_ context.Context, tenantID, ptyID string, closedAt time.Time, event domain.OutboxEvent) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.sessions.byPtyID[ptyID]
	if !ok || s.TenantID != tenantID {
		return false, context.Canceled
	}
	if s.ClosedAt != nil {
		return false, nil
	}
	t := closedAt
	s.ClosedAt = &t
	f.sessions.byPtyID[ptyID] = s
	f.events = append(f.events, event)
	return true, nil
}

func newKillWithEvents(t *testing.T, s domain.TerminalSession) (*KillTerminalSession, *fakeTerminalCloser, *fakeDevServerAgentClient) {
	t.Helper()
	ds, err := domain.NewDevServer("ds-1", "tenant-1", "10.0.0.5", domain.ConnectionModeDirectWebSocket, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	sessions := &fakeTerminalSessionRepository{byPtyID: map[string]domain.TerminalSession{s.PtyID: s}}
	resolver := &fakeConnectionResolver{
		byConnectionID: map[string]domain.DevServer{"conn-1": ds},
		connByID:       map[string]domain.Connection{"conn-1": {ID: "conn-1", TenantID: "tenant-1", Status: domain.ConnectionStatusEstablished}},
	}
	agent := &fakeDevServerAgentClient{}
	closer := &fakeTerminalCloser{sessions: sessions}
	uc := NewKillTerminalSession(sessions, resolver, &fakeDevServerRepository{byID: map[string]domain.DevServer{"ds-1": ds}}, agent).WithClosedEvents(closer)
	return uc, closer, agent
}

func mcpSession() domain.TerminalSession {
	return domain.TerminalSession{
		PtyID: "pty-1", TenantID: "tenant-1", ConnectionID: "conn-1", Cwd: "/secret/cwd", CreatedByUserID: "user-1",
		Origin: &domain.SessionOrigin{Type: "mcp", ClientName: "Claude Code", MCPSessionID: "mcp-1", UserID: "user-1"},
	}
}

func TestKillTerminalSession_EnqueuesClosedEventOnceOnTransition(t *testing.T) {
	uc, closer, agent := newKillWithEvents(t, mcpSession())
	ctx := withTenant(context.Background(), "tenant-1")

	if err := uc.ExecuteWithInput(ctx, KillTerminalSessionInput{PtyID: "pty-1", Reason: "idle", Actor: "user-9"}); err != nil {
		t.Fatal(err)
	}
	if err := uc.ExecuteWithInput(ctx, KillTerminalSessionInput{PtyID: "pty-1", Reason: "idle"}); err != nil {
		t.Fatal(err)
	}
	if len(closer.events) != 1 {
		t.Fatalf("want exactly 1 event across a repeat close, got %d", len(closer.events))
	}
	if len(agent.killPtyCalls) != 2 {
		t.Fatalf("agent teardown still runs on a repeat close, got %d", len(agent.killPtyCalls))
	}
	ev := closer.events[0]
	if ev.Subject != "orca.infrafleet.terminal.closed" || ev.TenantID != "tenant-1" || ev.ID != domain.TerminalClosedEventID("tenant-1", "pty-1") {
		t.Fatalf("unexpected envelope: %+v", ev)
	}
	var got map[string]any
	if err := json.Unmarshal(ev.PayloadJSON, &got); err != nil {
		t.Fatal(err)
	}
	origin, _ := got["origin"].(map[string]any)
	if got["reason"] != "idle" || got["pty_id"] != "pty-1" || got["user_id"] != "user-1" || got["actor"] != "user-9" ||
		origin["type"] != "mcp" || origin["client_name"] != "Claude Code" || origin["mcp_session_id"] != "mcp-1" {
		t.Fatalf("unexpected payload: %s", ev.PayloadJSON)
	}
	if strings.Contains(string(ev.PayloadJSON), "secret") || strings.Contains(string(ev.PayloadJSON), "cwd") {
		t.Fatalf("payload must not carry cwd/command/output: %s", ev.PayloadJSON)
	}
}

func TestKillTerminalSession_UnknownReasonIsUserAndUIOriginOmitted(t *testing.T) {
	s := mcpSession()
	s.Origin = nil
	uc, closer, _ := newKillWithEvents(t, s)
	if err := uc.ExecuteWithInput(withTenant(context.Background(), "tenant-1"), KillTerminalSessionInput{PtyID: "pty-1", Reason: "rm -rf"}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	_ = json.Unmarshal(closer.events[0].PayloadJSON, &got)
	if got["reason"] != "user" || got["origin"] != nil || got["user_id"] != "user-1" {
		t.Fatalf("unexpected payload: %s", closer.events[0].PayloadJSON)
	}
}

func TestTerminalClosedEventID_IsDeterministicPerTenantAndPty(t *testing.T) {
	a := domain.TerminalClosedEventID("t1", "p1")
	if a != domain.TerminalClosedEventID("t1", "p1") || a == domain.TerminalClosedEventID("t2", "p1") || a == domain.TerminalClosedEventID("t1", "p2") {
		t.Fatal("event id must be stable and scoped by tenant and pty")
	}
}
