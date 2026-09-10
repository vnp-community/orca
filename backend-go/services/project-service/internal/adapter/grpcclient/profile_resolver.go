package grpcclient

import (
	"context"
	"encoding/json"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"

	"github.com/stablyai/orca-go/services/project-service/internal/usecase"
)

// TenantProfileResolver implements usecase.ProfileResolver against
// tenant-service's GetResolvedProfile RPC (fleet.allowedServerTags
// visibility filter) and infra-fleet-service's ListDevServers RPC (a dev
// server's own tags) — dialed the same way InfraFleetDevServerLister/
// InfraFleetHealthChecker already do.
type TenantProfileResolver struct {
	tenant     tenantv1.TenantServiceClient
	infraFleet infrafleetv1.InfraFleetServiceClient
}

func NewTenantProfileResolver(tenant tenantv1.TenantServiceClient, infraFleet infrafleetv1.InfraFleetServiceClient) *TenantProfileResolver {
	return &TenantProfileResolver{tenant: tenant, infraFleet: infraFleet}
}

// resolvedFleetSection is the subset of GetResolvedProfileResponse's
// resolved_settings_json this adapter reads — fleet.allowedServerTags.
type resolvedFleetSection struct {
	Fleet *struct {
		AllowedServerTags *[]string `json:"allowedServerTags"`
	} `json:"fleet"`
}

func (r *TenantProfileResolver) GetResolvedProfile(ctx context.Context, tenantID, userID string) (usecase.ResolvedProfileView, error) {
	resp, err := r.tenant.GetResolvedProfile(ctx, &tenantv1.GetResolvedProfileRequest{UserId: userID})
	if err != nil {
		return usecase.ResolvedProfileView{}, fmt.Errorf("grpcclient: tenant-service GetResolvedProfile: %w", err)
	}
	var decoded resolvedFleetSection
	if err := json.Unmarshal([]byte(resp.GetResolvedSettingsJson()), &decoded); err != nil {
		return usecase.ResolvedProfileView{}, fmt.Errorf("grpcclient: unmarshal resolved_settings_json: %w", err)
	}
	if decoded.Fleet == nil || decoded.Fleet.AllowedServerTags == nil {
		return usecase.NewResolvedProfileView(nil, false), nil
	}
	return usecase.NewResolvedProfileView(*decoded.Fleet.AllowedServerTags, true), nil
}

func (r *TenantProfileResolver) DevServerTags(ctx context.Context, tenantID, devServerID string) ([]string, error) {
	resp, err := r.infraFleet.ListDevServers(ctx, &infrafleetv1.ListDevServersRequest{})
	if err != nil {
		return nil, fmt.Errorf("grpcclient: infra-fleet-service ListDevServers: %w", err)
	}
	for _, ds := range resp.GetDevServers() {
		if ds.GetId() == devServerID {
			return ds.GetTags(), nil
		}
	}
	return nil, nil
}
