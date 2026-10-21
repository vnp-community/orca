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

// testSpecStartDurable is its own durable consumer: it must not share the one of StartExecution, or the two would split the
// events between them instead of each seeing all of them.
const testSpecStartDurable = "request-service-testspec-start"

// TestSpecStarter is *usecase.StartTestSpecs.
type TestSpecStarter interface {
	Execute(ctx context.Context, requestID string) (bool, error)
}

var _ TestSpecStarter = (*usecase.StartTestSpecs)(nil)

// TestSpecStatusConsumer starts the test-spec lane when a Request enters planning, the moment Plan generation begins, so both run
// side by side. Entering executing starts it too for a Request that skipped planning or whose first event was lost (the start is
// idempotent). It listens to the same event as the execution starter and shares nothing else with it.
type TestSpecStatusConsumer struct {
	bus     *commoneventbus.Consumer
	starter TestSpecStarter
}

func NewTestSpecStatusConsumer(bus *commoneventbus.Consumer, starter TestSpecStarter) *TestSpecStatusConsumer {
	return &TestSpecStatusConsumer{bus: bus, starter: starter}
}

func (c *TestSpecStatusConsumer) Run(ctx context.Context) error {
	return c.bus.Subscribe(ctx, requestStream, testSpecStartDurable, domain.SubjectRequestStatusChanged, c.Handle)
}

// Handle never asks for a redelivery: the lane keeps its own state on a row and the reconcile pass starts what a lost event
// missed, so a failure here is logged and acknowledged. A panic of the lane must not reach the bus either.
func (c *TestSpecStatusConsumer) Handle(ctx context.Context, ev commoneventbus.Event) (err error) {
	var p statusChangedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil || p.RequestID == "" {
		return nil
	}
	if !startsTestSpecLane(p.To) || ev.TenantID == "" {
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "test specs: start panicked", slog.String("event_id", ev.ID), slog.String("request_id", p.RequestID), slog.Any("panic", r))
			err = nil
		}
	}()
	if _, serr := c.starter.Execute(tenant.WithTenantID(ctx, ev.TenantID), p.RequestID); serr != nil {
		slog.WarnContext(ctx, "test specs: start failed", slog.String("event_id", ev.ID), slog.String("request_id", p.RequestID), slog.Any("error", serr))
	}
	return nil
}

func startsTestSpecLane(to string) bool {
	return to == string(domain.RequestStatusPlanning) || to == string(domain.RequestStatusExecuting)
}
