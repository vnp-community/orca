package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// ResolveDeviceSharedSecret is internal-only — never routed by
// api-gateway's REST facade (see auth.proto's RPC doc comment). The row's
// ciphertext being NULL post-unpair (BR-MB-04) IS the enforcement point:
// an error here, never a stale secret.
type ResolveDeviceSharedSecret struct {
	devices PairedDeviceRepository
	sealer  SharedSecretSealer
	clock   Clock
}

func NewResolveDeviceSharedSecret(devices PairedDeviceRepository, sealer SharedSecretSealer, clock Clock) *ResolveDeviceSharedSecret {
	return &ResolveDeviceSharedSecret{devices: devices, sealer: sealer, clock: clock}
}

func (uc *ResolveDeviceSharedSecret) Execute(ctx context.Context, deviceID string) ([]byte, error) {
	device, err := uc.devices.Get(ctx, deviceID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindNotFound, "AUTH_DEVICE_NOT_FOUND", "paired device not found", domain.ErrDeviceNotFound)
	}
	if device.Status != domain.DeviceActive || device.SharedSecretCiphertext == nil {
		return nil, apperrors.New(apperrors.KindNotFound, "AUTH_DEVICE_NOT_FOUND", "paired device not found", domain.ErrDeviceNotFound)
	}
	secret, err := uc.sealer.Decrypt(ctx, device.SharedSecretCiphertext, device.VaultKeyRef)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "AUTH_RESOLVE_SECRET_FAILED", "failed to unseal device shared secret", err)
	}
	_ = uc.devices.Touch(ctx, device.ID, uc.clock.Now()) // best-effort; a Touch failure must not fail the caller's decrypt
	return secret, nil
}
