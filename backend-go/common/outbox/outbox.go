// Package outbox implements the poll-based relay half of the
// transactional-outbox pattern from
// specs/backend-go/architecture/05-data-architecture.md's "Transactional
// outbox + async events (default)" section: a service writes its domain
// state change and an outbox row in the SAME Postgres transaction, then a
// relay process (this package) polls unpublished rows and publishes them
// to NATS JetStream. This closes the small window every
// publish-directly-after-commit usecase in this scaffold previously
// accepted (docs/execution-plan.md Epic G): a crash between the DB commit
// and the direct publish call used to silently drop the event; with
// outbox, the event row is already durably committed alongside the domain
// write, so the relay just needs to notice it and retry until NATS
// confirms — nothing is lost even if the process dies mid-relay.
//
// This package intentionally owns only the relay LOOP, not the schema or
// the SQL. Per this codebase's database-per-service rule
// (architecture/05 again), each service's own internal/adapter/postgres
// implements Store against its own outbox table (e.g.
// usage.outbox_events), matching every other repository in this
// codebase's hand-written-SQL, one-package-per-service convention.
// Enqueueing a row (the INSERT that must share a transaction with the
// domain write) is also each service's own responsibility for the same
// reason — this package cannot safely wrap an arbitrary caller's
// in-flight transaction from the outside.
package outbox

import (
	"context"
	"log/slog"
	"time"

	"github.com/stablyai/orca-go/common/eventbus"
)

// Record is one durably-committed outbox row, ready to publish.
type Record struct {
	ID      string
	Subject string
	Event   eventbus.Event
}

// Store is the persistence port the relay polls — implemented per service
// against that service's own outbox table.
type Store interface {
	// FetchUnpublished returns up to limit not-yet-published rows, oldest
	// first, so the relay makes forward progress under sustained backlog
	// instead of thrashing on the newest rows.
	FetchUnpublished(ctx context.Context, limit int) ([]Record, error)
	// MarkPublished marks ids as published — called only after every one
	// of them was handed to eventbus.Publisher.Publish successfully, so a
	// crash between publish and mark just means at-least-once redelivery
	// on the next poll (consumers must already be idempotent, per
	// architecture/05/08's dedupe-on-event-ID convention), never a lost
	// event.
	MarkPublished(ctx context.Context, ids []string) error
}

// Config tunes the relay's polling cadence and batch size.
type Config struct {
	// PollInterval is the ticker period. A batch that comes back full polls
	// again immediately (see Relay.Run) rather than waiting out the next
	// tick, so backlog drains as fast as NATS/the DB allow; PollInterval
	// only bounds latency for the common case of an otherwise-idle outbox.
	PollInterval time.Duration
	// BatchSize bounds one poll's FetchUnpublished call.
	BatchSize int
}

// DefaultConfig is a reasonable default for this scaffold's event volume —
// tune per service once real traffic data exists.
var DefaultConfig = Config{PollInterval: 2 * time.Second, BatchSize: 100}

// Relay polls Store and publishes each unpublished row via a
// *eventbus.Publisher, then marks it published. Every replica of a
// horizontally-scaled service can safely run its own Relay against the
// same Store: MarkPublished only ever runs after a successful publish, and
// a row published twice by two replicas racing the same poll is exactly
// the at-least-once case consumers must already tolerate — no
// leader-election or locking is needed for correctness, only for avoiding
// (harmless) duplicate work.
type Relay struct {
	store  Store
	pub    *eventbus.Publisher
	cfg    Config
	logger *slog.Logger
}

// NewRelay constructs a Relay. A non-positive PollInterval or BatchSize in
// cfg falls back to DefaultConfig; a nil logger falls back to
// slog.Default().
func NewRelay(store Store, pub *eventbus.Publisher, cfg Config, logger *slog.Logger) *Relay {
	if cfg.PollInterval <= 0 || cfg.BatchSize <= 0 {
		cfg = DefaultConfig
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Relay{store: store, pub: pub, cfg: cfg, logger: logger}
}

// Run polls until ctx is cancelled. A fetch or publish error is logged and
// retried on the next tick — never fatal to the process, since the whole
// point of outbox is that the domain write already durably succeeded
// independent of this loop's health.
func (r *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.cfg.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for r.drainOneBatch(ctx) {
				// The last batch came back full — more work may be waiting
				// right now, so poll again immediately instead of idling
				// until the next tick.
			}
		}
	}
}

// drainOneBatch publishes up to one batch and reports whether the batch was
// full (a hint there may be more work waiting right now).
func (r *Relay) drainOneBatch(ctx context.Context) bool {
	records, err := r.store.FetchUnpublished(ctx, r.cfg.BatchSize)
	if err != nil {
		r.logger.WarnContext(ctx, "outbox: fetch unpublished failed", slog.Any("error", err))
		return false
	}
	if len(records) == 0 {
		return false
	}

	published := make([]string, 0, len(records))
	for _, rec := range records {
		if err := r.pub.Publish(ctx, rec.Subject, rec.Event); err != nil {
			r.logger.WarnContext(ctx, "outbox: publish failed, will retry next poll",
				slog.String("id", rec.ID), slog.String("subject", rec.Subject), slog.Any("error", err))
			// Stop at the first failure in this batch rather than skipping
			// ahead to the next row — a persistently-failing NATS
			// connection shouldn't silently reorder delivery relative to
			// rows that would otherwise publish fine.
			break
		}
		published = append(published, rec.ID)
	}
	if len(published) == 0 {
		return false
	}
	if err := r.store.MarkPublished(ctx, published); err != nil {
		r.logger.WarnContext(ctx, "outbox: mark published failed — next poll will re-publish these rows (consumers must be idempotent)",
			slog.Any("error", err))
	}
	return len(published) == len(records)
}
