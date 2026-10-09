package usecase

import (
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// devServerLookupError maps a DevServerRepository failure: only the domain
// sentinel is NotFound; any other error is infra and must not look like a
// missing dev server (it used to, which hid DB outages as 404).
func devServerLookupError(err error) error {
	if errors.Is(err, domain.ErrDevServerNotFound) {
		return apperrors.New(apperrors.KindNotFound, "INFRA_DEV_SERVER_NOT_FOUND", "dev server not found for this tenant", err)
	}
	return apperrors.New(apperrors.KindInternal, "INFRA_DEV_SERVER_LOOKUP_FAILED", "failed to look up dev server", err)
}

// ephemeralVmRuntimeError maps an EphemeralVmRuntimeStore failure the same way
// devServerLookupError does; notFoundMsg keeps each caller's existing message.
func ephemeralVmRuntimeError(err error, notFoundMsg string) error {
	if errors.Is(err, domain.ErrEphemeralVmRuntimeNotFound) {
		return apperrors.New(apperrors.KindNotFound, "INFRA_EPHEMERAL_VM_RUNTIME_NOT_FOUND", notFoundMsg, err)
	}
	return apperrors.New(apperrors.KindInternal, "INFRA_EPHEMERAL_VM_RUNTIME_STORE_FAILED", "failed to access ephemeral vm runtime", err)
}
