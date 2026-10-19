package usecase

import (
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

// accountRepoError maps a repository failure on an account lookup/mutation: a missing account is
// NotFound (the caller's id names nothing), everything else is Internal so outages are not masked.
func accountRepoError(err error, internalCode, internalMessage string) *apperrors.AppError {
	if errors.Is(err, domain.ErrAccountNotFound) {
		return apperrors.New(apperrors.KindNotFound, "AIPROVIDER_ACCOUNT_NOT_FOUND", "provider account not found", err)
	}
	return apperrors.New(apperrors.KindInternal, internalCode, internalMessage, err)
}
