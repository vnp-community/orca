package tools

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

type stubAgentLister struct {
	mu     sync.Mutex
	byMcp  map[string][]AgentSessionRef
	err    error
	called []string
	ident  []wscompat.Identity
}

func (s *stubAgentLister) ListMcpAgentSessions(_ context.Context, id wscompat.Identity, mcpSessionID string) ([]AgentSessionRef, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.called = append(s.called, mcpSessionID)
	s.ident = append(s.ident, id)
	return s.byMcp[mcpSessionID], s.err
}

// A crashed replica's agents live only in infra-fleet; the durable reaper must
// stop them (polite stop first, SIGKILL after the grace) and touch nothing of
// another MCP session.
func TestReapSession_StopsAgentsFoundInInfraFleetAfterAReplicaDied(t *testing.T) {
	lister := &stubAgentLister{byMcp: map[string][]AgentSessionRef{
		"gone": {{SessionID: "agent-pty-90", Status: "running"}, {SessionID: "agent-pty-91", Status: "idle"}},
	}}
	f := newPtyFixture(t, func(c *PtyToolsConfig) { c.AgentLister = lister }, nil)
	for _, id := range []string{"pty-90", "pty-91"} {
		f.fleet.ptys[id] = &fakePty{id: id, out: make(chan *infrafleetv1.PtyServerFrame, 8)}
	}

	if err := f.ex.ReapSession(context.Background(), "t1", "alice", "gone"); err != nil {
		t.Fatal(err)
	}
	f.fleet.mu.Lock()
	stops, kills := append([]string(nil), f.fleet.agentStops...), append([]string(nil), f.fleet.agentKills...)
	f.fleet.mu.Unlock()
	if !containsAll(stops, "agent-pty-90", "agent-pty-91") || !containsAll(kills, "agent-pty-90", "agent-pty-91") {
		t.Fatalf("stops=%v kills=%v", stops, kills)
	}
	if len(lister.ident) != 1 || lister.ident[0].TenantID != "t1" || lister.ident[0].UserID != "alice" {
		t.Fatalf("listing must run as the session owner: %+v", lister.ident)
	}
	if err := f.ex.ReapSession(context.Background(), "t1", "alice", "alive"); err != nil {
		t.Fatal(err)
	}
	f.fleet.mu.Lock()
	n := len(f.fleet.agentKills)
	f.fleet.mu.Unlock()
	if n != 2 {
		t.Fatalf("another mcp session's agents were touched: kills=%d", n)
	}
}

func TestReapSession_AgentListingFailureIsRetriedButTerminalsStillClosed(t *testing.T) {
	lister := &stubAgentLister{err: errors.New("infra-fleet down")}
	f := newPtyFixture(t, func(c *PtyToolsConfig) { c.AgentLister = lister }, nil)
	f.fleet.open = nil
	err := f.ex.ReapSession(context.Background(), "t1", "alice", "gone")
	if err == nil {
		t.Fatal("a failed agent listing must be returned so the event is redelivered")
	}
	f.fleet.mu.Lock()
	lists := f.fleet.listCalls
	f.fleet.mu.Unlock()
	if lists == 0 {
		t.Fatal("terminals must still be reaped when the agent listing fails")
	}
}

func TestReapSession_WithoutAgentListerKeepsOldBehaviour(t *testing.T) {
	f := newPtyFixture(t, nil, nil)
	if err := f.ex.ReapSession(context.Background(), "t1", "alice", "gone"); err != nil {
		t.Fatal(err)
	}
}
