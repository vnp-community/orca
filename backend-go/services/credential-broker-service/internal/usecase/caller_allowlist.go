package usecase

import (
	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

// ensureCallerAllowed enforces the category-to-caller allow-list. The error is
// deliberately generic: it must not reveal whether a credential exists.
func ensureCallerAllowed(category domain.Category, requestingService string) error {
	if category.AllowsCaller(requestingService) {
		return nil
	}
	return apperrors.New(apperrors.KindPermissionDenied, "CREDENTIAL_CALLER_NOT_ALLOWED", "caller is not allowed to use this credential category", domain.ErrCallerNotAllowed)
}
