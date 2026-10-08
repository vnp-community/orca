package usecase

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// taskStatusChangedSubject is the processed_events subject of task events handled here.
const taskStatusChangedSubject = "orca.task.task.statuschanged"

// ReportTaskOutcomeInput is one task-service statuschanged event (or its RPC twin).
type ReportTaskOutcomeInput struct {
	EventID         string
	RequestID       string
	TaskID          string
	TaskType        string
	ParentID        string
	PreviousStatus  string
	NewStatus       string
	Cause           string
	ExecutionLinkID string
	ErrorMessage    string
	OccurredAt      time.Time
}

// ReportTaskOutcome records what happened to a task and lets the Request react. The write is one short
// transaction; the reaction (calls to task-service, TransitionRequest) happens after commit. If the reaction is
// cut short the event is already consumed, and the reconcile loop re-derives it from task state.
type ReportTaskOutcome struct {
	Requests  RequestReader
	Tasks     TaskClient
	Outcomes  TaskRunOutcomeRepository
	Processed ProcessedEventRepository
	Evaluate  *EvaluateExecution
	Tx        TxRunner
	Outbox    OutboxWriter
	Settings  ExecutionSettings
	Log       *slog.Logger
}

func (uc *ReportTaskOutcome) log() *slog.Logger {
	if uc.Log != nil {
		return uc.Log
	}
	return slog.Default()
}

func (uc *ReportTaskOutcome) Execute(ctx context.Context, in ReportTaskOutcomeInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ErrRequestTenantRequired()
	}
	if in.EventID == "" || in.RequestID == "" || in.TaskID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_OUTCOME_INVALID", "event_id, request_id and task_id are required", nil)
	}
	outcome, ok := domain.ClassifyTaskOutcome(in.TaskType, in.Cause, in.NewStatus)
	if !ok {
		uc.log().Debug("task event outside the outcome table", slog.String("event_id", in.EventID), slog.String("cause", in.Cause), slog.String("status", in.NewStatus))
		return nil
	}
	req, err := uc.Requests.Get(ctx, in.RequestID)
	if err != nil {
		if isNotFound(err) {
			uc.log().Warn("task event for an unknown request ignored", slog.String("event_id", in.EventID), slog.String("request_id", in.RequestID))
			return nil
		}
		return err
	}
	executing := req.Status == domain.RequestStatusExecuting
	taskCount := 0
	if outcome == domain.OutcomePhaseDone && executing {
		// Counted before the transaction: no outbound call while a database transaction is open.
		kids, err := uc.Tasks.ListTasks(ctx, ListTasksQuery{RequestIDs: []string{req.ID}, ParentID: in.TaskID, TaskTypes: executionTaskTypes})
		if err != nil {
			return err
		}
		taskCount = len(kids)
	}
	if in.OccurredAt.IsZero() {
		in.OccurredAt = time.Now().UTC()
	}

	var duplicate bool
	err = uc.Tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		if duplicate, err = uc.Processed.MarkProcessed(ctx, in.EventID, taskStatusChangedSubject); err != nil || duplicate {
			return err
		}
		inserted, err := uc.Outcomes.Insert(ctx, domain.TaskRunOutcome{
			ID: uuid.NewString(), TenantID: tenantID, RequestID: req.ID, TaskID: in.TaskID, ContainerID: in.ParentID, Outcome: outcome,
			Cause: in.Cause, ExecutionLinkID: in.ExecutionLinkID, ErrorMessage: in.ErrorMessage, EventID: in.EventID, OccurredAt: in.OccurredAt.UTC(),
		})
		if err != nil || !inserted || outcome != domain.OutcomePhaseDone || !executing {
			return err
		}
		ev, err := NewOutboxEvent(ctx, domain.SubjectPhaseCompleted, domain.PhaseCompletedPayload{RequestID: req.ID, PhaseTaskID: in.TaskID, TaskCount: taskCount})
		if err != nil {
			return err
		}
		return uc.Outbox.InsertOutboxEvent(ctx, ev)
	})
	if err != nil || duplicate {
		return err
	}
	if !executing {
		return nil // a run that finishes after the Request left executing is only recorded
	}
	if outcome == domain.OutcomeStarted && uc.Settings.normalized().MaxParallelTasks <= 1 {
		return nil // nothing else can start while the only slot is taken
	}
	if err := uc.Evaluate.Run(ctx, req); err != nil {
		uc.log().Warn("reaction to task event failed; reconcile will retry", slog.String("request_id", req.ID),
			slog.String("event_id", in.EventID), slog.String("error", truncateForLog(err.Error())))
	}
	return nil
}

func truncateForLog(s string) string {
	if len(s) > 300 {
		return strings.ToValidUTF8(s[:300], "") + "..."
	}
	return s
}
