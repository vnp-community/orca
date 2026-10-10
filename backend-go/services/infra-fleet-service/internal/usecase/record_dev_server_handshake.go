package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// HandshakeSource gives the handshake facts of a dev server's live session. Implemented by devserveragent.Client.
type HandshakeSource interface {
	LastHandshakeInfo(devServerID string) (HandshakeInfo, bool)
}

// RecordDevServerHandshake stores what an agent said about itself (platform, arch, Node and agent version) when its
// session attaches. Only the SSH connect and bulk provisioning paths wrote these columns before, so a dev server that
// connects by WebSocket showed null platform and a health status of "pending" forever.
type RecordDevServerHandshake struct {
	devServers DevServerRepository
	agent      HandshakeSource
	clock      Clock
}

func NewRecordDevServerHandshake(devServers DevServerRepository, agent HandshakeSource, clock Clock) *RecordDevServerHandshake {
	return &RecordDevServerHandshake{devServers: devServers, agent: agent, clock: clock}
}

// Execute does nothing when the session is gone again. A health status other than pending is kept: a degraded or
// unhealthy verdict comes from the fleet health poll, which a reconnect does not clear.
func (uc *RecordDevServerHandshake) Execute(ctx context.Context, tenantID, devServerID string) error {
	info, ok := uc.agent.LastHandshakeInfo(devServerID)
	if !ok {
		return nil
	}
	ds, err := uc.devServers.Get(ctx, tenantID, devServerID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_DEV_SERVER_LOOKUP_FAILED", "failed to read the dev server for its handshake", err)
	}
	health := ds.HealthStatus
	if health == "" || health == domain.DevServerHealthPending {
		health = domain.DevServerHealthHealthy
	}
	if err := uc.devServers.UpdateProvisionResult(ctx, tenantID, devServerID, health, info, uc.clock.Now()); err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_DEV_SERVER_HANDSHAKE_NOT_SAVED", "failed to save the dev server's handshake facts", err)
	}
	return nil
}
