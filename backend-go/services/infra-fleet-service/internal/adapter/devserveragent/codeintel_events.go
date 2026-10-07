package devserveragent

import (
	"sync"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

const codeIntelSubBufferCapacity = 64

type codeIntelSub struct {
	mu        sync.Mutex
	cond      *sync.Cond
	queue     []domain.CodeIntelEvent
	out       chan domain.CodeIntelEvent
	dropped   bool
	closed    bool
	closeOnce sync.Once
	done      chan struct{}
}

func newCodeIntelSub() *codeIntelSub {
	s := &codeIntelSub{
		queue: make([]domain.CodeIntelEvent, 0, codeIntelSubBufferCapacity),
		out:   make(chan domain.CodeIntelEvent),
		done:  make(chan struct{}),
	}
	s.cond = sync.NewCond(&s.mu)
	go s.run()
	return s
}

func (s *codeIntelSub) push(ev domain.CodeIntelEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	// Reserve slot for the item in flight in s.run, keeping total buffered events <= 64.
	if len(s.queue) >= codeIntelSubBufferCapacity-1 {
		s.dropped = true
		return
	}
	s.queue = append(s.queue, ev)
	s.cond.Signal()
}

func (s *codeIntelSub) run() {
	defer close(s.out)
	for {
		s.mu.Lock()
		for len(s.queue) == 0 && !s.dropped && !s.closed {
			s.cond.Wait()
		}
		if s.closed && len(s.queue) == 0 && !s.dropped {
			s.mu.Unlock()
			return
		}

		var ev domain.CodeIntelEvent
		if len(s.queue) > 0 {
			ev = s.queue[0]
			s.queue = s.queue[1:]
		} else if s.dropped {
			ev = domain.CodeIntelEvent{
				Kind:       domain.CodeIntelEventKindOverflow,
				ReceivedAt: time.Now(),
			}
			s.dropped = false
		}
		s.mu.Unlock()

		select {
		case s.out <- ev:
		case <-s.done:
			return
		}
	}
}

func (s *codeIntelSub) close() {
	s.closeOnce.Do(func() {
		close(s.done)
		s.mu.Lock()
		s.closed = true
		s.cond.Broadcast()
		s.mu.Unlock()
	})
}

// SubscribeCodeIntelEvents registers a subscriber for devServerID's CodeIntel events.
// The subscription is tracked at the Client level so subscribers can register before
// a devServer agent connects. Returns a receive-only channel and an idempotent unsubscribe func.
func (c *Client) SubscribeCodeIntelEvents(devServerID string) (<-chan domain.CodeIntelEvent, func()) {
	sub := newCodeIntelSub()

	c.codeIntelMu.Lock()
	c.codeIntelSubs[devServerID] = append(c.codeIntelSubs[devServerID], sub)
	c.codeIntelMu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			c.codeIntelMu.Lock()
			current := c.codeIntelSubs[devServerID]
			filtered := make([]*codeIntelSub, 0, len(current))
			for _, s := range current {
				if s != sub {
					filtered = append(filtered, s)
				}
			}
			if len(filtered) == 0 {
				delete(c.codeIntelSubs, devServerID)
			} else {
				c.codeIntelSubs[devServerID] = filtered
			}
			c.codeIntelMu.Unlock()

			sub.close()
		})
	}

	return sub.out, unsubscribe
}

// publishCodeIntelEvent fans out an event to all subscribers of devServerID.
func (c *Client) publishCodeIntelEvent(devServerID string, ev domain.CodeIntelEvent) {
	c.codeIntelMu.Lock()
	subs := append([]*codeIntelSub(nil), c.codeIntelSubs[devServerID]...)
	c.codeIntelMu.Unlock()

	for _, sub := range subs {
		sub.push(ev)
	}
}

// emitResync emits a resync event to all subscribers of devServerID upon transport attachment.
func (c *Client) emitResync(devServerID string) {
	ev := domain.CodeIntelEvent{
		Kind:       domain.CodeIntelEventKindResync,
		ReceivedAt: time.Now(),
	}
	c.publishCodeIntelEvent(devServerID, ev)
}

// routeCodeIntelNotification decodes and dispatches a JSON-RPC notification to devServerID subscribers.
func (c *Client) routeCodeIntelNotification(devServerID string, n JSONRPCNotification) {
	ev, ok := decodeCodeIntelNotification(n, time.Now())
	if !ok {
		c.logger.Debug("discarding invalid codeintel notification", "method", n.Method, "devServerID", devServerID)
		return
	}
	c.publishCodeIntelEvent(devServerID, ev)
}
