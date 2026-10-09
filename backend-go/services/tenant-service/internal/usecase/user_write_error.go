package usecase

import (
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// userWriteError maps a failed write that carries a caller-supplied user id: an unrepresentable id is
// the caller's mistake (InvalidArgument), anything else is an Internal persistence failure.
func userWriteError(err error, internalCode, internalMessage string) *apperrors.AppError {
	if errors.Is(err, domain.ErrMalformedUserID) {
		return apperrors.New(apperrors.KindInvalidArgument, "TENANT_INVALID_USER_ID", "user_id is not a valid id", err)
	}
	return apperrors.New(apperrors.KindInternal, internalCode, internalMessage, err)
}
