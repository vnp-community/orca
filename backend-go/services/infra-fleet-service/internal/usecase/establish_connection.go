package usecase

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// SSHConnectedSubject is the outbox event subject a successfully-established
// SSH connection publishes under — auth-service's natsconsumer.AuditIngestConsumer
// (TASK-AUTH-05-08) durably subscribes here to append an "ssh.connect" audit
// entry. Named orca.<service>.<entity>.<event> per
// specs/backend-go/architecture/08-inter-service-communication.md.
const SSHConnectedSubject = "orca.infrafleet.ssh.connected"

// sshConnectedPayload is SSHConnectedSubject's JSON payload shape.
// TenantID/OccurredAt are deliberately NOT duplicated here — they already
// travel on the eventbus.Event envelope itself (tenant_id/occurred_at
// columns on the outbox row), same convention usage-service's
// sessionRecordedPayload follows.
type sshConnectedPayload struct {
	ActorUserID  string `json:"actor_user_id"`
	ConnectionID string `json:"connection_id"`
	Host         string `json:"host"`
}

// EstablishConnection performs the actual SSH + Dev Server Agent handshake
// synchronously — it is the connection-establishment act, not a record of
// one requested.
type EstablishConnection struct {
	sshTargets SshTargetRepository
	devServers DevServerRepository
	conns      ConnectionRepository
	agent      DevServerAgentClient
}

func NewEstablishConnection(sshTargets SshTargetRepository, devServers DevServerRepository, conns ConnectionRepository, agent DevServerAgentClient) *EstablishConnection {
	return &EstablishConnection{sshTargets: sshTargets, devServers: devServers, conns: conns, agent: agent}
}

type EstablishConnectionInput struct {
	SshTargetID string
}

func (uc *EstablishConnection) Execute(ctx context.Context, in EstablishConnectionInput) (domain.Connection, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Connection{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	target, err := uc.sshTargets.Get(ctx, tenantID, in.SshTargetID)
	if err != nil {
		return domain.Connection{}, err
	}

	// Find-or-create the DevServer row this SSH target backs — an SSH
	// target only becomes routable once it's the ssh_target_id of a
	// relay-ssh-mode DevServer. ID generation happens here, not in
	// postgres/, matching register_dev_server.go's own convention.
	devServer, found, err := uc.devServers.FindBySshTarget(ctx, tenantID, target.ID)
	if err != nil {
		return domain.Connection{}, apperrors.New(apperrors.KindInternal, "INFRA_DEV_SERVER_RESOLVE_FAILED", "failed to resolve dev server for ssh target", err)
	}
	if !found {
		devServer, err = domain.NewDevServer(uuid.NewString(), tenantID, target.Host, domain.ConnectionModeRelaySSH, target.ID)
		if err != nil {
			return domain.Connection{}, apperrors.New(apperrors.KindInternal, "INFRA_DEV_SERVER_CONSTRUCT_FAILED", "failed to construct dev server for ssh target", err)
		}
		devServer, err = uc.devServers.Register(ctx, devServer)
		if err != nil {
			return domain.Connection{}, apperrors.New(apperrors.KindInternal, "INFRA_DEV_SERVER_REGISTER_FAILED", "failed to register dev server for ssh target", err)
		}
	}

	// The handshake itself — bootstrap/deploy is a separate concern if the
	// relay binary isn't deployed yet; Health() here confirms an
	// already-bootstrapped target is actually reachable before the
	// Connection is marked established. Per infra-fleet-service.md §8's
	// deadline rule, the caller (gRPC handler) carries an explicit timeout
	// longer than the intra-cluster default.
	reachable, err := uc.agent.Health(ctx, devServer)
	if err != nil || !reachable {
		return domain.Connection{}, apperrors.New(apperrors.KindFailedPrecondition, "INFRA_SSH_CONNECT_FAILED", "failed to establish SSH connection to target", err)
	}

	conn, err := domain.NewConnection(uuid.NewString(), tenantID, devServer.ID, "", "")
	if err != nil {
		return domain.Connection{}, apperrors.New(apperrors.KindInternal, "INFRA_CONNECTION_CONSTRUCT_FAILED", "failed to construct connection", err)
	}
	conn.Status = "established"

	// actorUserID is best-effort: EstablishConnection has never required a
	// user in context (service-to-service callers are legitimate), and
	// domain.AuditEntry already tolerates an empty ActorID for a
	// system-initiated event — so a missing user here degrades the audit
	// trail's actor field, not the connection itself.
	actorUserID, _ := tenant.UserID(ctx)

	event, err := uc.buildSSHConnectedOutboxEvent(actorUserID, conn.ID, target.Host)
	if err != nil {
		return domain.Connection{}, apperrors.New(apperrors.KindInternal, "INFRA_MARSHAL_EVENT_FAILED", "failed to marshal ssh.connect outbox event payload", err)
	}

	// Connection write and outbox enqueue happen in ONE transaction (Epic G,
	// docs/execution-plan.md; see domain.OutboxEvent's doc comment) —
	// mirrors usage-service's RecordUsageSession/SaveSession precedent.
	// Because the actual NATS publish happens asynchronously via
	// common/outbox.Relay (started in cmd/server/main.go), a publish
	// failure — NATS being unreachable — can never fail this call: the
	// outbox row is already durably committed here regardless, and the
	// relay simply retries until it succeeds.
	return uc.conns.CreateConnectionWithOutbox(ctx, conn, event)
}

// buildSSHConnectedOutboxEvent constructs the domain.OutboxEvent
// CreateConnectionWithOutbox enqueues alongside the connection write.
func (uc *EstablishConnection) buildSSHConnectedOutboxEvent(actorUserID, connectionID, host string) (domain.OutboxEvent, error) {
	payload, err := json.Marshal(sshConnectedPayload{
		ActorUserID:  actorUserID,
		ConnectionID: connectionID,
		Host:         host,
	})
	if err != nil {
		return domain.OutboxEvent{}, err
	}
	return domain.OutboxEvent{
		ID:          uuid.NewString(),
		Subject:     SSHConnectedSubject,
		OccurredAt:  time.Now().UTC(),
		PayloadJSON: payload,
	}, nil
}
