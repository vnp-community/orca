package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stablyai/orca-go/common/auditclient"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// fakeAuthServiceClient stubs AppendAuditEntry only — every other
// AuthServiceClient method is left nil-embedded and unused, mirroring
// common/auditclient/client_test.go's own fake.
type fakeAuthServiceClient struct {
	authv1.AuthServiceClient
	calls   int
	lastReq *authv1.AppendAuditEntryRequest
}

func (f *fakeAuthServiceClient) AppendAuditEntry(_ context.Context, in *authv1.AppendAuditEntryRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	f.calls++
	f.lastReq = in
	return &emptypb.Empty{}, nil
}

func TestEstablishConnection_HealthGatesResult(t *testing.T) {
	t.Run("healthy agent establishes connection", func(t *testing.T) {
		sshTargets := &fakeSshTargetRepository{single: domain.SshTarget{ID: "s1", TenantID: "t1", Host: "h1"}}
		devServers := &fakeDevServerRepository{found: false}
		conns := &fakeConnectionRepository{}
		agent := &fakeDevServerAgentClient{healthy: true}
		uc := NewEstablishConnection(sshTargets, devServers, conns, agent, nil)

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
		uc := NewEstablishConnection(sshTargets, devServers, conns, agent, nil)

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
		uc := NewEstablishConnection(sshTargets, devServers, conns, agent, nil)

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
	uc := NewEstablishConnection(sshTargets, devServers, conns, agent, nil)

	_, err := uc.Execute(withTenant(context.Background(), "t1"), EstablishConnectionInput{SshTargetID: "s1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if devServers.updateProvisionResultCalls != 1 {
		t.Fatalf("expected UpdateProvisionResult to be called exactly once, got %d", devServers.updateProvisionResultCalls)
	}
	if devServers.lastProvisionStatus != domain.DevServerHealthHealthy {
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
	uc := NewEstablishConnection(sshTargets, devServers, conns, agent, nil)

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
	uc := NewEstablishConnection(&fakeSshTargetRepository{}, &fakeDevServerRepository{}, &fakeConnectionRepository{}, &fakeDevServerAgentClient{}, nil)
	_, err := uc.Execute(context.Background(), EstablishConnectionInput{SshTargetID: "s1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

// --- TASK-BE-022: audit-append (action="ssh.connect") on both allow and
// deny branches ---

func TestEstablishConnection_UnreachableAgentAppendsExactlyOneDeniedAuditEntry(t *testing.T) {
	sshTargets := &fakeSshTargetRepository{single: domain.SshTarget{ID: "s1", TenantID: "t1", Host: "h1"}}
	devServers := &fakeDevServerRepository{found: false}
	conns := &fakeConnectionRepository{}
	agent := &fakeDevServerAgentClient{healthy: false}
	fake := &fakeAuthServiceClient{}
	uc := NewEstablishConnection(sshTargets, devServers, conns, agent, auditclient.New(fake))

	ctx := tenant.WithUserID(withTenant(context.Background(), "t1"), "user-1")
	if _, err := uc.Execute(ctx, EstablishConnectionInput{SshTargetID: "s1"}); err == nil {
		t.Fatal("expected error when agent is unreachable")
	}
	if fake.calls != 1 {
		t.Fatalf("expected exactly one AppendAuditEntry call, got %d", fake.calls)
	}
	if fake.lastReq.GetAction() != "ssh.connect" {
		t.Fatalf("expected action %q, got %q", "ssh.connect", fake.lastReq.GetAction())
	}
	if fake.lastReq.GetOutcome() != "denied" {
		t.Fatalf("expected outcome %q, got %q", "denied", fake.lastReq.GetOutcome())
	}
	if fake.lastReq.GetActorId() != "user-1" {
		t.Fatalf("expected actor_id %q, got %q", "user-1", fake.lastReq.GetActorId())
	}
}

func TestEstablishConnection_HealthyAgentAppendsExactlyOneAllowedAuditEntry(t *testing.T) {
	sshTargets := &fakeSshTargetRepository{single: domain.SshTarget{ID: "s1", TenantID: "t1", Host: "h1"}}
	devServers := &fakeDevServerRepository{found: false}
	conns := &fakeConnectionRepository{}
	agent := &fakeDevServerAgentClient{healthy: true}
	fake := &fakeAuthServiceClient{}
	uc := NewEstablishConnection(sshTargets, devServers, conns, agent, auditclient.New(fake))

	ctx := tenant.WithUserID(withTenant(context.Background(), "t1"), "user-1")
	conn, err := uc.Execute(ctx, EstablishConnectionInput{SshTargetID: "s1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.calls != 1 {
		t.Fatalf("expected exactly one AppendAuditEntry call, got %d", fake.calls)
	}
	if fake.lastReq.GetOutcome() != "allowed" {
		t.Fatalf("expected outcome %q, got %q", "allowed", fake.lastReq.GetOutcome())
	}
	if fake.lastReq.GetTarget() != "devserver:"+conn.DevServerID {
		t.Fatalf("expected target %q, got %q", "devserver:"+conn.DevServerID, fake.lastReq.GetTarget())
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
	uc := NewEstablishConnection(sshTargets, devServers, conns, agent, nil)

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
	uc := NewEstablishConnection(sshTargets, devServers, conns, agent, nil)

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
	uc := NewEstablishConnection(sshTargets, devServers, conns, agent, nil)

	_, err := uc.Execute(withTenant(context.Background(), "t1"), EstablishConnectionInput{SshTargetID: "s1"})
	if err == nil {
		t.Fatal("expected error to propagate from the outbox-enqueueing repository call")
	}
}

// TestEstablishConnection_NilAuditClientIsANoOp proves the auditClient is
// optional (every test above except the two audit-specific ones never wires
// one) and never panics.
func TestEstablishConnection_NilAuditClientIsANoOp(t *testing.T) {
	sshTargets := &fakeSshTargetRepository{single: domain.SshTarget{ID: "s1", TenantID: "t1", Host: "h1"}}
	devServers := &fakeDevServerRepository{found: false}
	conns := &fakeConnectionRepository{}
	agent := &fakeDevServerAgentClient{healthy: true}
	uc := NewEstablishConnection(sshTargets, devServers, conns, agent, nil)

	ctx := tenant.WithUserID(withTenant(context.Background(), "t1"), "user-1")
	if _, err := uc.Execute(ctx, EstablishConnectionInput{SshTargetID: "s1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
