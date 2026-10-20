package eventbus

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

const planAutoDurable = "request-service-plan-auto-generate"

// PlanAutoEnqueuer turns a Request entering planning into a stored job.
type PlanAutoEnqueuer interface {
	Handle(ctx context.Context, ev usecase.PlanAutoEvent) error
}

var _ PlanAutoEnqueuer = (*usecase.EnqueuePlanAuto)(nil)

// planAutoStatusPayload reads only what the job needs; the other two consumers of the subject keep their own structs.
type planAutoStatusPayload struct {
	RequestID string `json:"request_id"`
	ProjectID string `json:"project_id"`
	To        string `json:"to"`
	Trigger   string `json:"trigger"`
}

// PlanAutoConsumer reads status_changed events with to=planning after an approved Analysis or a type confirmation
// (CR-REQ-044). processed_events is keyed by event id alone, so each event may be marked by one consumer: the filter
// here (to=planning) must stay disjoint from classification (to=classifying) and resume (trigger=information_provided).
// A new consumer for these triggers would have to share this one instead of marking the same event.
type PlanAutoConsumer struct {
	bus      *commoneventbus.Consumer
	enqueuer PlanAutoEnqueuer
	// maxEventAge drops events older than this: a new durable replays the stream's history, and starting paid plans for
	// Requests that have waited in planning for days would be a surprise bill. Zero keeps every event.
	maxEventAge time.Duration
	now         func() time.Time
}

func NewPlanAutoConsumer(bus *commoneventbus.Consumer, enqueuer PlanAutoEnqueuer) *PlanAutoConsumer {
	return &PlanAutoConsumer{bus: bus, enqueuer: enqueuer, now: time.Now}
}

// WithMaxEventAge makes the consumer ignore events older than d; a person starts those plans by hand.
func (c *PlanAutoConsumer) WithMaxEventAge(d time.Duration) *PlanAutoConsumer {
	c.maxEventAge = d
	return c
}

// Run blocks until ctx ends; call it in a goroutine tracked by a WaitGroup.
func (c *PlanAutoConsumer) Run(ctx context.Context) error {
	return c.bus.Subscribe(ctx, requestStream, planAutoDurable, domain.SubjectRequestStatusChanged, c.Handle)
}

// Handle acks malformed or irrelevant events; it returns an error only when redelivery can help.
func (c *PlanAutoConsumer) Handle(ctx context.Context, ev commoneventbus.Event) error {
	var p planAutoStatusPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil || p.RequestID == "" {
		slog.WarnContext(ctx, "status_changed event ignored: unreadable payload", slog.String("event_id", ev.ID))
		return nil
	}
	if p.To != string(domain.RequestStatusPlanning) {
		return nil
	}
	if c.maxEventAge > 0 && !ev.OccurredAt.IsZero() && c.now().Sub(ev.OccurredAt) > c.maxEventAge {
		slog.InfoContext(ctx, "plan auto-generation skipped: the event is older than the start window", slog.String("request_id", p.RequestID))
		return nil
	}
	if p.Trigger != string(domain.TriggerAnalysisApproved) && p.Trigger != string(domain.TriggerTypeConfirmed) {
		return nil
	}
	if ev.TenantID == "" {
		slog.WarnContext(ctx, "status_changed event ignored: no tenant", slog.String("event_id", ev.ID))
		return nil
	}
	err := c.enqueuer.Handle(tenant.WithTenantID(ctx, ev.TenantID), usecase.PlanAutoEvent{
		ID: ev.ID, TenantID: ev.TenantID, RequestID: p.RequestID, ProjectID: p.ProjectID, To: p.To, Trigger: p.Trigger,
	})
	if err != nil {
		// A failure here is redelivered at once; without this line a permanent one loops silently.
		slog.WarnContext(ctx, "plan auto-generation enqueue failed; will be redelivered", slog.String("request_id", p.RequestID), slog.Any("error", err))
	}
	return err
}
