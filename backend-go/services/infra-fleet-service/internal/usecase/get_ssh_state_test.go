package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestGetSshState_ThreeCases(t *testing.T) {
	cases := []struct {
		name           string
		devServerFound bool
		connFound      bool
		wantConnected  bool
	}{
		{name: "no dev server bound", devServerFound: false, wantConnected: false},
		{name: "dev server bound, no active connection", devServerFound: true, connFound: false, wantConnected: false},
		{name: "dev server bound, active connection", devServerFound: true, connFound: true, wantConnected: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			devServers := &fakeDevServerRepository{found: tc.devServerFound}
			conns := &fakeConnectionRepository{found: tc.connFound}
			uc := NewGetSshState(&fakeSshTargetRepository{}, devServers, conns)

			got, err := uc.Execute(withTenant(context.Background(), "t1"), SshStateInput{SshTargetID: "s1"})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Connected != tc.wantConnected {
				t.Errorf("got Connected=%v, want %v", got.Connected, tc.wantConnected)
			}
		})
	}
}

// TestGetSshState_ReconnectingStatus is TASK-SSH-03-05's regression: a
// connection row in "reconnecting" status must report Connected=false and
// Status="reconnecting" — distinct from "closed" (which GetActiveByDevServer
// never even returns, found=false instead) and distinct from a fully
// established connection.
func TestGetSshState_ReconnectingStatus(t *testing.T) {
	devServers := &fakeDevServerRepository{found: true}
	conns := &fakeConnectionRepository{
		found:      true,
		activeConn: domain.Connection{ID: "conn-1", Status: "reconnecting"},
	}
	uc := NewGetSshState(&fakeSshTargetRepository{}, devServers, conns)

	got, err := uc.Execute(withTenant(context.Background(), "t1"), SshStateInput{SshTargetID: "s1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Connected {
		t.Error("expected Connected=false while reconnecting")
	}
	if got.Status != "reconnecting" {
		t.Errorf("expected Status=%q, got %q", "reconnecting", got.Status)
	}
	if got.ConnectionID != "conn-1" {
		t.Errorf("expected ConnectionID to still be populated, got %q", got.ConnectionID)
	}
}

func TestGetSshState_EstablishedStatus_ReportsConnectedTrue(t *testing.T) {
	devServers := &fakeDevServerRepository{found: true}
	conns := &fakeConnectionRepository{
		found:      true,
		activeConn: domain.Connection{ID: "conn-2", Status: "established"},
	}
	uc := NewGetSshState(&fakeSshTargetRepository{}, devServers, conns)

	got, err := uc.Execute(withTenant(context.Background(), "t1"), SshStateInput{SshTargetID: "s1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Connected {
		t.Error("expected Connected=true for an established connection")
	}
	if got.Status != "established" {
		t.Errorf("expected Status=%q, got %q", "established", got.Status)
	}
}

func TestGetSshState_RequiresTenantContext(t *testing.T) {
	uc := NewGetSshState(&fakeSshTargetRepository{}, &fakeDevServerRepository{}, &fakeConnectionRepository{})
	_, err := uc.Execute(context.Background(), SshStateInput{SshTargetID: "s1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}
