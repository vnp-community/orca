package domain

import (
	"fmt"

	"github.com/stablyai/orca-go/common/apperrors"
)

func ErrFlowUnknownType(t RequestType) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_FLOW_UNKNOWN_TYPE", fmt.Sprintf("no flow registered for request type %q", t), nil)
}

func ErrTransitionNotAllowed(from RequestStatus, trigger Trigger) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_TRANSITION_NOT_ALLOWED",
		fmt.Sprintf("trigger %q is not allowed from status %q", trigger, from), nil)
}

func ErrStateStale(expected, actual RequestStatus) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_STATE_STALE",
		fmt.Sprintf("request is in status %q, caller expected %q", actual, expected), nil)
}

func ErrTypeNotSet() error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_TYPE_NOT_SET", "request type must be set before this transition", nil)
}

func ErrReasonRequired() error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_REASON_REQUIRED", "a reason is required for this transition", nil)
}

func ErrResumeStatusInvalid(s RequestStatus) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_RESUME_STATUS_INVALID",
		fmt.Sprintf("resume status %q must be analyzing, planning or executing", s), nil)
}

func newNotFound(code, msg string) error {
	return apperrors.New(apperrors.KindNotFound, code, msg, nil)
}
