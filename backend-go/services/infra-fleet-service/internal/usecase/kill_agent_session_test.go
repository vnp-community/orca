package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// fakeWriteActivityChecker implements WriteActivityChecker for tests.
type fakeWriteActivityChecker struct {
	busy bool
	err  error
}

func (f *fakeWriteActivityChecker) HasInFlightWrite(ctx context.Context, worktreeID string) (bool, error) {
	return f.busy, f.err
}

func newAgentSessionFixture() (*fakeConnectionResolver, *fakeAgentSessionRepository, domain.DevServer) {
	ds, _ := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelayWebSocket, "")
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	sessions := &fakeAgentSessionRepository{byID: map[string]domain.AgentSession{
		"sess-1": {ID: "sess-1", TenantID: "tenant-1", PtyID: "agent-pty-1", ConnectionID: "conn-1", WorktreeID: "wt-1", StartedAt: time.Now(), LastActiveAt: time.Now()},
	}}
	return resolver, sessions, ds
}

func TestKillAgentSession_MarksStoppedEvenWhenAgentCallFails(t *testing.T) {
	resolver, sessions, _ := newAgentSessionFixture()
	agent := &fakeDevServerAgentClient{killAgentErr: errors.New("devserveragent: not connected")}
	uc := NewKillAgentSession(sessions, resolver, agent, nil)

	ctx := withTenant(context.Background(), "tenant-1")
	err := uc.Execute(ctx, "sess-1", "SIGKILL")
	if err == nil {
		t.Fatal("expected the agent's error to propagate")
	}
	if len(sessions.markStoppedCalls) != 1 {
		t.Fatalf("expected the session to still be marked stopped, got calls: %v", sessions.markStoppedCalls)
	}
}

func TestKillAgentSession_WriteActivityBusy_BlocksKill(t *testing.T) {
	resolver, sessions, _ := newAgentSessionFixture()
	agent := &fakeDevServerAgentClient{}
	uc := NewKillAgentSession(sessions, resolver, agent, &fakeWriteActivityChecker{busy: true})

	ctx := withTenant(context.Background(), "tenant-1")
	err := uc.Execute(ctx, "sess-1", "SIGKILL")
	if err == nil {
		t.Fatal("expected an error when a write is in flight")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "INFRA_AGENT_KILL_BLOCKED_FILE_WRITE_IN_PROGRESS" {
		t.Fatalf("expected INFRA_AGENT_KILL_BLOCKED_FILE_WRITE_IN_PROGRESS, got %v", err)
	}
	if len(agent.killAgentCalls) != 0 {
		t.Error("expected KillAgent to never be called while busy")
	}
}

func TestKillAgentSession_WriteActivityCheckerErrors_FailsOpen(t *testing.T) {
	resolver, sessions, _ := newAgentSessionFixture()
	agent := &fakeDevServerAgentClient{}
	uc := NewKillAgentSession(sessions, resolver, agent, &fakeWriteActivityChecker{err: errors.New("checker unreachable")})

	ctx := withTenant(context.Background(), "tenant-1")
	if err := uc.Execute(ctx, "sess-1", "SIGKILL"); err != nil {
		t.Fatalf("expected the kill to proceed (fail open) when the checker errors, got: %v", err)
	}
	if len(agent.killAgentCalls) != 1 {
		t.Error("expected KillAgent to be called despite the checker error")
	}
}

func TestKillAgentSession_NilWriteActivityChecker_KillProceeds(t *testing.T) {
	resolver, sessions, _ := newAgentSessionFixture()
	agent := &fakeDevServerAgentClient{}
	uc := NewKillAgentSession(sessions, resolver, agent, nil)

	ctx := withTenant(context.Background(), "tenant-1")
	if err := uc.Execute(ctx, "sess-1", "SIGKILL"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agent.killAgentCalls) != 1 || agent.killAgentCalls[0] != "agent-pty-1:SIGKILL" {
		t.Errorf("expected exactly one KillAgent call with SIGKILL, got %v", agent.killAgentCalls)
	}
}
