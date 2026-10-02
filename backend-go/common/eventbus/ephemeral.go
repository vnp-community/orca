package eventbus

import (
	"fmt"

	"github.com/nats-io/nats.go"
)

// Ephemeral is core-NATS pub/sub: NOT durable, NO replay. It exists for
// self-healing control signals between replicas (cancel, session closed,
// list_changed hints) where losing a message is acceptable. Domain events
// must keep using Publisher/Consumer (JetStream). Note that Consumer.
// SubscribeEphemeral is a JetStream ephemeral consumer, not this.
type Ephemeral struct{ nc *nats.Conn }

// NewEphemeral opens its own connection so a signal storm cannot starve the
// JetStream connection (pending limits are bounded per subscription).
func NewEphemeral(url string) (*Ephemeral, func(), error) {
	nc, err := nats.Connect(url, nats.Name("orca-ephemeral"))
	if err != nil {
		return nil, nil, fmt.Errorf("eventbus: connecting to nats: %w", err)
	}
	return &Ephemeral{nc: nc}, nc.Close, nil
}

func (e *Ephemeral) Publish(subject string, data []byte) error {
	return e.nc.Publish(subject, data)
}

// Subscribe calls fn sequentially for every message on subject (wildcards
// allowed). The returned func unsubscribes. A slow fn drops messages once the
// bounded pending buffer is full instead of growing memory.
func (e *Ephemeral) Subscribe(subject string, fn func(subject string, data []byte)) (func(), error) {
	sub, err := e.nc.Subscribe(subject, func(m *nats.Msg) { fn(m.Subject, m.Data) })
	if err != nil {
		return nil, fmt.Errorf("eventbus: subscribing to %s: %w", subject, err)
	}
	if err := sub.SetPendingLimits(1024, 4<<20); err != nil {
		_ = sub.Unsubscribe()
		return nil, err
	}
	return func() { _ = sub.Unsubscribe() }, nil
}
