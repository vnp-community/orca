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

// testScriptStartDurable is its own durable consumer, so it sees every event instead of splitting them with the other lanes.
const testScriptStartDurable = "request-service-testscript-start"

// TestScriptStarter is *usecase.StartTestScripts.
type TestScriptStarter interface {
	Execute(ctx context.Context, requestID string) (bool, error)
}

var _ TestScriptStarter = (*usecase.StartTestScripts)(nil)

// TestScriptStatusConsumer starts the test-script lane when a Request enters executing and its test specs already finished. When
// they finish later, the test-spec lane's finished hook starts it instead (docs/integrate/TDD-REQ-TESTSCRIPT-001 4.2).
type TestScriptStatusConsumer struct {
	bus     *commoneventbus.Consumer
	starter TestScriptStarter
}

func NewTestScriptStatusConsumer(bus *commoneventbus.Consumer, starter TestScriptStarter) *TestScriptStatusConsumer {
	return &TestScriptStatusConsumer{bus: bus, starter: starter}
}

func (c *TestScriptStatusConsumer) Run(ctx context.Context) error {
	return c.bus.Subscribe(ctx, requestStream, testScriptStartDurable, domain.SubjectRequestStatusChanged, c.Handle)
}

// Handle never asks for a redelivery and never lets a panic of the lane reach the bus: the lane keeps its state on its row and
// the reconcile pass starts what a lost event missed.
func (c *TestScriptStatusConsumer) Handle(ctx context.Context, ev commoneventbus.Event) (err error) {
	var p statusChangedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil || p.RequestID == "" {
		return nil
	}
	if p.To != string(domain.RequestStatusExecuting) || ev.TenantID == "" {
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "test scripts: start panicked", slog.String("event_id", ev.ID), slog.String("request_id", p.RequestID), slog.Any("panic", r))
			err = nil
		}
	}()
	if _, serr := c.starter.Execute(tenant.WithTenantID(ctx, ev.TenantID), p.RequestID); serr != nil {
		slog.WarnContext(ctx, "test scripts: start failed", slog.String("event_id", ev.ID), slog.String("request_id", p.RequestID), slog.Any("error", serr))
	}
	return nil
}
