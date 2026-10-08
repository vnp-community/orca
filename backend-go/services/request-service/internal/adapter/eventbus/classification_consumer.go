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

const (
	requestStream             = "REQUEST"
	classifierDurable         = "request-classifier"
	processedEventsRetention  = 7 * 24 * time.Hour
	processedEventsPruneEvery = 24 * time.Hour
)

// ClassificationTrigger starts classification for a request that entered `classifying`.
// It only enqueues a durable run, so acking the event never depends on the AI call.
type ClassificationTrigger interface {
	Run(ctx context.Context, requestID, eventID, trigger string) error
}

type ClassificationConsumer struct {
	bus     *commoneventbus.Consumer
	trigger ClassificationTrigger
}

func NewClassificationConsumer(bus *commoneventbus.Consumer, trigger ClassificationTrigger) *ClassificationConsumer {
	return &ClassificationConsumer{bus: bus, trigger: trigger}
}

// Run blocks until ctx ends; call it in a goroutine tracked by a WaitGroup. The durable name
// is fixed so replicas share one cursor and each event is handled once cluster-wide.
func (c *ClassificationConsumer) Run(ctx context.Context) error {
	return c.bus.Subscribe(ctx, requestStream, classifierDurable, domain.SubjectRequestStatusChanged, c.Handle)
}

type statusChangedPayload struct {
	RequestID string `json:"request_id"`
	To        string `json:"to"`
	Trigger   string `json:"trigger"`
}

// Handle returns an error only for conditions worth redelivering; malformed or irrelevant
// events are acked so they cannot block the durable cursor.
func (c *ClassificationConsumer) Handle(ctx context.Context, ev commoneventbus.Event) error {
	var p statusChangedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil || p.RequestID == "" {
		slog.WarnContext(ctx, "status_changed event ignored: unreadable payload", slog.String("event_id", ev.ID))
		return nil
	}
	if p.To != string(domain.RequestStatusClassifying) {
		return nil
	}
	if ev.TenantID == "" {
		slog.WarnContext(ctx, "status_changed event ignored: no tenant", slog.String("event_id", ev.ID))
		return nil
	}
	return c.trigger.Run(tenant.WithTenantID(ctx, ev.TenantID), p.RequestID, ev.ID, p.Trigger)
}

var _ ClassificationTrigger = (*usecase.ClassificationRunner)(nil)

// RunPruneLoop deletes processed_events past the redelivery window once per interval.
func RunPruneLoop(ctx context.Context, repo usecase.ProcessedEventRepository, every, retention time.Duration, now func() time.Time) {
	if every <= 0 {
		every = processedEventsPruneEvery
	}
	if retention <= 0 {
		retention = processedEventsRetention
	}
	if now == nil {
		now = time.Now
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		if n, err := repo.Prune(ctx, now().Add(-retention)); err != nil {
			slog.WarnContext(ctx, "processed_events prune failed", slog.Any("error", err))
		} else if n > 0 {
			slog.InfoContext(ctx, "processed_events pruned", slog.Int64("rows", n))
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
