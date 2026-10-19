package usecase

import (
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
)

// isNotFoundKind reports any repository miss. Callers that re-label a miss for the RPC (for example a
// clarification whose request is gone) must use it so a database failure is not turned into NotFound.
func isNotFoundKind(err error) bool {
	var ae *apperrors.AppError
	return errors.As(err, &ae) && ae.Kind == apperrors.KindNotFound
}
