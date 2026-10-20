package domain

import (
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
)

// ParseApprovalExecutionMode reads the mode an approver picks while deciding. Only "continuous" is a choice: step by
// step is what an approval without a choice already does, and a project that wants it sets it in its settings, so
// ParseExecutionMode (which accepts both) is the wrong parser here.
func ParseApprovalExecutionMode(raw string) (ExecutionMode, error) {
	switch ExecutionMode(strings.ToLower(strings.TrimSpace(raw))) {
	case "":
		return "", nil
	case ExecutionContinuous:
		return ExecutionContinuous, nil
	}
	return "", ErrExecutionModeInvalid(raw)
}

// ExecutionModeChoiceApplies is true for the approvals that start a run: the one decision that makes the Request enter
// executing. Precheck and DecideApproval share it so "which approval may carry a mode" has one definition.
func ExecutionModeChoiceApplies(a Approval) bool {
	if a.Stage != string(RequestStatusAwaitingPlanApproval) {
		return false
	}
	switch a.SubjectType {
	case SubjectPlan, SubjectTaskList, SubjectPreDeploy:
		return true
	}
	return false
}

func ErrExecutionModeNotApplicable() error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_EXECUTION_MODE_NOT_APPLICABLE",
		"an execution mode can only be chosen when approving the plan that starts the run", nil)
}

// ErrExecutionModeAlreadyPinned refuses the decision rather than dropping the choice: the approver confirmed that the
// run would go continuously, so quietly approving a step by step run would break that promise.
func ErrExecutionModeAlreadyPinned(pinned ExecutionMode) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_EXECUTION_MODE_ALREADY_PINNED",
		"this request already runs in "+string(pinned)+" mode and the mode cannot be changed", nil)
}
