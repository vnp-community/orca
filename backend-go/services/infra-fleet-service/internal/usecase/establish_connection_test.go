package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestEstablishConnection_HealthGatesResult(t *testing.T) {
	t.Run("healthy agent establishes connection", func(t *testing.T) {
		sshTargets := &fakeSshTargetRepository{single: domain.SshTarget{ID: "s1", TenantID: "t1", Host: "h1"}}
		devServers := &fakeDevServerRepository{found: false}
		conns := &fakeConnectionRepository{}
		agent := &fakeDevServerAgentClient{healthy: true}
		uc := NewEstablishConnection(sshTargets, devServers, conns, agent)

		conn, err := uc.Execute(withTenant(context.Background(), "t1"), EstablishConnectionInput{SshTargetID: "s1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if conn.Status != "established" {
			t.Errorf("got status %q, want established", conn.Status)
		}
		if !devServers.registerCalled || devServers.lastRegistered.Mode != domain.ConnectionModeRelaySSH {
			t.Error("expected a relay-ssh-mode DevServer to be registered")
		}
	})

	t.Run("unreachable agent fails", func(t *testing.T) {
		sshTargets := &fakeSshTargetRepository{single: domain.SshTarget{ID: "s1", TenantID: "t1", Host: "h1"}}
		devServers := &fakeDevServerRepository{found: false}
		conns := &fakeConnectionRepository{}
		agent := &fakeDevServerAgentClient{healthy: false}
		uc := NewEstablishConnection(sshTargets, devServers, conns, agent)

		_, err := uc.Execute(withTenant(context.Background(), "t1"), EstablishConnectionInput{SshTargetID: "s1"})
		if err == nil {
			t.Fatal("expected error when agent is unreachable")
		}
	})

	t.Run("existing dev server binding is reused, not re-registered", func(t *testing.T) {
		sshTargets := &fakeSshTargetRepository{single: domain.SshTarget{ID: "s1", TenantID: "t1", Host: "h1"}}
		devServers := &fakeDevServerRepository{found: true, bySshTarget: domain.DevServer{ID: "ds1", TenantID: "t1", Host: "h1", Mode: domain.ConnectionModeRelaySSH, SSHTargetID: "s1"}}
		conns := &fakeConnectionRepository{}
		agent := &fakeDevServerAgentClient{healthy: true}
		uc := NewEstablishConnection(sshTargets, devServers, conns, agent)

		conn, err := uc.Execute(withTenant(context.Background(), "t1"), EstablishConnectionInput{SshTargetID: "s1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if conn.DevServerID != "ds1" {
			t.Errorf("expected the existing dev server to be reused, got %q", conn.DevServerID)
		}
		if devServers.registerCalled {
			t.Error("expected no new DevServer to be registered when one is already bound")
		}
	})
}

func TestEstablishConnection_PersistsHandshakeInfoAfterSuccessfulConnect(t *testing.T) {
	sshTargets := &fakeSshTargetRepository{single: domain.SshTarget{ID: "s1", TenantID: "t1", Host: "h1"}}
	devServers := &fakeDevServerRepository{found: false}
	conns := &fakeConnectionRepository{}
	fixture := HandshakeInfo{Platform: "linux", Arch: "x64", NodeVersion: "v22.3.0", AgentVersion: "5.0.0"}
	agent := &fakeDevServerAgentClient{healthy: true, lastHandshakeInfo: fixture, lastHandshakeOK: true}
	uc := NewEstablishConnection(sshTargets, devServers, conns, agent)

	_, err := uc.Execute(withTenant(context.Background(), "t1"), EstablishConnectionInput{SshTargetID: "s1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if devServers.updateProvisionResultCalls != 1 {
		t.Fatalf("expected UpdateProvisionResult to be called exactly once, got %d", devServers.updateProvisionResultCalls)
	}
	if devServers.lastProvisionStatus != domain.DevServerStatusHealthy {
		t.Errorf("expected status=healthy, got %q", devServers.lastProvisionStatus)
	}
	if devServers.lastProvisionInfo != fixture {
		t.Errorf("expected the handshake info to be persisted verbatim, got %+v", devServers.lastProvisionInfo)
	}
}

func TestEstablishConnection_LastHandshakeInfoNotOKSkipsPersistWithoutErroring(t *testing.T) {
	sshTargets := &fakeSshTargetRepository{single: domain.SshTarget{ID: "s1", TenantID: "t1", Host: "h1"}}
	devServers := &fakeDevServerRepository{found: false}
	conns := &fakeConnectionRepository{}
	agent := &fakeDevServerAgentClient{healthy: true, lastHandshakeOK: false}
	uc := NewEstablishConnection(sshTargets, devServers, conns, agent)

	conn, err := uc.Execute(withTenant(context.Background(), "t1"), EstablishConnectionInput{SshTargetID: "s1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn.Status != "established" {
		t.Errorf("expected the connection to still establish successfully, got status %q", conn.Status)
	}
	if devServers.updateProvisionResultCalls != 0 {
		t.Errorf("expected UpdateProvisionResult to be skipped when LastHandshakeInfo's ok=false, got %d calls", devServers.updateProvisionResultCalls)
	}
}

func TestEstablishConnection_RequiresTenantContext(t *testing.T) {
	uc := NewEstablishConnection(&fakeSshTargetRepository{}, &fakeDevServerRepository{}, &fakeConnectionRepository{}, &fakeDevServerAgentClient{})
	_, err := uc.Execute(context.Background(), EstablishConnectionInput{SshTargetID: "s1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}
