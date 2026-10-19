package usecase

import (
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// taskRepoError classifies a TaskRepository failure: only domain.ErrTaskNotFound is a 404,
// so a database outage is never reported to the caller as "unknown id".
func taskRepoError(err error, notFoundMsg, internalCode, internalMsg string) error {
	if errors.Is(err, domain.ErrTaskNotFound) {
		return apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", notFoundMsg, err)
	}
	return apperrors.New(apperrors.KindInternal, internalCode, internalMsg, err)
}
