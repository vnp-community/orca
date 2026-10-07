package broadcaster

import (
	"sync"

	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
)

// CodeIntelPushBroadcaster fans out CodeIntelPush messages to active gRPC server stream subscribers.
type CodeIntelPushBroadcaster struct {
	mu   sync.RWMutex
	subs map[uint64]chan *codeintelv1.CodeIntelPush
	next uint64
}

// NewCodeIntelPushBroadcaster creates a new broadcaster.
func NewCodeIntelPushBroadcaster() *CodeIntelPushBroadcaster {
	return &CodeIntelPushBroadcaster{
		subs: make(map[uint64]chan *codeintelv1.CodeIntelPush),
	}
}

// Publish broadcasts a push event to all subscribers without blocking.
// If a subscriber buffer (size 64) is full, drops the message and sends an overflow notice.
func (b *CodeIntelPushBroadcaster) Publish(push *codeintelv1.CodeIntelPush) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, ch := range b.subs {
		select {
		case ch <- push:
		default:
			// Buffer full (slow consumer): drop and send overflow signal
			overflowPush := &codeintelv1.CodeIntelPush{
				Kind:            "changed",
				EventId:         push.EventId,
				WorktreeId:      push.WorktreeId,
				RepoBindingId:   push.RepoBindingId,
				ProjectId:       push.ProjectId,
				Reason:          "overflow",
				OccurredAt:      push.OccurredAt,
			}
			select {
			case ch <- overflowPush:
			default:
				// If still full, drop silently
			}
		}
	}
}

// Subscribe attaches a new subscriber channel with a buffer of 64 messages.
// Returns the channel and an unsubscribe cleanup function.
func (b *CodeIntelPushBroadcaster) Subscribe() (<-chan *codeintelv1.CodeIntelPush, func()) {
	ch := make(chan *codeintelv1.CodeIntelPush, 64)
	b.mu.Lock()
	id := b.next
	b.next++
	b.subs[id] = ch
	b.mu.Unlock()

	unsubscribe := func() {
		b.mu.Lock()
		delete(b.subs, id)
		close(ch)
		b.mu.Unlock()
	}

	return ch, unsubscribe
}
