package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type GetCapabilitiesInput struct {
	ConnectionID string
	DevServerID  string
	Refresh      bool
}

type GetCapabilitiesResult struct {
	Profile   domain.CapabilityProfile
	Connected bool
	Found     bool
}

// GetDevServerCapabilities serves the stored profile, probing the agent only
// when it is connected and the profile is missing, stale or a refresh was
// requested. A disconnected dev server never triggers a dial.
type GetDevServerCapabilities struct {
	resolver   ConnectionResolver
	devServers DevServerRepository
	store      CapabilityProfileStore
	refresh    *RefreshDevServerCapabilities
	agent      DevServerAgentClient
	ttl        time.Duration
	clock      Clock
	logger     *slog.Logger
}

func NewGetDevServerCapabilities(
	resolver ConnectionResolver,
	devServers DevServerRepository,
	store CapabilityProfileStore,
	refresh *RefreshDevServerCapabilities,
	agent DevServerAgentClient,
	ttl time.Duration,
	clock Clock,
) *GetDevServerCapabilities {
	return &GetDevServerCapabilities{
		resolver:   resolver,
		devServers: devServers,
		store:      store,
		refresh:    refresh,
		agent:      agent,
		ttl:        ttl,
		clock:      clock,
		logger:     slog.Default(),
	}
}

func (uc *GetDevServerCapabilities) Execute(ctx context.Context, in GetCapabilitiesInput) (GetCapabilitiesResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return GetCapabilitiesResult{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if (in.ConnectionID == "") == (in.DevServerID == "") {
		return GetCapabilitiesResult{}, apperrors.New(apperrors.KindInvalidArgument, "INFRA_CAPABILITY_BAD_REQUEST", "exactly one of connection_id or dev_server_id is required", nil)
	}

	devServerID := in.DevServerID
	if in.ConnectionID != "" {
		found, ds, _, rerr := uc.resolver.ResolveConnection(ctx, tenantID, in.ConnectionID)
		if rerr != nil {
			return GetCapabilitiesResult{}, apperrors.New(apperrors.KindInternal, "INFRA_RESOLVE_FAILED", "failed to resolve connection", rerr)
		}
		if !found {
			return GetCapabilitiesResult{}, apperrors.New(apperrors.KindNotFound, "INFRA_CONNECTION_NOT_FOUND", "connection not found for this tenant", nil)
		}
		devServerID = ds.ID
	} else if _, gerr := uc.devServers.Get(ctx, tenantID, devServerID); gerr != nil {
		return GetCapabilitiesResult{}, apperrors.New(apperrors.KindNotFound, "INFRA_DEV_SERVER_NOT_FOUND", "dev server not found for this tenant", gerr)
	}

	p, found, err := uc.store.Get(ctx, tenantID, devServerID)
	if err != nil {
		return GetCapabilitiesResult{}, apperrors.New(apperrors.KindInternal, "INFRA_CAPABILITY_STORE_FAILED", "failed to read capability profile", err)
	}

	connected := uc.agent.IsConnected(devServerID)
	stale := found && uc.clock.Now().Sub(p.ProbedAt) > uc.ttl
	if connected && (in.Refresh || !found || stale) {
		np, rerr := uc.refresh.Execute(ctx, tenantID, devServerID, in.Refresh)
		switch {
		case rerr == nil:
			p, found = np, true
		case !found:
			return GetCapabilitiesResult{}, rerr
		default:
			uc.logger.WarnContext(ctx, "capability refresh failed, serving stored profile", slog.String("devServerId", devServerID), slog.Any("error", rerr))
		}
	}

	if !found {
		return GetCapabilitiesResult{}, apperrors.New(apperrors.KindNotFound, "INFRA_CAPABILITY_PROFILE_NOT_FOUND", "no capability profile for this dev server yet", nil)
	}
	return GetCapabilitiesResult{Profile: p, Connected: connected, Found: true}, nil
}
