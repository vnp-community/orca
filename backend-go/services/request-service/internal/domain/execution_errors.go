package domain

import (
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/common/apperrors"
)

func ErrRequestNotExecuting(status RequestStatus) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_NOT_EXECUTING", fmt.Sprintf("request is %q, not executing", status), nil)
}

func ErrPhaseNotInPlan(phaseID string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_PHASE_NOT_IN_PLAN", fmt.Sprintf("phase %s does not belong to this request's plan", phaseID), nil)
}

func ErrPhaseNotApproved(phaseID string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_PHASE_NOT_APPROVED", fmt.Sprintf("phase %s has no approved phase approval", phaseID), nil)
}

func ErrPhasePredecessorNotDone(phaseID string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_PHASE_PREDECESSOR_NOT_DONE", fmt.Sprintf("a phase that %s depends on is not done", phaseID), nil)
}

func ErrExecuteForbidden() error {
	return apperrors.New(apperrors.KindPermissionDenied, "REQUEST_EXECUTE_FORBIDDEN", "the approving user may not execute this task", nil)
}

func ErrPhaseRequired() error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_PHASE_REQUIRED", "this request is split into phases: phase_task_id is required", nil)
}

func ErrExecutionTaskServiceUnavailable(cause error) error {
	return apperrors.New(apperrors.KindUnavailable, "REQUEST_EXECUTION_TASK_SERVICE_UNAVAILABLE", "task service is unavailable", cause)
}

func ErrBacklogInvalidView() error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_BACKLOG_INVALID_VIEW", "a backlog view is required", nil)
}

func ErrBacklogTaskServiceUnavailable(cause error) error {
	return apperrors.New(apperrors.KindUnavailable, "REQUEST_BACKLOG_TASK_SERVICE_UNAVAILABLE", "task service is unavailable", cause)
}

func ErrCheckNotAllowedNow(kind CheckKind, status RequestStatus, reqType RequestType) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_CHECK_NOT_ALLOWED_NOW",
		fmt.Sprintf("check %q cannot be recorded for a %q request in status %q", kind, reqType, status), nil)
}

func ErrCheckInvalidMetrics(detail string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_CHECK_INVALID_METRICS", detail, nil)
}

func ErrCheckForbidden() error {
	return apperrors.New(apperrors.KindPermissionDenied, "REQUEST_CHECK_FORBIDDEN", "only the request owner or an approver may record checks", nil)
}

func ErrPreDeployRequired(taskID string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_PRE_DEPLOY_REQUIRED", fmt.Sprintf("task %s needs an approved pre_deploy approval", taskID), nil)
}

func ErrPerfBaselineMissing() error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_PERF_BASELINE_MISSING", "a passed perf_baseline check is required before planning", nil)
}

func ErrPlanPerfCheckTasksMissing() error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_PLAN_PERF_CHECK_TASKS_MISSING", "the plan must start with a check:baseline task and end with a check:after task", nil)
}

func ErrPlanTestCheckTasksMissing() error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_PLAN_TEST_CHECK_TASKS_MISSING", "the plan must start with a check:tests_before task and end with a check:tests_after task", nil)
}

func ErrRunbookRollbackMissing(detail string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_RUNBOOK_ROLLBACK_MISSING", detail, nil)
}

func ErrRunbookIrreversibleStepUngated(detail string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_RUNBOOK_IRREVERSIBLE_STEP_UNGATED", detail, nil)
}

func ErrHotfixPlanShape(detail string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_HOTFIX_PLAN_SHAPE", detail, nil)
}

// Task-service failures the execution use cases tell apart; the grpc client maps TASK_* codes onto them.
var (
	ErrTaskAlreadyRunning    = errors.New("domain: task dispatch already in progress")
	ErrTaskDispatchTransient = errors.New("domain: task dispatch failed transiently")
	ErrTaskForbidden         = errors.New("domain: task service denied the acting user")
	ErrTaskNotFoundRemote    = errors.New("domain: task not found in task service")
	// ErrTaskServiceDown wraps transport failures (unavailable, deadline) of any task-service call.
	ErrTaskServiceDown = errors.New("domain: task service is unreachable")
)

// DispatchError carries the task-service error code that stopped a dispatch (for example TASK_EXECUTE_NO_CONNECTION).
type DispatchError struct {
	Code string
	Err  error
}

func (e *DispatchError) Error() string { return fmt.Sprintf("dispatch failed: %s: %v", e.Code, e.Err) }
func (e *DispatchError) Unwrap() error { return e.Err }

func ErrForbiddenToStart() error {
	return apperrors.New(apperrors.KindPermissionDenied, "REQUEST_START_FORBIDDEN", "only the request owner, an admin or a phase approver may start a phase", nil)
}
