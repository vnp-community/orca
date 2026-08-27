# TASK-MB-01-06: Implement `ListPairedDevices`/`UnpairDevice`/`ResolveDeviceSharedSecret` usecases

**From Solution:** SOL-MB-01
**Priority:** P0
**Service:** `auth-service`
**File:** `backend-go/services/auth-service/internal/usecase/list_paired_devices.go`, `backend-go/services/auth-service/internal/usecase/unpair_device.go`, `backend-go/services/auth-service/internal/usecase/resolve_device_shared_secret.go`
**Depends on:** TASK-MB-01-02, TASK-MB-01-03, TASK-MB-01-04
**Status:** `[ ]` TODO

---

## Context

BR-MB-04 requires `UnpairDevice` to wipe the shared secret (not just flag
revoked) so `ResolveDeviceSharedSecret` — the internal-only RPC every other
service uses to decrypt a device's E2E payloads — can never again return it,
even from a stale cache.

## Changes to make

`backend-go/services/auth-service/internal/usecase/list_paired_devices.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

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
	userID := tenant.UserIDFromContext(ctx)
	return uc.devices.List(ctx, tenantID, userID)
}
```

`backend-go/services/auth-service/internal/usecase/unpair_device.go`:

```go
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
```

`backend-go/services/auth-service/internal/usecase/resolve_device_shared_secret.go`:

```go
package usecase

import (
	"context"
	"time"

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
}

func NewResolveDeviceSharedSecret(devices PairedDeviceRepository, sealer SharedSecretSealer) *ResolveDeviceSharedSecret {
	return &ResolveDeviceSharedSecret{devices: devices, sealer: sealer}
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
	_ = uc.devices.Touch(ctx, device.ID, time.Now()) // best-effort; a Touch failure must not fail the caller's decrypt
	return secret, nil
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/auth-service/... && go vet ./services/auth-service/...
go test ./services/auth-service/internal/usecase/... -run 'ListPairedDevices|UnpairDevice|ResolveDeviceSharedSecret'
```

Test cases: `UnpairDevice` then `ResolveDeviceSharedSecret` on the same
device → `AUTH_DEVICE_NOT_FOUND`, and assert the fake `SharedSecretSealer.Decrypt`
was NOT called (the nulled ciphertext is checked before any decrypt attempt).
