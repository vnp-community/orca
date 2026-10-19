package domain

import (
	"fmt"

	"github.com/stablyai/orca-go/common/apperrors"
)

// ErrSolutionRevisionInProgress answers a second change request that lost the race to another one: the analysis is already
// running again, so the UI must refresh instead of believing its feedback was applied. It keeps the wrong-state code the
// clients already handle.
func ErrSolutionRevisionInProgress(status RequestStatus) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_SOLUTION_WRONG_STATE",
		fmt.Sprintf("request in status %s is already being re-analysed; reload and request changes again once the new proposal is ready", status), nil)
}
