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

const clarificationResumeDurable = "request-service-clarification-resume"

// ClarificationResumer restarts the stage that waited for information.
type ClarificationResumer interface {
	Handle(ctx context.Context, ev usecase.ResumeEvent) error
}

var _ ClarificationResumer = (*usecase.ResumeAfterClarification)(nil)

// ClarificationResumeConsumer reads status_changed events with trigger information_provided. It has its own
// durable so it never competes with the classification consumer for the same subject.
type ClarificationResumeConsumer struct {
	bus     *commoneventbus.Consumer
	resumer ClarificationResumer
}

func NewClarificationResumeConsumer(bus *commoneventbus.Consumer, resumer ClarificationResumer) *ClarificationResumeConsumer {
	return &ClarificationResumeConsumer{bus: bus, resumer: resumer}
}

// Run blocks until ctx ends; call it in a goroutine tracked by a WaitGroup.
func (c *ClarificationResumeConsumer) Run(ctx context.Context) error {
	return c.bus.Subscribe(ctx, requestStream, clarificationResumeDurable, domain.SubjectRequestStatusChanged, c.Handle)
}

// Handle acks malformed or irrelevant events; it returns an error only when redelivery can help.
func (c *ClarificationResumeConsumer) Handle(ctx context.Context, ev commoneventbus.Event) error {
	var p statusChangedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil || p.RequestID == "" {
		slog.WarnContext(ctx, "status_changed event ignored: unreadable payload", slog.String("event_id", ev.ID))
		return nil
	}
	if p.Trigger != string(domain.TriggerInformationProvided) {
		return nil
	}
	if ev.TenantID == "" {
		slog.WarnContext(ctx, "status_changed event ignored: no tenant", slog.String("event_id", ev.ID))
		return nil
	}
	return c.resumer.Handle(tenant.WithTenantID(ctx, ev.TenantID), usecase.ResumeEvent{
		ID: ev.ID, TenantID: ev.TenantID, RequestID: p.RequestID, To: p.To, Trigger: p.Trigger,
	})
}
