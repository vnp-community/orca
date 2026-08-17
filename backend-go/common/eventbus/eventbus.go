// Package eventbus wraps NATS JetStream publish/consume per
// specs/backend-go/architecture/08-inter-service-communication.md's event
// conventions: subjects named orca.<service>.<entity>.<event>, every
// payload carries an event ID for consumer-side dedup, publishing always
// goes through the transactional-outbox pattern (see
// architecture/05-data-architecture.md) rather than a direct publish call
// inside a request handler.
package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Event is the envelope every published event uses — schema fields are
// additive-only across versions per the inter-service-communication doc.
type Event struct {
	ID         string          `json:"id"`
	TenantID   string          `json:"tenant_id"`
	OccurredAt time.Time       `json:"occurred_at"`
	Version    int             `json:"version"`
	Payload    json.RawMessage `json:"payload"`
}

// Publisher publishes events onto a subject. The OutboxRelay in a service's
// adapter/eventbus/ package is the only caller of this in production — see
// architecture/05's outbox-pattern rationale for why usecase/ code never
// calls Publish directly inside a request-handling transaction.
type Publisher struct {
	js jetstream.JetStream
}

// Consumer subscribes a named, durable consumer to a subject and invokes fn
// for each message, acking only after fn returns nil (at-least-once
// delivery — consumers must be idempotent, per the inter-service-communication doc).
type Consumer struct {
	js jetstream.JetStream
}

// Connect opens a NATS connection and returns both a Publisher and Consumer
// sharing it — the common case for a service that both publishes its own
// domain events and consumes others' (e.g. notification-service).
func Connect(ctx context.Context, url string) (*Publisher, *Consumer, func() error, error) {
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("eventbus: connecting to nats: %w", err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, nil, nil, fmt.Errorf("eventbus: creating jetstream context: %w", err)
	}
	closer := func() error {
		nc.Close()
		return nil
	}
	return &Publisher{js: js}, &Consumer{js: js}, closer, nil
}

// EnsureStream idempotently creates (or updates) a JetStream stream backing
// the given subjects — called once at service startup for every subject
// pattern this service publishes to.
func (p *Publisher) EnsureStream(ctx context.Context, name string, subjects []string) error {
	_, err := p.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     name,
		Subjects: subjects,
	})
	if err != nil {
		return fmt.Errorf("eventbus: ensuring stream %s: %w", name, err)
	}
	return nil
}

// Publish sends one event to subject. Called by an outbox relay loop, not
// directly from a request handler — see the package doc comment.
func (p *Publisher) Publish(ctx context.Context, subject string, event Event) error {
	b, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("eventbus: marshaling event: %w", err)
	}
	if _, err := p.js.Publish(ctx, subject, b); err != nil {
		return fmt.Errorf("eventbus: publishing to %s: %w", subject, err)
	}
	return nil
}

// Handler processes one delivered event; returning an error leaves the
// message unacked for redelivery.
type Handler func(ctx context.Context, event Event) error

// Subscribe creates (or reuses) a durable consumer named consumerName on
// streamName filtered to subject, and runs fn for every message until ctx
// is cancelled. The consumer name must be stable across restarts so
// JetStream resumes from the last acked position rather than replaying
// everything.
func (c *Consumer) Subscribe(ctx context.Context, streamName, consumerName, subject string, fn Handler) error {
	stream, err := c.js.Stream(ctx, streamName)
	if err != nil {
		return fmt.Errorf("eventbus: looking up stream %s: %w", streamName, err)
	}
	cons, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:       consumerName,
		FilterSubject: subject,
		AckPolicy:     jetstream.AckExplicitPolicy,
	})
	if err != nil {
		return fmt.Errorf("eventbus: creating consumer %s: %w", consumerName, err)
	}

	_, err = cons.Consume(func(msg jetstream.Msg) {
		var event Event
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			_ = msg.Nak() // malformed payload; NAK for visibility rather than silently dropping
			return
		}
		if err := fn(ctx, event); err != nil {
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
	})
	if err != nil {
		return fmt.Errorf("eventbus: starting consume loop: %w", err)
	}
	<-ctx.Done()
	return nil
}
