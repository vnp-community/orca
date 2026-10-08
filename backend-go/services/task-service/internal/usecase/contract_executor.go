package usecase

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// ContractExecuteInput is one contract run. The digests come from request-service when it knows
// them; task-service leaves them empty and request-service joins on (task_id, attempt).
type ContractExecuteInput struct {
	TenantID, TaskID, RequestID, WorktreePath, Prompt, ResultNonce string
	ExecutionLinkID                                                string
	Attempt                                                        int
	SpecDigest, PacketDigest, TemplateVersion                      string
}

type ContractExecuteOutput struct {
	ExecutionRef string
	Record       domain.ExecutionRecord
}

// ContractAgentExecutor runs a task with a caller-supplied prompt and a result nonce, and records the outcome.
type ContractAgentExecutor interface {
	ExecuteWithContract(ctx context.Context, in ContractExecuteInput) (ContractExecuteOutput, error)
}

// ExecutionFailure is the typed error of a run that did not end in a clean `done` result.
type ExecutionFailure struct {
	Class    domain.FailureClass
	Code     string
	RecordID string
	Cause    error
}

func (e *ExecutionFailure) Error() string {
	msg := "execution failed: " + string(e.Class) + " " + e.Code
	if e.Cause != nil {
		msg += ": " + e.Cause.Error()
	}
	return msg
}

func (e *ExecutionFailure) Unwrap() error { return e.Cause }

// WithContract enables the contract path: a task with a spec runs on Engine 1 and, when the
// caller sent a result nonce, through ExecuteWithContract. nil arguments keep the old behavior.
func (uc *ExecuteTask) WithContract(c ContractAgentExecutor, specs TaskSpecLookup) *ExecuteTask {
	uc.contract, uc.specs = c, specs
	return uc
}

// parseAttempt reads the trailing attempt of "req:<request_id>:<task_id>:<attempt>"; anything else is attempt 1.
func parseAttempt(requestID string) int {
	i := strings.LastIndex(requestID, ":")
	if i < 0 {
		return 1
	}
	n, err := strconv.Atoi(requestID[i+1:])
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// runContractAgent is dispatchDirectAgentAsync's body for a contract run. A run ends in review
// only with parse_status=ok and result.status=done; everything else reverts the task so the
// caller (request-service) decides, from failure_class, whether to retry.
func (uc *ExecuteTask) runContractAgent(ctx context.Context, tenantID string, in ExecuteTaskInput, task domain.Task, worktreePath, linkID string, previousStatus domain.Status, dispatchStart time.Time) {
	out, err := uc.contract.ExecuteWithContract(ctx, ContractExecuteInput{
		TenantID: tenantID, TaskID: in.TaskID, RequestID: in.RequestID, WorktreePath: worktreePath, Prompt: in.Prompt,
		ResultNonce: in.ResultNonce, ExecutionLinkID: linkID, Attempt: in.Attempt,
	})
	if err != nil {
		_ = uc.links.Complete(ctx, tenantID, linkID, "failed")
		var outcome runOutcome
		var failure *ExecutionFailure
		if errors.As(err, &failure) {
			outcome = runOutcome{FailureClass: string(failure.Class), ExecutionRecordID: failure.RecordID}
		}
		uc.revertDispatchWithOutcome(ctx, tenantID, task, linkID, previousStatus, domain.EngineDirectAgent, err, outcome)
		slog.WarnContext(ctx, "task: contract execution failed",
			slog.String("task_id", in.TaskID), slog.String("failure_class", outcome.FailureClass), slog.String("error", err.Error()))
		return
	}
	if out.ExecutionRef != "" {
		_ = uc.links.SetExternalRef(ctx, tenantID, linkID, out.ExecutionRef)
	}
	now := uc.clock.Now()
	events := runEventsWithOutcome(task, domain.StatusInProgress, domain.StatusReview, CauseExecutionCompleted, linkID, domain.EngineDirectAgent, "", now, runOutcome{ExecutionRecordID: out.Record.ID})
	if err := uc.repo.CompleteExecution(ctx, tenantID, in.TaskID, string(domain.StatusReview), now.Sub(dispatchStart).Hours(), events); err != nil {
		_ = uc.links.Complete(ctx, tenantID, linkID, "failed")
		slog.WarnContext(ctx, "task: contract completion write failed", slog.String("task_id", in.TaskID), slog.Any("completion_error", err))
		return
	}
	syncContainerParent(ctx, uc.sync, in.TaskID)
	_ = uc.links.Complete(ctx, tenantID, linkID, "completed")
}
