package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// ListPairedDevices returns the caller's own paired devices (identity from
// context — never a request field, same convention as
// InitiateDevicePairing).
type ListPairedDevices struct {
	devices PairedDeviceRepository
}

func NewListPairedDevices(devices PairedDeviceRepository) *ListPairedDevices {
	return &ListPairedDevices{devices: devices}
}

func (uc *ListPairedDevices) Execute(ctx context.Context) ([]domain.PairedDevice, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "AUTH_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "AUTH_NO_ACTOR", "no authenticated user in request context", nil)
	}
	devices, err := uc.devices.List(ctx, tenantID, userID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "AUTH_LIST_DEVICES_FAILED", "failed to list paired devices", err)
	}
	return devices, nil
}
