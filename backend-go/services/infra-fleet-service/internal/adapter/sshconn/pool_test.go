package sshconn_test

import (
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshconn"
)

func TestCap_RejectsEleventhConcurrentConnection(t *testing.T) {
	c := sshconn.NewCap()

	var releases []func()
	for i := 0; i < 10; i++ {
		release, err := c.Acquire("tenant-1", "host-a")
		if err != nil {
			t.Fatalf("Acquire #%d: unexpected error: %v", i+1, err)
		}
		releases = append(releases, release)
	}

	_, err := c.Acquire("tenant-1", "host-a")
	if !errors.Is(err, sshconn.ErrTooManyConcurrentConnections) {
		t.Fatalf("expected the 11th Acquire to fail with ErrTooManyConcurrentConnections, got %v", err)
	}

	// Releasing one slot frees capacity for a new Acquire.
	releases[0]()
	if _, err := c.Acquire("tenant-1", "host-a"); err != nil {
		t.Fatalf("expected Acquire to succeed after a release, got %v", err)
	}
}

func TestCap_DifferentHostsAndTenantsDontShareACounter(t *testing.T) {
	c := sshconn.NewCap()

	for i := 0; i < 10; i++ {
		if _, err := c.Acquire("tenant-1", "host-a"); err != nil {
			t.Fatalf("Acquire tenant-1/host-a #%d: %v", i+1, err)
		}
	}

	if _, err := c.Acquire("tenant-1", "host-b"); err != nil {
		t.Errorf("expected a different host to have its own counter, got %v", err)
	}
	if _, err := c.Acquire("tenant-2", "host-a"); err != nil {
		t.Errorf("expected a different tenant to have its own counter, got %v", err)
	}
}

func TestCap_ReleaseIsIdempotent(t *testing.T) {
	c := sshconn.NewCap()
	release, err := c.Acquire("tenant-1", "host-a")
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	release()
	release() // must not double-decrement / panic

	for i := 0; i < 10; i++ {
		if _, err := c.Acquire("tenant-1", "host-a"); err != nil {
			t.Fatalf("Acquire #%d after idempotent release: %v", i+1, err)
		}
	}
}
