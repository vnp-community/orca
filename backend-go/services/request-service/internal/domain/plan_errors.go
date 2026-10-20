package domain

import (
	"fmt"

	"github.com/stablyai/orca-go/common/apperrors"
)

func planInvalid(code, msg string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, code, msg, nil)
}

func ErrPlanNotApplicable(t RequestType) error {
	return planInvalid("REQUEST_PLAN_NOT_APPLICABLE", fmt.Sprintf("request type %q has no Plan", t))
}

func ErrPlanWrongState(status RequestStatus) error {
	return planInvalid("REQUEST_STATE_STALE", fmt.Sprintf("request in status %s cannot generate or commit a Plan (needs planning)", status))
}

func ErrPlanSolutionNotApproved(id string) error {
	return planInvalid("REQUEST_PLAN_SOLUTION_NOT_APPROVED", fmt.Sprintf("request %s has no approved Solution to plan from", id))
}

func ErrPlanPhasesNotAllowed() error {
	return planInvalid("REQUEST_PLAN_PHASES_NOT_ALLOWED", "this request type and size is planned without Phases")
}

func ErrPlanPhasesRequired() error {
	return planInvalid("REQUEST_PLAN_PHASES_REQUIRED", "this request type and size is planned in Phases; the proposal has none")
}

func ErrPlanMixedChildren() error {
	return planInvalid("REQUEST_PLAN_MIXED_CHILDREN", "a Plan holds Phases or tasks directly, not both")
}

func ErrPlanEmpty(where string) error {
	return planInvalid("REQUEST_PLAN_EMPTY", fmt.Sprintf("%s has no tasks", where))
}

func ErrPlanTooLarge(detail string) error {
	return planInvalid("REQUEST_PLAN_TOO_LARGE", detail)
}

func ErrPlanInvalidDependency(detail string) error {
	return planInvalid("REQUEST_PLAN_INVALID_DEPENDENCY", detail)
}

func ErrPlanRegressionTestMissing() error {
	return planInvalid("REQUEST_PLAN_REGRESSION_TEST_MISSING", "a bug or security fix plan needs a task labelled "+PlanLabelTestRegression)
}

func ErrPlanInvalidTaskType(t string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_PLAN_INVALID_TASK_TYPE", fmt.Sprintf("task_type %q is not one of task, bug, feature, epic", t), nil)
}

func ErrPlanInvalidTitle(detail string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_PLAN_INVALID_TITLE", detail, nil)
}

func ErrPlanInvalidField(detail string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_PLAN_INVALID_FIELD", detail, nil)
}

// ErrPlanAIInvalidJSON is what a person sees when the model's answer is not a usable Plan; the raw text stays on the run.
func ErrPlanAIInvalidJSON(detail string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_PLAN_AI_INVALID_JSON", detail, nil)
}

func ErrPlanRunNotFound(id string) error {
	return apperrors.New(apperrors.KindNotFound, "REQUEST_PLAN_RUN_NOT_FOUND", fmt.Sprintf("plan run not found: %s", id), nil)
}

func ErrPlanTaskServiceUnavailable(cause error) error {
	return apperrors.New(apperrors.KindUnavailable, "REQUEST_PLAN_TASK_SERVICE_UNAVAILABLE", "task-service could not store the Plan", cause)
}

func ErrPlanForbidden() error {
	return apperrors.New(apperrors.KindPermissionDenied, "REQUEST_PLAN_FORBIDDEN", "only the reporter, an administrator or an approver of the pending decision may ask for a Plan or changes to it", nil)
}

// ErrPlanIdempotencyKeyInvalid refuses a key wider than analysis_runs.idempotency_key.
func ErrPlanIdempotencyKeyInvalid() error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_PLAN_IDEMPOTENCY_KEY_INVALID", fmt.Sprintf("the idempotency key is limited to %d characters", PlanAutoKeyMaxLen), nil)
}

// ErrPlanRevisionInProgress: somebody else's request for changes is already being written; asking again would start a
// second run for the same Plan.
func ErrPlanRevisionInProgress() error {
	return planInvalid("REQUEST_PLAN_REVISION_IN_PROGRESS", "another request for changes to this Plan is already being worked on")
}

// ErrPlanRevisionNotApplicable: a hotfix has a single fix task made from its diagnosis, not a Plan to revise.
func ErrPlanRevisionNotApplicable() error {
	return planInvalid("REQUEST_PLAN_REVISION_NOT_APPLICABLE", "this request type has no Plan documents to change")
}

func ErrPlanRevisionNotFound(revision int) error {
	return apperrors.New(apperrors.KindNotFound, "REQUEST_PLAN_REVISION_NOT_FOUND", fmt.Sprintf("this Plan has no version %d", revision), nil)
}

func ErrPlanRevisionPageTokenInvalid() error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_PLAN_REVISION_PAGE_TOKEN_INVALID", "the page token is not one this list returned", nil)
}

// ErrPlanRevisionTextPruned: only the newest versions keep the text of their documents; older ones keep the file list.
func ErrPlanRevisionTextPruned() error {
	return planInvalid("REQUEST_PLAN_REVISION_TEXT_PRUNED", "the text of this version was dropped to bound storage; only the current and the newest versions keep it")
}

func ErrPlanFeedbackTooLong() error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_PLAN_FEEDBACK_TOO_LONG", "feedback is limited to 2000 characters", nil)
}

func ErrPlanAIUnavailable() error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_PLAN_AI_UNAVAILABLE", "project has no connected dev server to run the AI call that writes the Plan", nil)
}

func ErrPlanHasRunningTasks() error {
	return planInvalid("REQUEST_PLAN_HAS_RUNNING_TASKS", "the current Plan has running tasks and cannot be replaced")
}

func ErrPlanSupersedeMismatch() error {
	return planInvalid("REQUEST_PLAN_SUPERSEDE_MISMATCH", "the Request's recorded Plan is not the active Plan in task-service")
}

func ErrPlanNotCommitted(requestID string) error {
	return planInvalid("REQUEST_PLAN_NOT_COMMITTED", "request "+requestID+" has no committed Plan to approve")
}

func ErrPlanBundleNotFound(requestID string) error {
	return apperrors.New(apperrors.KindNotFound, "REQUEST_PLAN_BUNDLE_NOT_FOUND", "request "+requestID+" has no authored OpenSpec change", nil)
}

func ErrPlanBundlePathInvalid(path string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_PLAN_BUNDLE_PATH_INVALID", "the path must name a file inside the change: "+path, nil)
}

func ErrPlanBundleFileNotFound(path string) error {
	return apperrors.New(apperrors.KindNotFound, "REQUEST_PLAN_BUNDLE_FILE_NOT_FOUND", "the change has no file "+path, nil)
}

// ErrProposalRunActive refuses a cleanup while a plan run writes in the proposal worktree.
func ErrProposalRunActive(requestID string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_PROPOSAL_RUN_ACTIVE", "a plan run of this Request is still writing in the proposal worktree; wait for it to finish", nil)
}
