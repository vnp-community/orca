package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestResolveDirectWebSocketDevServer_ReusesExistingRow(t *testing.T) {
	existing := domain.DevServer{
		ID:       "ds-existing",
		TenantID: "tenant-1",
		Host:     "dev-01",
		Mode:     domain.ConnectionModeDirectWebSocket,
		Status:   domain.DevServerStatusApproved,
		GroupID:  "group-1",
	}
	repo := &fakeDevServerRepository{foundByHost: true, byHostAndMode: existing}
	uc := NewResolveDirectWebSocketDevServer(repo)

	got, err := uc.Execute(context.Background(), ResolveDirectWebSocketDevServerInput{
		TenantID:    "tenant-1",
		DevServerID: "dev-01",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != existing {
		t.Fatalf("want existing row returned unchanged, got %+v", got)
	}
	if repo.registerCalled {
		t.Error("want Register NOT called when a row already exists — an admin's approval/group must survive an agent reconnect")
	}
}

func TestResolveDirectWebSocketDevServer_RegistersNewRowWhenNoneExists(t *testing.T) {
	repo := &fakeDevServerRepository{foundByHost: false, byID: map[string]domain.DevServer{}}
	uc := NewResolveDirectWebSocketDevServer(repo)

	got, err := uc.Execute(context.Background(), ResolveDirectWebSocketDevServerInput{
		TenantID:    "tenant-1",
		DevServerID: "dev-01",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.registerCalled {
		t.Fatal("want Register called for a first-time devServerID")
	}
	if got.TenantID != "tenant-1" || got.Host != "dev-01" || got.Mode != domain.ConnectionModeDirectWebSocket {
		t.Fatalf("unexpected registered dev server: %+v", got)
	}
	if got.ID == "" {
		t.Error("want a generated non-empty ID")
	}
	// Why this matters: NewDevServer always defaults Status to
	// pending_approval — the whole point of CR-DS-006's admin-approval
	// gate. A newly-connected agent must show up in the Admin Console's
	// Approvals tab, not silently pre-approved.
	if got.Status != domain.DevServerStatusPendingApproval {
		t.Errorf("want pending_approval status on first registration, got %q", got.Status)
	}
}

func TestResolveDirectWebSocketDevServer_PropagatesFindError(t *testing.T) {
	repo := &fakeDevServerRepository{findByHostErr: errors.New("db down")}
	uc := NewResolveDirectWebSocketDevServer(repo)

	_, err := uc.Execute(context.Background(), ResolveDirectWebSocketDevServerInput{
		TenantID:    "tenant-1",
		DevServerID: "dev-01",
	})
	if err == nil {
		t.Fatal("want error propagated from FindByHostAndMode")
	}
}

func TestResolveDirectWebSocketDevServer_PropagatesRegisterError(t *testing.T) {
	repo := &fakeDevServerRepository{foundByHost: false, registerErr: errors.New("insert failed")}
	uc := NewResolveDirectWebSocketDevServer(repo)

	_, err := uc.Execute(context.Background(), ResolveDirectWebSocketDevServerInput{
		TenantID:    "tenant-1",
		DevServerID: "dev-01",
	})
	if err == nil {
		t.Fatal("want error propagated from Register")
	}
}
