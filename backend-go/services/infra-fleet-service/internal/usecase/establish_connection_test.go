package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
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

func TestEstablishConnection_RequiresTenantContext(t *testing.T) {
	uc := NewEstablishConnection(&fakeSshTargetRepository{}, &fakeDevServerRepository{}, &fakeConnectionRepository{}, &fakeDevServerAgentClient{})
	_, err := uc.Execute(context.Background(), EstablishConnectionInput{SshTargetID: "s1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

// TestEstablishConnection_PublishesSSHConnectedOutboxEvent confirms the
// outbox publish (TASK-AUTH-05-08) is attempted after a successful
// connection, with the payload auth-service's natsconsumer.AuditIngestConsumer
// expects.
func TestEstablishConnection_PublishesSSHConnectedOutboxEvent(t *testing.T) {
	sshTargets := &fakeSshTargetRepository{single: domain.SshTarget{ID: "s1", TenantID: "t1", Host: "10.0.0.9"}}
	devServers := &fakeDevServerRepository{found: false}
	conns := &fakeConnectionRepository{}
	agent := &fakeDevServerAgentClient{healthy: true}
	uc := NewEstablishConnection(sshTargets, devServers, conns, agent)

	ctx := tenant.WithUserID(withTenant(context.Background(), "t1"), "user-1")
	conn, err := uc.Execute(ctx, EstablishConnectionInput{SshTargetID: "s1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(conns.outboxEvents) != 1 {
		t.Fatalf("expected exactly 1 outbox event, got %d", len(conns.outboxEvents))
	}
	event := conns.outboxEvents[0]
	if event.Subject != SSHConnectedSubject {
		t.Errorf("got subject %q, want %q", event.Subject, SSHConnectedSubject)
	}
	if event.ID == "" {
		t.Error("expected a generated outbox event ID")
	}
	if event.OccurredAt.IsZero() {
		t.Error("expected a non-zero OccurredAt")
	}

	var payload sshConnectedPayload
	if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil {
		t.Fatalf("unmarshaling payload: %v", err)
	}
	if payload.ActorUserID != "user-1" {
		t.Errorf("got actor_user_id %q, want %q", payload.ActorUserID, "user-1")
	}
	if payload.ConnectionID != conn.ID {
		t.Errorf("got connection_id %q, want %q", payload.ConnectionID, conn.ID)
	}
	if payload.Host != "10.0.0.9" {
		t.Errorf("got host %q, want %q", payload.Host, "10.0.0.9")
	}

	// CreateConnection (no-outbox) must never be called on this path — the
	// atomic CreateConnectionWithOutbox is the only write.
	if len(conns.created) != 0 {
		t.Errorf("expected no plain CreateConnection call, got %d", len(conns.created))
	}
}

// TestEstablishConnection_MissingActorUserIDIsNotFatal confirms a missing
// user in context degrades the outbox event's actor field rather than
// failing the connection — EstablishConnection has never required a user in
// context (service-to-service callers are legitimate).
func TestEstablishConnection_MissingActorUserIDIsNotFatal(t *testing.T) {
	sshTargets := &fakeSshTargetRepository{single: domain.SshTarget{ID: "s1", TenantID: "t1", Host: "h1"}}
	devServers := &fakeDevServerRepository{found: false}
	conns := &fakeConnectionRepository{}
	agent := &fakeDevServerAgentClient{healthy: true}
	uc := NewEstablishConnection(sshTargets, devServers, conns, agent)

	_, err := uc.Execute(withTenant(context.Background(), "t1"), EstablishConnectionInput{SshTargetID: "s1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(conns.outboxEvents) != 1 {
		t.Fatalf("expected the outbox publish to still be attempted, got %d events", len(conns.outboxEvents))
	}
	var payload sshConnectedPayload
	if err := json.Unmarshal(conns.outboxEvents[0].PayloadJSON, &payload); err != nil {
		t.Fatalf("unmarshaling payload: %v", err)
	}
	if payload.ActorUserID != "" {
		t.Errorf("expected an empty actor_user_id, got %q", payload.ActorUserID)
	}
}

// TestEstablishConnection_OutboxWriteFailurePropagates confirms the write
// path's only failure surface for the outbox enqueue is the SAME repository
// call that already wrote the connection — CreateConnectionWithOutbox
// failing fails the whole Execute call exactly the way a plain
// CreateConnection failure always has (this is not a NEW failure mode the
// outbox introduces); the actual async NATS publish (common/outbox.Relay,
// started in cmd/server/main.go) is fully decoupled from this call and can
// never fail it.
func TestEstablishConnection_OutboxWriteFailurePropagates(t *testing.T) {
	sshTargets := &fakeSshTargetRepository{single: domain.SshTarget{ID: "s1", TenantID: "t1", Host: "h1"}}
	devServers := &fakeDevServerRepository{found: false}
	conns := &fakeConnectionRepository{outboxErr: errors.New("db unavailable")}
	agent := &fakeDevServerAgentClient{healthy: true}
	uc := NewEstablishConnection(sshTargets, devServers, conns, agent)

	_, err := uc.Execute(withTenant(context.Background(), "t1"), EstablishConnectionInput{SshTargetID: "s1"})
	if err == nil {
		t.Fatal("expected error to propagate from the outbox-enqueueing repository call")
	}
}
