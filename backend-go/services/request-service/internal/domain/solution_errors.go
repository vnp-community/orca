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
	return apperrors.New(apperrors.KindPermissionDenied, "REQUEST_SOLUTION_FORBIDDEN", "only the reporter, an administrator or an approver of the pending decision may run or steer analysis", nil)
}

func ErrSolutionFeedbackTooLong() error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_SOLUTION_FEEDBACK_TOO_LONG", "feedback is limited to 2000 characters", nil)
}

func ErrSolutionModeNotAllowed() error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_SOLUTION_MODE_NOT_ALLOWED", "a solution with options is generated in complete mode unless the project turned on solution_agent", nil)
}

func ErrAnalysisNoConnection() error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_ANALYSIS_NO_CONNECTION", "project has no connected dev server to run the analysis", nil)
}

func ErrAnalysisNoRepoPath() error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_ANALYSIS_NO_REPO_PATH", "the project's default repository has no folder path on its dev server (its URL is not an absolute path), so the agent has no directory to read", nil)
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
	// Plan runs of the openspec engine.
	RunErrSpecOutOfScope    = "REQUEST_SPEC_RUN_OUT_OF_SCOPE"
	RunErrSpecAgentOld      = "REQUEST_ENGINE_AGENT_UNSUPPORTED"
	RunErrSpecAgentFailed   = "REQUEST_OPENSPEC_AGENT_FAILED"
	RunErrSpecTimeout       = "REQUEST_OPENSPEC_TIMEOUT"
	RunErrSpecInvalidOutput = "REQUEST_OPENSPEC_INVALID_OUTPUT"
	// RunErrPlanRevisionNoChange: a revision run ended with every document as it was.
	RunErrPlanRevisionNoChange = "REQUEST_PLAN_REVISION_NO_CHANGE"
	// RunErrBaseRefNotFound: the Request's base ref no longer exists in the repository, so no worktree could be made from it.
	RunErrBaseRefNotFound = "REQUEST_BASE_REF_NOT_FOUND"
	// Hotfix only: the fix task or its pre_deploy approval could not be created when the diagnosis finished.
	RunErrHotfixFixTask  = "REQUEST_HOTFIX_FIX_TASK_FAILED"
	RunErrHotfixApproval = "REQUEST_HOTFIX_APPROVAL_FAILED"
)
