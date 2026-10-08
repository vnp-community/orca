package domain

import (
	"fmt"

	"github.com/stablyai/orca-go/common/apperrors"
)

func ErrReturnStageInvalid(s string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_RETURN_STAGE_INVALID", fmt.Sprintf("invalid return stage for the request status: %q", s), nil)
}

func ErrReturnCategoryInvalid(s string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_RETURN_CATEGORY_INVALID", fmt.Sprintf("invalid return category: %q", s), nil)
}

func ErrReturnBlockedActiveExecution(id string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_RETURN_BLOCKED_ACTIVE_EXECUTION", fmt.Sprintf("request %s still has running tasks", id), nil)
}

func ErrCancelBlockedActiveExecution(id string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_CANCEL_BLOCKED_ACTIVE_EXECUTION", fmt.Sprintf("request %s still has running tasks", id), nil)
}

func ErrReopenNotAllowed(from RequestStatus) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_REOPEN_NOT_ALLOWED", fmt.Sprintf("only a request in request_backlog can be reopened, not %q", from), nil)
}

func ErrCancelNotAllowed(from RequestStatus) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_CANCEL_NOT_ALLOWED", fmt.Sprintf("a request in %q cannot be cancelled", from), nil)
}

func ErrParentRequestNotFound(id string) error {
	return apperrors.New(apperrors.KindNotFound, "REQUEST_PARENT_NOT_FOUND", fmt.Sprintf("parent request not found: %s", id), nil)
}

func ErrChildNotAllowed(reason string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_CHILD_NOT_ALLOWED", reason, nil)
}

func ErrChildLimit(parentID string, limit int) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_CHILD_LIMIT", fmt.Sprintf("request %s already has %d child requests", parentID, limit), nil)
}

func ErrChildDepthExceeded(limit int) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_CHILD_DEPTH_EXCEEDED", fmt.Sprintf("child requests nest at most %d levels", limit), nil)
}

func ErrClientRequestIDRequired() error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_CLIENT_REQUEST_ID_REQUIRED", "client_request_id is required", nil)
}
