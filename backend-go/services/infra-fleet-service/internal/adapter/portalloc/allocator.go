// Package portalloc hands out local ports for auto-port-forwarding
// (SOL-SSH-04) — see Allocator's doc comment.
package portalloc

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
)

const (
	portRangeStart = 3001
	portRangeEnd   = 9999
)

var wellKnownExcluded = map[int]bool{22: true, 25: true, 53: true, 80: true, 443: true} // BR-SSH-16

// ErrNoPortAvailable is returned when every port in [3001, 9999] is either
// already allocated or bound at the OS level by something else.
var ErrNoPortAvailable = fmt.Errorf("portalloc: no free local port available in [%d, %d]", portRangeStart, portRangeEnd)

// Allocator hands out a free local port in [3001, 9999] (BR-SSH-17),
// avoiding wellKnownExcluded. The in-use set is keyed globally by
// portForwardID (not per-connection), so a second request for the same
// remote port from a DIFFERENT connectionId can never collide on the same
// local port even though both remote ports are numerically identical —
// this IS BR-SSH-19's namespacing.
type Allocator struct {
	mu    sync.Mutex
	inUse map[int]string // local port -> portForwardID holding it
}

func NewAllocator() *Allocator {
	return &Allocator{inUse: make(map[int]string)}
}

// Allocate probes candidates in randomized order (avoids always handing out
// the lowest free port, which would make concurrent allocation races more
// likely) and net.Listen+immediately-Close each candidate to confirm it's
// free at the OS level too — defense against a non-Orca process already
// holding it, not just this Allocator's own bookkeeping.
func (a *Allocator) Allocate(portForwardID string) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	candidates := rand.Perm(portRangeEnd - portRangeStart + 1)
	for _, offset := range candidates {
		port := portRangeStart + offset
		if wellKnownExcluded[port] {
			continue
		}
		if _, taken := a.inUse[port]; taken {
			continue
		}
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			continue // held by something outside this Allocator's bookkeeping
		}
		_ = ln.Close()
		a.inUse[port] = portForwardID
		return port, nil
	}
	return 0, ErrNoPortAvailable
}

// Release frees localPort for reuse.
func (a *Allocator) Release(localPort int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.inUse, localPort)
}
