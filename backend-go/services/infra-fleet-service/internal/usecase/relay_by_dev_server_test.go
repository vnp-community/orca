package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestRelayByDevServer_RequiresTenantContext(t *testing.T) {
	uc := NewRelayByDevServer(&fakeDevServerRepository{}, &fakeDevServerAgentClient{})
	_, err := uc.Execute(context.Background(), RelayByDevServerInput{DevServerID: "ds-1", Method: "fs.readDir"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestRelayByDevServer_RequiresDevServerID(t *testing.T) {
	uc := NewRelayByDevServer(&fakeDevServerRepository{}, &fakeDevServerAgentClient{})
	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, RelayByDevServerInput{Method: "fs.readDir"})
	if err == nil {
		t.Fatal("expected an error when devServerId is omitted")
	}
}

func TestRelayByDevServer_RequiresMethod(t *testing.T) {
	uc := NewRelayByDevServer(&fakeDevServerRepository{}, &fakeDevServerAgentClient{})
	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, RelayByDevServerInput{DevServerID: "ds-1"})
	if err == nil {
		t.Fatal("expected an error when method is omitted")
	}
}

func TestRelayByDevServer_DevServerNotFound(t *testing.T) {
	repo := &fakeDevServerRepository{}
	repo.getErr = errors.New("not found")
	uc := NewRelayByDevServer(repo, &fakeDevServerAgentClient{})
	ctx := withTenant(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, RelayByDevServerInput{DevServerID: "ds-missing", Method: "fs.readDir"})
	if err == nil {
		t.Fatal("expected an error for an unknown devServerId")
	}
}

// TestRelayByDevServer_NotConnected is the "chicken-and-egg" regression:
// a freshly-connected dev server with no infra.connections row yet
// (nothing bound to it) must still be usable for a pre-project action like
// browsing — this usecase never touches infra.connections at all, and
// fails only on the agent's OWN live-session state.
func TestRelayByDevServer_NotConnected(t *testing.T) {
	ds, err := domain.NewDevServer("ds-1", "tenant-1", "10.0.0.5", domain.ConnectionModeDirectWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	repo := &fakeDevServerRepository{}
	repo.byID = map[string]domain.DevServer{"ds-1": ds}
	agent := &fakeDevServerAgentClient{isConnected: false}
	uc := NewRelayByDevServer(repo, agent)
	ctx := withTenant(context.Background(), "tenant-1")

	_, err = uc.Execute(ctx, RelayByDevServerInput{DevServerID: "ds-1", Method: "fs.readDir"})
	if err == nil {
		t.Fatal("expected an error when the dev server's agent has no live session")
	}
}

func TestRelayByDevServer_ConnectedRelaysToAgent(t *testing.T) {
	ds, err := domain.NewDevServer("ds-1", "tenant-1", "10.0.0.5", domain.ConnectionModeDirectWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	repo := &fakeDevServerRepository{}
	repo.byID = map[string]domain.DevServer{"ds-1": ds}
	agent := &fakeDevServerAgentClient{
		isConnected: true,
		execResult:  map[string]any{"entries": []any{}},
	}
	uc := NewRelayByDevServer(repo, agent)
	ctx := withTenant(context.Background(), "tenant-1")

	result, err := uc.Execute(ctx, RelayByDevServerInput{DevServerID: "ds-1", Method: "fs.readDir", Params: map[string]any{"path": "/"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent.lastMethod != "fs.readDir" {
		t.Errorf("want agent.Exec called with fs.readDir, got %q", agent.lastMethod)
	}
	if _, ok := result["entries"]; !ok {
		t.Errorf("unexpected result: %+v", result)
	}
}
