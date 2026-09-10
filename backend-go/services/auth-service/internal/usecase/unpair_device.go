package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

// UnpairDevice — BR-MB-04: wipes the shared secret (not just a status
// flag), the enforcement mechanism for ResolveDeviceSharedSecret never
// again returning it, not a housekeeping choice.
type UnpairDevice struct {
	devices PairedDeviceRepository
}

func NewUnpairDevice(devices PairedDeviceRepository) *UnpairDevice {
	return &UnpairDevice{devices: devices}
}

func (uc *UnpairDevice) Execute(ctx context.Context, deviceID string) error {
	if err := uc.devices.RevokeAndWipeSecret(ctx, deviceID); err != nil {
		return apperrors.New(apperrors.KindInternal, "AUTH_UNPAIR_FAILED", "failed to revoke paired device", err)
	}
	return nil
}
