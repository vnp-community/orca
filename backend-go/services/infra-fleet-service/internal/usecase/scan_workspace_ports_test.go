package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// fakeDevServerAgentClient is an in-memory DevServerAgentClient — stands in
// for adapter/devserveragent's real stub in these usecase-layer tests, which
// only exercise ScanWorkspacePorts' dispatch logic, not the wire protocol.
type fakeDevServerAgentClient struct {
	execResult map[string]any
	execErr    error
	execCalls  []string // methods called with, for assertions

	// execCalled/lastMethod are a simpler single-call view of execCalls,
	// used by kill_workspace_port_test.go/establish_connection_test.go.
	execCalled bool
	lastMethod string

	// healthy/healthErr drive Health's fake answer — used by
	// establish_connection_test.go.
	healthy   bool
	healthErr error
}

func (f *fakeDevServerAgentClient) Exec(ctx context.Context, devServer domain.DevServer, method string, params map[string]any) (map[string]any, error) {
	f.execCalls = append(f.execCalls, method)
	f.execCalled = true
	f.lastMethod = method
	if f.execErr != nil {
		return nil, f.execErr
	}
	return f.execResult, nil
}

func (f *fakeDevServerAgentClient) Health(ctx context.Context, devServer domain.DevServer) (bool, error) {
	if f.healthErr != nil {
		return false, f.healthErr
	}
	return f.healthy, nil
}

func TestScanWorkspacePorts_RequiresTenantContext(t *testing.T) {
	uc := NewScanWorkspacePorts(&fakeConnectionResolver{}, &fakeDevServerAgentClient{})
	_, err := uc.Execute(context.Background(), ScanWorkspacePortsInput{})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestScanWorkspacePorts_NoConnectionID_ReturnsEmptyWithoutRelaying(t *testing.T) {
	resolver := &fakeConnectionResolver{}
	agent := &fakeDevServerAgentClient{}
	uc := NewScanWorkspacePorts(resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	ports, err := uc.Execute(ctx, ScanWorkspacePortsInput{WorktreeID: "wt-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ports) != 0 {
		t.Errorf("expected no ports for a connectionless worktree, got %v", ports)
	}
	if len(agent.execCalls) != 0 {
		t.Error("expected no relay to the agent when no connectionId is set")
	}
}

// This is the regression test for TS Gap 7: a bound connectionId must
// always relay, never silently short-circuit to an empty result.
func TestScanWorkspacePorts_ConnectionIDBound_AlwaysRelays(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelaySSH, "ssht1")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	agent := &fakeDevServerAgentClient{execResult: map[string]any{"openPorts": []any{float64(3000), float64(8080)}}}
	uc := NewScanWorkspacePorts(resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	ports, err := uc.Execute(ctx, ScanWorkspacePortsInput{ConnectionID: "conn-1", WorktreeID: "wt-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agent.execCalls) != 1 || agent.execCalls[0] != "ports.scan" {
		t.Fatalf("expected exactly one ports.scan relay call, got %v", agent.execCalls)
	}
	if len(ports) != 2 || ports[0] != 3000 || ports[1] != 8080 {
		t.Errorf("expected [3000 8080], got %v", ports)
	}
}

// A bound connectionId whose agent call fails must propagate the error, not
// swallow it into an empty result — the exact bug class TS Gap 7 describes.
func TestScanWorkspacePorts_ConnectionIDBound_AgentFailurePropagates(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelaySSH, "ssht1")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	agent := &fakeDevServerAgentClient{execErr: errors.New("devserveragent: not implemented")}
	uc := NewScanWorkspacePorts(resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err = uc.Execute(ctx, ScanWorkspacePortsInput{ConnectionID: "conn-1", WorktreeID: "wt-1"})
	if err == nil {
		t.Fatal("expected the agent's error to propagate, not be swallowed into an empty result")
	}
}

func TestScanWorkspacePorts_ConnectionIDNotResolved_ReturnsEmptyWithoutRelaying(t *testing.T) {
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{}}
	agent := &fakeDevServerAgentClient{}
	uc := NewScanWorkspacePorts(resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	ports, err := uc.Execute(ctx, ScanWorkspacePortsInput{ConnectionID: "unknown-conn", WorktreeID: "wt-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ports) != 0 {
		t.Errorf("expected no ports when the connectionId doesn't resolve, got %v", ports)
	}
	if len(agent.execCalls) != 0 {
		t.Error("expected no relay to the agent when the connectionId doesn't resolve to a live dev server")
	}
}
