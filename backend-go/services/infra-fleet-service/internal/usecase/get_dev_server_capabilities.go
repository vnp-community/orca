package usecase

import (
	"context"
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

type GetDevServerCapabilities struct {
	resolver ConnectionResolver
	store    CapabilityProfileStore
	refresh  *RefreshDevServerCapabilities
	agent    DevServerAgentClient
	ttl      time.Duration
	clock    Clock
}

func NewGetDevServerCapabilities(
	resolver ConnectionResolver,
	store CapabilityProfileStore,
	refresh *RefreshDevServerCapabilities,
	agent DevServerAgentClient,
	ttl time.Duration,
	clock Clock,
) *GetDevServerCapabilities {
	return &GetDevServerCapabilities{
		resolver: resolver,
		store:    store,
		refresh:  refresh,
		agent:    agent,
		ttl:      ttl,
		clock:    clock,
	}
}

func (uc *GetDevServerCapabilities) Execute(ctx context.Context, in GetCapabilitiesInput) (GetCapabilitiesResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return GetCapabilitiesResult{}, err
	}

	if (in.ConnectionID == "" && in.DevServerID == "") || (in.ConnectionID != "" && in.DevServerID != "") {
		return GetCapabilitiesResult{}, apperrors.New(apperrors.KindInvalidArgument, "INFRA_CAPABILITY_BAD_REQUEST", "must provide exactly one of connection_id or dev_server_id", nil)
	}

	devServerID := in.DevServerID
	if in.ConnectionID != "" {
		_, ds, _, err := uc.resolver.ResolveConnection(ctx, tenantID, in.ConnectionID)
		if err != nil {
			return GetCapabilitiesResult{}, err
		}
		devServerID = ds.ID
	}

	connected := uc.agent.IsConnected(devServerID)

	p, ok, err := uc.store.Get(ctx, tenantID, devServerID)
	if err != nil {
		return GetCapabilitiesResult{}, err
	}

	needsRefresh := in.Refresh || !ok
	if ok && !needsRefresh {
		if uc.clock.Now().Sub(p.ProbedAt) > uc.ttl {
			needsRefresh = true
		}
	}

	if connected && needsRefresh {
		// Call refresh
		np, err := uc.refresh.Execute(ctx, tenantID, devServerID, in.Refresh)
		if err == nil {
			p = np
			ok = true
		} else {
			// Log error and fallback to old profile if exists
		}
	}

	if !connected && !ok {
		return GetCapabilitiesResult{}, apperrors.New(apperrors.KindNotFound, "INFRA_CAPABILITY_PROFILE_NOT_FOUND", "profile not found", nil)
	}

	return GetCapabilitiesResult{
		Profile:   p,
		Connected: connected,
		Found:     ok,
	}, nil
}
