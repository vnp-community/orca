package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestIsDevServerConnected_RequiresTenantContext(t *testing.T) {
	uc := NewIsDevServerConnected(&fakeDevServerRepository{}, &fakeDevServerAgentClient{})
	_, err := uc.Execute(context.Background(), "ds-1")
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestIsDevServerConnected_RequiresDevServerID(t *testing.T) {
	uc := NewIsDevServerConnected(&fakeDevServerRepository{}, &fakeDevServerAgentClient{})
	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, "")
	if err == nil {
		t.Fatal("expected an error when devServerId is omitted")
	}
}

func TestIsDevServerConnected_DevServerNotFound(t *testing.T) {
	repo := &fakeDevServerRepository{getErr: errors.New("not found")}
	uc := NewIsDevServerConnected(repo, &fakeDevServerAgentClient{})
	ctx := withTenant(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, "ds-missing")
	if err == nil {
		t.Fatal("expected an error for an unknown devServerId")
	}
}

// TestIsDevServerConnected_ReflectsAgentState is the live-bug regression:
// devServer.list's Status was hardcoded to "disconnected" always — this
// usecase reports the agent's REAL live-session state instead.
func TestIsDevServerConnected_ReflectsAgentState(t *testing.T) {
	ds, err := domain.NewDevServer("ds-1", "tenant-1", "10.0.0.5", domain.ConnectionModeDirectWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	repo := &fakeDevServerRepository{byID: map[string]domain.DevServer{"ds-1": ds}}
	ctx := withTenant(context.Background(), "tenant-1")

	connectedAgent := &fakeDevServerAgentClient{isConnected: true}
	got, err := NewIsDevServerConnected(repo, connectedAgent).Execute(ctx, "ds-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("want true when the agent reports a live session")
	}

	disconnectedAgent := &fakeDevServerAgentClient{isConnected: false}
	got, err = NewIsDevServerConnected(repo, disconnectedAgent).Execute(ctx, "ds-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("want false when the agent reports no live session")
	}
}
