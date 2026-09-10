package portalloc

import (
	"errors"
	"fmt"
	"net"
	"testing"
)

func TestAllocator_ExcludesWellKnownPorts(t *testing.T) {
	a := NewAllocator()
	for i := 0; i < 200; i++ {
		port, err := a.Allocate(fmt.Sprintf("pf-%d", i))
		if err != nil {
			t.Fatalf("Allocate #%d: %v", i, err)
		}
		if wellKnownExcluded[port] {
			t.Fatalf("Allocate returned excluded well-known port %d", port)
		}
	}
}

func TestAllocator_StaysWithinRange(t *testing.T) {
	a := NewAllocator()
	for i := 0; i < 200; i++ {
		port, err := a.Allocate(fmt.Sprintf("pf-%d", i))
		if err != nil {
			t.Fatalf("Allocate #%d: %v", i, err)
		}
		if port < portRangeStart || port > portRangeEnd {
			t.Fatalf("Allocate returned out-of-range port %d, want [%d, %d]", port, portRangeStart, portRangeEnd)
		}
	}
}

func TestAllocator_SameRemotePortDifferentConnections_GetDifferentLocalPorts(t *testing.T) {
	a := NewAllocator()
	// Same "remote port" concept (namespacing is keyed by portForwardID, not
	// remotePort — this Allocator never sees remotePort at all) — two
	// different portForwardIDs must never collide on the same local port.
	port1, err := a.Allocate("connA:pf-1")
	if err != nil {
		t.Fatalf("Allocate #1: %v", err)
	}
	port2, err := a.Allocate("connB:pf-1")
	if err != nil {
		t.Fatalf("Allocate #2: %v", err)
	}
	if port1 == port2 {
		t.Errorf("expected different local ports for different portForwardIDs, both got %d", port1)
	}
}

func TestAllocator_ReleaseFreesPortForReuse(t *testing.T) {
	a := NewAllocator()
	port, err := a.Allocate("pf-1")
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	a.Release(port)

	a.mu.Lock()
	_, stillTracked := a.inUse[port]
	a.mu.Unlock()
	if stillTracked {
		t.Errorf("expected port %d to be released from bookkeeping", port)
	}

	// A fresh listener can now genuinely bind that exact port at the OS
	// level too (not just the allocator's own map) — proves Release() didn't
	// leave anything held.
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("expected port %d to be bindable after Release, got: %v", port, err)
	}
	_ = ln.Close()
}

func TestAllocator_ExhaustedRangeReturnsErrNoPortAvailable(t *testing.T) {
	a := NewAllocator()
	// Fill the allocator's own bookkeeping for the whole range via real
	// Allocate() calls (each one genuinely net.Listen+Closes to confirm
	// freeness at call time, then marks it taken) — cheaper and less flaky
	// than holding ~7000 concurrent OS listeners simultaneously (risking the
	// test hitting the process's own file-descriptor ulimit, which would be
	// an environment artifact, not a real Allocator bug).
	//
	// The exact number of successful allocations before exhaustion depends
	// on how many ports in [3001, 9999] some OTHER process on this machine
	// already holds at the moment the test runs (Allocate's own net.Listen
	// probe correctly refuses those too) — so this loops until Allocate
	// itself reports exhaustion rather than asserting an exact count.
	total := portRangeEnd - portRangeStart + 1
	allocated := 0
	var lastErr error
	for i := 0; i < total+1; i++ { // +1: one more than the theoretical max, to guarantee exhaustion is reached
		if _, err := a.Allocate(fmt.Sprintf("pf-%d", i)); err != nil {
			lastErr = err
			break
		}
		allocated++
	}

	if !errors.Is(lastErr, ErrNoPortAvailable) {
		t.Fatalf("expected ErrNoPortAvailable once the range is exhausted (after allocating %d ports), got: %v", allocated, lastErr)
	}
	if allocated == 0 {
		t.Error("expected at least one successful allocation before exhaustion")
	}
}
