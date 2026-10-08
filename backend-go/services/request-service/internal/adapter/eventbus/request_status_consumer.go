package eventbus

import (
	"context"
	"encoding/json"
	"log/slog"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

const (
	executionStartDurable = "request-service-status-start"
	executionGateDurable  = "request-service-execution-gate"
)

// ExecutionStarter is *usecase.StartExecution.
type ExecutionStarter interface {
	Execute(ctx context.Context, requestID string) (bool, error)
}

var _ ExecutionStarter = (*usecase.StartExecution)(nil)

// RequestStatusConsumer starts execution for Requests that need no button (no Phases) when they enter executing.
type RequestStatusConsumer struct {
	bus     *commoneventbus.Consumer
	starter ExecutionStarter
}

func NewRequestStatusConsumer(bus *commoneventbus.Consumer, starter ExecutionStarter) *RequestStatusConsumer {
	return &RequestStatusConsumer{bus: bus, starter: starter}
}

func (c *RequestStatusConsumer) Run(ctx context.Context) error {
	return c.bus.Subscribe(ctx, requestStream, executionStartDurable, domain.SubjectRequestStatusChanged, c.Handle)
}

// Handle redelivers only on errors worth retrying. A repeated delivery is harmless: starting is idempotent.
func (c *RequestStatusConsumer) Handle(ctx context.Context, ev commoneventbus.Event) error {
	var p statusChangedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil || p.RequestID == "" {
		slog.WarnContext(ctx, "status_changed event ignored: unreadable payload", slog.String("event_id", ev.ID))
		return nil
	}
	if p.To != string(domain.RequestStatusExecuting) || ev.TenantID == "" {
		return nil
	}
	_, err := c.starter.Execute(tenant.WithTenantID(ctx, ev.TenantID), p.RequestID)
	return err
}

// GateResumer is *usecase.ResumeAfterGate.
type GateResumer interface {
	Execute(ctx context.Context, requestID string, subject domain.SubjectType) error
}

var _ GateResumer = (*usecase.ResumeAfterGate)(nil)

// ApprovalDecidedConsumer resumes a Request after an execution-time gate (ops_request pre_deploy) is approved.
type ApprovalDecidedConsumer struct {
	bus     *commoneventbus.Consumer
	resumer GateResumer
}

func NewApprovalDecidedConsumer(bus *commoneventbus.Consumer, resumer GateResumer) *ApprovalDecidedConsumer {
	return &ApprovalDecidedConsumer{bus: bus, resumer: resumer}
}

func (c *ApprovalDecidedConsumer) Run(ctx context.Context) error {
	return c.bus.Subscribe(ctx, requestStream, executionGateDurable, domain.SubjectApprovalDecided, c.Handle)
}

func (c *ApprovalDecidedConsumer) Handle(ctx context.Context, ev commoneventbus.Event) error {
	var p domain.ApprovalDecidedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil || p.RequestID == "" {
		return nil
	}
	if p.Decision != string(domain.ApprovalStatusApproved) || p.SubjectType != string(domain.SubjectPreDeploy) || ev.TenantID == "" {
		return nil
	}
	return c.resumer.Execute(tenant.WithTenantID(ctx, ev.TenantID), p.RequestID, domain.SubjectType(p.SubjectType))
}
