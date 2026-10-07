package usecase

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// StreamCodeIntelEvents subscribes to live code intelligence events from devServerID's agent session.
// Does NOT require the agent to be connected at the time of subscription.
type StreamCodeIntelEvents struct {
	devServers DevServerRepository
	source     CodeIntelEventSource
	limiter    *CodeIntelStreamLimiter
}

func NewStreamCodeIntelEvents(devServers DevServerRepository, source CodeIntelEventSource, limiter *CodeIntelStreamLimiter) *StreamCodeIntelEvents {
	return &StreamCodeIntelEvents{
		devServers: devServers,
		source:     source,
		limiter:    limiter,
	}
}

func (uc *StreamCodeIntelEvents) Execute(ctx context.Context, devServerID string) (<-chan domain.CodeIntelEvent, func(), error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, nil, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if devServerID == "" {
		return nil, nil, apperrors.New(apperrors.KindInvalidArgument, "INFRA_NO_DEV_SERVER", "devServerId is required", nil)
	}

	devServer, err := uc.devServers.Get(ctx, tenantID, devServerID)
	if err != nil {
		return nil, nil, apperrors.New(apperrors.KindNotFound, "INFRA_DEV_SERVER_NOT_FOUND", "dev server not found for this tenant", err)
	}

	limiterKey := fmt.Sprintf("%s|%s", tenantID, devServer.ID)
	releaseLimiter, err := uc.limiter.Acquire(limiterKey)
	if err != nil {
		return nil, nil, err
	}

	events, unsubscribe := uc.source.SubscribeCodeIntelEvents(devServer.ID)

	release := func() {
		unsubscribe()
		releaseLimiter()
	}

	return events, release, nil
}
