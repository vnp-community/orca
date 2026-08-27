package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

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
		devServer, err = domain.NewDevServer(uuid.NewString(), tenantID, target.Host, domain.ConnectionModeRelaySSH, target.ID, nil)
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
	return uc.conns.CreateConnection(ctx, conn)
}
