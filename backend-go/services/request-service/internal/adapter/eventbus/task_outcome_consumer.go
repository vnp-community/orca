package eventbus

import (
	"context"
	"encoding/json"
	"log/slog"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

const (
	taskStream          = "TASK"
	taskOutcomeDurable  = "request-service-task-outcome"
	taskStatusChangedOn = "orca.task.task.statuschanged"
)

// TaskOutcomeReporter is *usecase.ReportTaskOutcome.
type TaskOutcomeReporter interface {
	Execute(ctx context.Context, in usecase.ReportTaskOutcomeInput) error
}

var _ TaskOutcomeReporter = (*usecase.ReportTaskOutcome)(nil)

// TaskOutcomeConsumer feeds task-service status events of Request-owned tasks into ReportTaskOutcome.
type TaskOutcomeConsumer struct {
	bus      *commoneventbus.Consumer
	reporter TaskOutcomeReporter
}

func NewTaskOutcomeConsumer(bus *commoneventbus.Consumer, reporter TaskOutcomeReporter) *TaskOutcomeConsumer {
	return &TaskOutcomeConsumer{bus: bus, reporter: reporter}
}

// Run blocks until ctx ends. One durable name shared by every replica makes each event handled once cluster-wide.
func (c *TaskOutcomeConsumer) Run(ctx context.Context) error {
	return c.bus.Subscribe(ctx, taskStream, taskOutcomeDurable, taskStatusChangedOn, c.Handle)
}

// taskStatusChangedPayload is the wire contract with task-service's statuschanged event (additive fields ignored).
type taskStatusChangedPayload struct {
	TaskID          string `json:"task_id"`
	PreviousStatus  string `json:"previous_status"`
	NewStatus       string `json:"new_status"`
	TaskType        string `json:"task_type"`
	ParentID        string `json:"parent_id"`
	RequestID       string `json:"request_id"`
	Cause           string `json:"cause"`
	ExecutionLinkID string `json:"execution_link_id"`
	ErrorMessage    string `json:"error_message"`
}

// Handle returns an error only when a redelivery could help; events of tasks without a Request, unreadable
// payloads and events without a tenant are acked so they cannot stall the durable cursor.
func (c *TaskOutcomeConsumer) Handle(ctx context.Context, ev commoneventbus.Event) error {
	var p taskStatusChangedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		slog.WarnContext(ctx, "task status event ignored: unreadable payload", slog.String("event_id", ev.ID))
		return nil
	}
	if p.RequestID == "" || p.TaskID == "" {
		return nil
	}
	if ev.TenantID == "" {
		slog.WarnContext(ctx, "task status event ignored: no tenant", slog.String("event_id", ev.ID))
		return nil
	}
	return c.reporter.Execute(tenant.WithTenantID(ctx, ev.TenantID), usecase.ReportTaskOutcomeInput{
		EventID: ev.ID, RequestID: p.RequestID, TaskID: p.TaskID, TaskType: p.TaskType, ParentID: p.ParentID,
		PreviousStatus: p.PreviousStatus, NewStatus: p.NewStatus, Cause: p.Cause, ExecutionLinkID: p.ExecutionLinkID,
		ErrorMessage: p.ErrorMessage, OccurredAt: ev.OccurredAt,
	})
}
