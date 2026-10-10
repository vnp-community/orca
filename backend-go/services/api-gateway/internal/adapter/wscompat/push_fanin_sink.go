package wscompat

import (
	"context"
	"sync"
)

// pushFanInSink lets many NATS callbacks feed one PushEvent channel and still
// close it safely. A send to a closed channel panics even inside a select with a
// ready ctx.Done(), and a JetStream callback can run after SubscribeEphemeral
// returned, so the producer side must never race the close.
type pushFanInSink struct {
	out  chan PushEvent
	done chan struct{} // closed first by close() so blocked senders let go
	mu   sync.RWMutex  // senders hold R while sending; close() takes W
	shut bool
	once sync.Once
}

func newPushFanInSink() *pushFanInSink {
	return &pushFanInSink{out: make(chan PushEvent), done: make(chan struct{})}
}

// send delivers ev, or drops it once ctx ended or the sink was closed.
func (s *pushFanInSink) send(ctx context.Context, ev PushEvent) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.shut {
		return false
	}
	select {
	case s.out <- ev:
		return true
	case <-ctx.Done():
		return false
	case <-s.done:
		return false
	}
}

// close is idempotent; the channel is closed only after in-flight sends return.
func (s *pushFanInSink) close() {
	s.once.Do(func() {
		close(s.done)
		s.mu.Lock()
		s.shut = true
		close(s.out)
		s.mu.Unlock()
	})
}
