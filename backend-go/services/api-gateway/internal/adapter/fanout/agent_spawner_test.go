package fanout

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
)

// fakeProjectServiceClient/fakeInfraFleetServiceClient are minimal test
// doubles — embed the nil interface and override only the methods
// GRPCAgentSpawner actually calls, matching this task set's sibling adapter
// test convention (see channels_worktree_test.go's fakeGitGatewayServiceClient).
type fakeProjectServiceClient struct {
	projectv1.ProjectServiceClient

	getProjectFunc   func(ctx context.Context, in *projectv1.GetProjectRequest) (*projectv1.GetProjectResponse, error)
	calledGetProject bool
}

func (f *fakeProjectServiceClient) GetProject(ctx context.Context, in *projectv1.GetProjectRequest, _ ...grpc.CallOption) (*projectv1.GetProjectResponse, error) {
	f.calledGetProject = true
	return f.getProjectFunc(ctx, in)
}

type fakeInfraFleetServiceClient struct {
	infrafleetv1.InfraFleetServiceClient

	resolveConnectionFunc    func(ctx context.Context, in *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error)
	spawnTerminalSessionFunc func(ctx context.Context, in *infrafleetv1.SpawnTerminalSessionRequest) (*infrafleetv1.SpawnTerminalSessionResponse, error)
	calledResolveConnection  bool
	calledSpawnTerminal      bool
}

func (f *fakeInfraFleetServiceClient) ResolveConnection(ctx context.Context, in *infrafleetv1.ResolveConnectionRequest, _ ...grpc.CallOption) (*infrafleetv1.ResolveConnectionResponse, error) {
	f.calledResolveConnection = true
	return f.resolveConnectionFunc(ctx, in)
}

func (f *fakeInfraFleetServiceClient) SpawnTerminalSession(ctx context.Context, in *infrafleetv1.SpawnTerminalSessionRequest, _ ...grpc.CallOption) (*infrafleetv1.SpawnTerminalSessionResponse, error) {
	f.calledSpawnTerminal = true
	return f.spawnTerminalSessionFunc(ctx, in)
}

// TestGRPCAgentSpawner_ResolvesInOrder_GetProjectThenResolveConnectionThenSpawn
// asserts the two-hop resolution SOL-WT-02 describes: GetProject ->
// ResolveConnection -> SpawnTerminalSession, in that order, and that Shell
// is populated via agentLaunchCommand.
func TestGRPCAgentSpawner_ResolvesInOrder_GetProjectThenResolveConnectionThenSpawn(t *testing.T) {
	var order []string
	var gotResolveReq *infrafleetv1.ResolveConnectionRequest
	var gotSpawnReq *infrafleetv1.SpawnTerminalSessionRequest

	project := &fakeProjectServiceClient{
		getProjectFunc: func(_ context.Context, in *projectv1.GetProjectRequest) (*projectv1.GetProjectResponse, error) {
			order = append(order, "GetProject")
			if in.GetId() != "proj-1" {
				t.Errorf("unexpected GetProjectRequest: %+v", in)
			}
			return &projectv1.GetProjectResponse{Project: &projectv1.Project{Id: "proj-1", DevServerId: "dev-1"}}, nil
		},
	}
	infra := &fakeInfraFleetServiceClient{
		resolveConnectionFunc: func(_ context.Context, in *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error) {
			order = append(order, "ResolveConnection")
			gotResolveReq = in
			return &infrafleetv1.ResolveConnectionResponse{Connected: true, ConnectionId: "conn-1"}, nil
		},
		spawnTerminalSessionFunc: func(_ context.Context, in *infrafleetv1.SpawnTerminalSessionRequest) (*infrafleetv1.SpawnTerminalSessionResponse, error) {
			order = append(order, "SpawnTerminalSession")
			gotSpawnReq = in
			return &infrafleetv1.SpawnTerminalSessionResponse{Session: &infrafleetv1.TerminalSession{PtyId: "pty-1"}}, nil
		},
	}

	spawner := NewGRPCAgentSpawner(project, infra)
	ptyID, connectionID, err := spawner.SpawnAgentTerminal(context.Background(), "proj-1", "/repo-feature", "claude")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ptyID != "pty-1" || connectionID != "conn-1" {
		t.Errorf("unexpected result: ptyID=%q connectionID=%q", ptyID, connectionID)
	}

	if len(order) != 3 || order[0] != "GetProject" || order[1] != "ResolveConnection" || order[2] != "SpawnTerminalSession" {
		t.Fatalf("expected call order [GetProject ResolveConnection SpawnTerminalSession], got %v", order)
	}
	if gotResolveReq.GetDevServerId() != "dev-1" {
		t.Errorf("expected ResolveConnectionRequest.DevServerId=dev-1, got %q", gotResolveReq.GetDevServerId())
	}
	if gotSpawnReq.GetConnectionId() != "conn-1" || gotSpawnReq.GetCwd() != "/repo-feature" {
		t.Errorf("unexpected SpawnTerminalSessionRequest: %+v", gotSpawnReq)
	}
	if gotSpawnReq.GetShell() != "claude" {
		t.Errorf("expected Shell to be populated via agentLaunchCommand(\"claude\")=claude, got %q", gotSpawnReq.GetShell())
	}
}

func TestGRPCAgentSpawner_UnknownAgentType_ShellEmpty(t *testing.T) {
	var gotSpawnReq *infrafleetv1.SpawnTerminalSessionRequest
	project := &fakeProjectServiceClient{
		getProjectFunc: func(_ context.Context, _ *projectv1.GetProjectRequest) (*projectv1.GetProjectResponse, error) {
			return &projectv1.GetProjectResponse{Project: &projectv1.Project{Id: "proj-1", DevServerId: "dev-1"}}, nil
		},
	}
	infra := &fakeInfraFleetServiceClient{
		resolveConnectionFunc: func(_ context.Context, _ *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error) {
			return &infrafleetv1.ResolveConnectionResponse{Connected: true, ConnectionId: "conn-1"}, nil
		},
		spawnTerminalSessionFunc: func(_ context.Context, in *infrafleetv1.SpawnTerminalSessionRequest) (*infrafleetv1.SpawnTerminalSessionResponse, error) {
			gotSpawnReq = in
			return &infrafleetv1.SpawnTerminalSessionResponse{Session: &infrafleetv1.TerminalSession{PtyId: "pty-1"}}, nil
		},
	}

	spawner := NewGRPCAgentSpawner(project, infra)
	if _, _, err := spawner.SpawnAgentTerminal(context.Background(), "proj-1", "/repo-feature", "some-unknown-agent"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotSpawnReq.GetShell() != "" {
		t.Errorf("expected empty Shell for an unknown agent type, got %q", gotSpawnReq.GetShell())
	}
}

func TestGRPCAgentSpawner_GetProjectFails_ResolveConnectionNeverCalled(t *testing.T) {
	project := &fakeProjectServiceClient{
		getProjectFunc: func(_ context.Context, _ *projectv1.GetProjectRequest) (*projectv1.GetProjectResponse, error) {
			return nil, errors.New("project-service unreachable")
		},
	}
	infra := &fakeInfraFleetServiceClient{}

	spawner := NewGRPCAgentSpawner(project, infra)
	if _, _, err := spawner.SpawnAgentTerminal(context.Background(), "proj-1", "/repo-feature", "claude"); err == nil {
		t.Fatal("expected an error")
	}
	if infra.calledResolveConnection || infra.calledSpawnTerminal {
		t.Error("expected neither ResolveConnection nor SpawnTerminalSession to be called when GetProject fails")
	}
}
