package eventbus

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
)

// AllowedSubjects lists the codeintel domain events consumed for cluster-wide push fan-out.
var AllowedSubjects = []string{
	"orca.codeintel.index.changed",
	"orca.codeintel.reindex.finished",
	"orca.codeintel.quality.gate_changed",
}

// PushPublisher is satisfied by CodeIntelPushBroadcaster.
type PushPublisher interface {
	Publish(push *codeintelv1.CodeIntelPush)
}

// EphemeralSubscriber abstracts NATS JetStream SubscribeEphemeral.
type EphemeralSubscriber interface {
	SubscribeEphemeral(ctx context.Context, streamName, subject string, fn commoneventbus.Handler) error
}

// eventLRU is a bounded, thread-safe deduplication cache for event IDs (capacity 1024).
type eventLRU struct {
	mu       sync.Mutex
	capacity int
	entries  map[string]struct{}
	order    []string
}

func newEventLRU(capacity int) *eventLRU {
	if capacity <= 0 {
		capacity = 1024
	}
	return &eventLRU{
		capacity: capacity,
		entries:  make(map[string]struct{}, capacity),
		order:    make([]string, 0, capacity),
	}
}

// checkAndAdd returns true if the eventID was already present (duplicate).
// Otherwise, it records the ID and returns false.
func (l *eventLRU) checkAndAdd(id string) bool {
	if id == "" {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, exists := l.entries[id]; exists {
		return true
	}

	if len(l.order) >= l.capacity {
		oldest := l.order[0]
		l.order = l.order[1:]
		delete(l.entries, oldest)
	}

	l.entries[id] = struct{}{}
	l.order = append(l.order, id)
	return false
}

// CodeIntelConsumer handles ephemeral stream subscriptions and fans out events to local subscribers.
type CodeIntelConsumer struct {
	subscriber  EphemeralSubscriber
	broadcaster PushPublisher
	lru         *eventLRU
	logger      *slog.Logger
}

// NewCodeIntelConsumer creates a new ephemeral NATS event consumer with an LRU cache of 1024 entries.
func NewCodeIntelConsumer(sub EphemeralSubscriber, broadcaster PushPublisher, logger *slog.Logger) *CodeIntelConsumer {
	if logger == nil {
		logger = slog.Default()
	}
	return &CodeIntelConsumer{
		subscriber:  sub,
		broadcaster: broadcaster,
		lru:         newEventLRU(1024),
		logger:      logger,
	}
}

// Start begins ephemeral subscriptions for all codeintel event subjects.
// It blocks until the context is cancelled or a non-recoverable error occurs.
func (c *CodeIntelConsumer) Start(ctx context.Context) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(AllowedSubjects))

	for _, subj := range AllowedSubjects {
		wg.Add(1)
		subject := subj
		go func() {
			defer wg.Done()
			err := c.subscriber.SubscribeEphemeral(ctx, "CODEINTEL", subject, func(hCtx context.Context, ev commoneventbus.Event) error {
				return c.handleEvent(hCtx, subject, ev)
			})
			if err != nil && ctx.Err() == nil {
				c.logger.WarnContext(ctx, "codeintel consumer: subscription stopped with error",
					slog.String("subject", subject), slog.Any("error", err))
				select {
				case errCh <- err:
				default:
				}
			}
		}()
	}

	<-ctx.Done()
	wg.Wait()
	close(errCh)

	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

func (c *CodeIntelConsumer) handleEvent(ctx context.Context, subject string, ev commoneventbus.Event) error {
	// 1. Drop if TenantID is empty per specification
	if ev.TenantID == "" {
		c.logger.DebugContext(ctx, "codeintel consumer: skipping event without tenant_id", slog.String("event_id", ev.ID))
		return nil
	}

	// 2. Dedup via LRU (1024 event_id capacity)
	if c.lru.checkAndAdd(ev.ID) {
		c.logger.DebugContext(ctx, "codeintel consumer: dropping duplicate event", slog.String("event_id", ev.ID))
		return nil
	}

	// 3. Unmarshal payload into CodeIntelPush
	var push codeintelv1.CodeIntelPush
	if err := json.Unmarshal(ev.Payload, &push); err != nil {
		c.logger.WarnContext(ctx, "codeintel consumer: malformed push payload",
			slog.String("event_id", ev.ID), slog.Any("error", err))
		return nil // ack malformed message to prevent infinite retry loops
	}

	// Ensure essential metadata from envelope is retained
	if push.EventId == "" {
		push.EventId = ev.ID
	}

	// 4. Dispatch to broadcaster
	c.broadcaster.Publish(&push)
	return nil
}
