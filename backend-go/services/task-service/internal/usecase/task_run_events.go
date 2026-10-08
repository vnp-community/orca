package usecase

import (
	"encoding/json"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// RunCause says why a run moved a task's status; request-service keys its
// consumer on it, so the values are a wire contract.
type RunCause string

const (
	CauseExecuteClaim       RunCause = "execute_claim"
	CauseExecutionCompleted RunCause = "execution_completed"
	CauseExecutionFailed    RunCause = "execution_failed"
	CauseRecovery           RunCause = "recovery"
	statusChangedSubject             = "orca.task.task.statuschanged"
	maxRunErrorMessageBytes          = 1024
)

// newRunStatusEvent builds the statuschanged event for a run-driven transition.
// Only request-owned tasks emit: plain tasks must not grow extra outbox rows.
func newRunStatusEvent(t domain.Task, prev, next domain.Status, cause RunCause, linkID string, engine domain.ExecutionEngine, errMsg string, now time.Time) (domain.OutboxEvent, bool) {
	return newRunStatusEventWithOutcome(t, prev, next, cause, linkID, engine, errMsg, now, runOutcome{})
}

// runOutcome carries what only a contract run knows; the zero value adds nothing to the payload.
type runOutcome struct {
	FailureClass, ExecutionRecordID string
}

func newRunStatusEventWithOutcome(t domain.Task, prev, next domain.Status, cause RunCause, linkID string, engine domain.ExecutionEngine, errMsg string, now time.Time, outcome runOutcome) (domain.OutboxEvent, bool) {
	if t.RequestID == "" {
		return domain.OutboxEvent{}, false
	}
	payload, err := json.Marshal(taskStatusChangedPayload{
		TaskID: t.ID, ProjectID: t.ProjectID, WorktreeID: t.WorktreeID,
		PreviousStatus: string(prev), NewStatus: string(next),
		TaskType: t.Type, ParentID: t.ParentID, RequestID: t.RequestID, Cause: string(cause),
		ExecutionLinkID: linkID, Engine: string(engine), ErrorMessage: truncateUTF8(errMsg, maxRunErrorMessageBytes),
		FailureClass: outcome.FailureClass, ExecutionRecordID: outcome.ExecutionRecordID,
	})
	if err != nil {
		return domain.OutboxEvent{}, false // a struct of strings cannot fail to marshal
	}
	return domain.OutboxEvent{ID: uuid.NewString(), Subject: statusChangedSubject, OccurredAt: now.UTC(), PayloadJSON: payload}, true
}

// runEvents wraps newRunStatusEvent into the slice the repository ports take (nil when none).
func runEvents(t domain.Task, prev, next domain.Status, cause RunCause, linkID string, engine domain.ExecutionEngine, errMsg string, now time.Time) []domain.OutboxEvent {
	return runEventsWithOutcome(t, prev, next, cause, linkID, engine, errMsg, now, runOutcome{})
}

func runEventsWithOutcome(t domain.Task, prev, next domain.Status, cause RunCause, linkID string, engine domain.ExecutionEngine, errMsg string, now time.Time, outcome runOutcome) []domain.OutboxEvent {
	if ev, ok := newRunStatusEventWithOutcome(t, prev, next, cause, linkID, engine, errMsg, now, outcome); ok {
		return []domain.OutboxEvent{ev}
	}
	return nil
}

// truncateUTF8 cuts at a rune boundary so the JSON stays valid.
func truncateUTF8(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
