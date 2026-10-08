package domain

import (
	"fmt"

	"github.com/stablyai/orca-go/common/apperrors"
)

func ErrRequestNotClassifiable(status RequestStatus) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_NOT_CLASSIFIABLE", fmt.Sprintf("request in status %s cannot be classified", status), nil)
}

func ErrRequestClassificationLimit() error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_CLASSIFICATION_LIMIT", fmt.Sprintf("classification attempts reached the limit of %d", MaxClassificationAttempts), nil)
}

func ErrRequestTypeRequired() error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_TYPE_REQUIRED", "request type is required", nil)
}

func ErrRequestSizeRequired(t RequestType) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_SIZE_REQUIRED", fmt.Sprintf("size is required for type %s", t), nil)
}

func ErrRequestHotfixRequiresUrgent() error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_HOTFIX_REQUIRES_URGENT", "a hotfix must have urgency urgent", nil)
}

// ErrTypeReasonRequired shares its code with the lifecycle's REQUEST_REASON_REQUIRED so clients see one code.
func ErrTypeReasonRequired(t RequestType) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_REASON_REQUIRED", fmt.Sprintf("a reason is required for type %s", t), nil)
}

func ErrRequestTypeUnchanged(t RequestType) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_TYPE_UNCHANGED", fmt.Sprintf("request is already of type %s", t), nil)
}

func ErrRequestTypeChangeNotAllowed(from, to RequestType) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_TYPE_CHANGE_NOT_ALLOWED", fmt.Sprintf("type %s cannot change to %s", from, to), nil)
}

func ErrRequestTypeChangeUseChild(from, to RequestType) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_TYPE_CHANGE_USE_CHILD", fmt.Sprintf("type %s cannot change to %s; spawn a child request instead", from, to), nil)
}

func ErrRequestTypeChangeBlockedActiveExecution() error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_TYPE_CHANGE_BLOCKED_ACTIVE_EXECUTION", "type cannot change while a task is executing", nil)
}

func ErrRequestActorNotAllowed(kind ActorKind) error {
	return apperrors.New(apperrors.KindPermissionDenied, "REQUEST_ACTOR_NOT_ALLOWED", fmt.Sprintf("actor kind %q cannot perform this action", kind), nil)
}

// ErrRequestTypeActionWrongStatus uses the lifecycle's REQUEST_TRANSITION_NOT_ALLOWED code
// for type actions invoked outside the statuses they are defined for.
func ErrRequestTypeActionWrongStatus(status RequestStatus, action string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_TRANSITION_NOT_ALLOWED", fmt.Sprintf("%s is not allowed in status %s", action, status), nil)
}
