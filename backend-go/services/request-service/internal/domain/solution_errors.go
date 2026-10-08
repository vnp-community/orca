package domain

import (
	"fmt"

	"github.com/stablyai/orca-go/common/apperrors"
)

func ErrSolutionRequestNotFound(id string) error {
	return apperrors.New(apperrors.KindNotFound, "REQUEST_SOLUTION_REQUEST_NOT_FOUND", fmt.Sprintf("request not found: %s", id), nil)
}

func ErrSolutionWrongState(status RequestStatus) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_SOLUTION_WRONG_STATE",
		fmt.Sprintf("request in status %s cannot start analysis (needs analyzing, or awaiting_analysis_approval with feedback)", status), nil)
}

func ErrSolutionKindNotAllowed(t RequestType) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_SOLUTION_KIND_NOT_ALLOWED",
		fmt.Sprintf("request type %q has no analysis step that this call can run", t), nil)
}

func ErrSolutionNoConnection() error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_SOLUTION_NO_CONNECTION", "project has no connected dev server to run the AI call", nil)
}

// ErrRequestSolutionNotFound is the RPC-facing not-found; the repository's SOLUTION_NOT_FOUND stays internal.
func ErrRequestSolutionNotFound(id string) error {
	return apperrors.New(apperrors.KindNotFound, "REQUEST_SOLUTION_NOT_FOUND", fmt.Sprintf("solution not found: %s", id), nil)
}

func ErrSolutionNotProposed(id string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_SOLUTION_NOT_PROPOSED", fmt.Sprintf("solution %s is not proposed", id), nil)
}

func ErrSolutionOptionNotFound(optionID string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_SOLUTION_OPTION_NOT_FOUND", fmt.Sprintf("option %q is not in the solution", optionID), nil)
}

func ErrSolutionOptionNotChosen() error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_SOLUTION_OPTION_NOT_CHOSEN", "choose an option before approving the solution", nil)
}

func ErrSolutionForbidden() error {
	return apperrors.New(apperrors.KindPermissionDenied, "REQUEST_SOLUTION_FORBIDDEN", "only the reporter or an administrator may run or steer analysis", nil)
}

func ErrSolutionFeedbackTooLong() error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_SOLUTION_FEEDBACK_TOO_LONG", "feedback is limited to 2000 characters", nil)
}

func ErrSolutionModeNotAllowed() error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_SOLUTION_MODE_NOT_ALLOWED", "a solution with options is only generated in complete mode", nil)
}

func ErrAnalysisNoConnection() error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_ANALYSIS_NO_CONNECTION", "project has no connected dev server to run the analysis", nil)
}

func ErrAnalysisNoRepoPath() error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_ANALYSIS_NO_REPO_PATH", "the project connection has no repository path", nil)
}

func ErrAnalysisBusy() error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_ANALYSIS_BUSY", "too many read-only analyses are already running for this project", nil)
}

// Codes written to analysis_runs.error_code; they never travel as RPC errors.
const (
	RunErrInvalidOutput     = "REQUEST_SOLUTION_INVALID_OUTPUT"
	RunErrInterrupted       = "REQUEST_SOLUTION_RUN_INTERRUPTED"
	RunErrAIFailed          = "REQUEST_SOLUTION_AI_FAILED"
	RunErrAITimeout         = "REQUEST_SOLUTION_AI_TIMEOUT"
	RunErrNoConnection      = "REQUEST_SOLUTION_NO_CONNECTION"
	RunErrAnalysisInvalid   = "REQUEST_ANALYSIS_INVALID_OUTPUT"
	RunErrAnalysisAgent     = "REQUEST_ANALYSIS_AGENT_FAILED"
	RunErrAnalysisTimeout   = "REQUEST_ANALYSIS_TIMEOUT"
	RunErrRepoModified      = "REQUEST_ANALYSIS_REPO_MODIFIED"
	RunErrAgentTooOld       = "REQUEST_ANALYSIS_AGENT_TOO_OLD"
	RunErrReadonlyNotActive = "REQUEST_ANALYSIS_READONLY_NOT_APPLIED"
	RunErrNoRepoPath        = "REQUEST_ANALYSIS_NO_REPO_PATH"
	RunErrAnalysisNoConn    = "REQUEST_ANALYSIS_NO_CONNECTION"
	RunErrInternal          = "REQUEST_SOLUTION_INTERNAL_ERROR"
)
