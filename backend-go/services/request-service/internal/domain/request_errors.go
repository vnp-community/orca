package domain

import (
	"fmt"
	"github.com/stablyai/orca-go/common/apperrors"
)

func ErrRequestTenantRequired() error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_TENANT_REQUIRED", "tenant id is required", nil)
}

func ErrRequestReporterRequired() error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_REPORTER_REQUIRED", "reporter id is required", nil)
}

func ErrRequestInvalidType(t string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_INVALID_TYPE", fmt.Sprintf("invalid request type: %s", t), nil)
}

func ErrRequestInvalidStatus(s string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_INVALID_STATUS", fmt.Sprintf("invalid request status: %s", s), nil)
}

func ErrRequestInvalidSize(s string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_INVALID_SIZE", fmt.Sprintf("invalid request size: %s", s), nil)
}

func ErrRequestInvalidUrgency(s string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_INVALID_URGENCY", fmt.Sprintf("invalid request urgency: %s", s), nil)
}

func ErrRequestInvalidSourceProvider(s string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_INVALID_SOURCE_PROVIDER", fmt.Sprintf("invalid source provider: %s", s), nil)
}

func ErrRequestTitleRequired() error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_TITLE_REQUIRED", "request title is required", nil)
}

func ErrRequestNotFound(id string) error {
	return apperrors.New(apperrors.KindNotFound, "REQUEST_NOT_FOUND", fmt.Sprintf("request not found: %s", id), nil)
}

func ErrRequestVersionConflict(id string, expected int64) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_VERSION_CONFLICT", fmt.Sprintf("request %s version conflict (expected %d)", id, expected), nil)
}

func ErrSourceAlreadyExists(existingRequestID string) error {
	return apperrors.New(apperrors.KindAlreadyExists, "REQUEST_SOURCE_ALREADY_EXISTS", fmt.Sprintf("request source already exists: %s", existingRequestID), nil)
}

func ErrRequestLinkSelf() error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_LINK_SELF", "cannot link a request to itself", nil)
}

func ErrSolutionNotFound(id string) error {
	return apperrors.New(apperrors.KindNotFound, "SOLUTION_NOT_FOUND", fmt.Sprintf("solution not found: %s", id), nil)
}

func ErrSolutionVersionConflict(id string, expected int64) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "SOLUTION_VERSION_CONFLICT", fmt.Sprintf("solution %s version conflict (expected %d)", id, expected), nil)
}

func ErrBacklogCategoryUnavailable() error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_BACKLOG_CATEGORY_UNAVAILABLE", "backlog category filter needs the request-lifecycle migration", nil)
}

func ErrBacklogReturnHistoryUnavailable() error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_BACKLOG_RETURN_HISTORY_UNAVAILABLE", "return history needs the request-lifecycle migration", nil)
}
