package usecase

import "context"

type TenantSettings struct {
	FeatureFlags map[string]bool
}

func GetTenantSettings(ctx context.Context, tenantID string) (TenantSettings, error) {
	return TenantSettings{}, nil
}
