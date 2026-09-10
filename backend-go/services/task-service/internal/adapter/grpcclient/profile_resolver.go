package grpcclient

import (
	"context"
	"encoding/json"
	"fmt"

	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
)

// ProfileResolver implements usecase.ProfileResolver by dialing
// tenant-service's GetResolvedProfile RPC directly.
type ProfileResolver struct {
	client tenantv1.TenantServiceClient
}

func NewProfileResolver(client tenantv1.TenantServiceClient) *ProfileResolver {
	return &ProfileResolver{client: client}
}

func (r *ProfileResolver) GetResolvedProfile(ctx context.Context, userID string) (map[string]any, error) {
	// tenant-service's GetResolvedProfile usecase calls tenant.RequireTenantID
	// against its own inbound-interceptor-populated context — forward the
	// caller's tenant as outbound metadata, same as every other call this
	// package makes (see tenant_forwarding.go's doc comment).
	outCtx, err := withTenantMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("grpcclient: profile resolver: %w", err)
	}
	resp, err := r.client.GetResolvedProfile(outCtx, &tenantv1.GetResolvedProfileRequest{UserId: userID})
	if err != nil {
		return nil, fmt.Errorf("grpcclient: tenant-service GetResolvedProfile: %w", err)
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(resp.GetResolvedSettingsJson()), &settings); err != nil {
		return nil, fmt.Errorf("grpcclient: unmarshal resolved_settings_json: %w", err)
	}
	return settings, nil
}
