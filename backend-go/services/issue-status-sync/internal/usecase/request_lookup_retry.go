package usecase

import "sync"

// maxTransientDeliveries bounds how often a transient failure may Nak the
// same event; consumers have no MaxDeliver, so without it a persistent
// failure would redeliver forever.
const maxTransientDeliveries = 3

const deliveryCounterCapacity = 10000

// deliveryCounter counts deliveries per event ID in memory. It is not shared
// between replicas, so the real bound is 3 per replica that receives the event.
type deliveryCounter struct {
	mu     sync.Mutex
	counts map[string]int
}

func newDeliveryCounter() *deliveryCounter {
	return &deliveryCounter{counts: map[string]int{}}
}

// next records one more delivery of eventID and returns the new count.
func (c *deliveryCounter) next(eventID string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.counts[eventID]; !ok && len(c.counts) >= deliveryCounterCapacity {
		// Random map order makes this evict arbitrary entries; exact LRU is not worth it here.
		for k := range c.counts {
			delete(c.counts, k)
			if len(c.counts) < deliveryCounterCapacity*9/10 {
				break
			}
		}
	}
	c.counts[eventID]++
	return c.counts[eventID]
}

func (c *deliveryCounter) forget(eventID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.counts, eventID)
}
