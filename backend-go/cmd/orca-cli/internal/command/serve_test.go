package command

import (
	"context"
	"errors"
	"testing"
)

// TestServe_NotLocal_ReturnsErrorWithoutCallingSupervisor proves --local
// false refuses to start without ever calling sup.Start — sup=nil means a
// mistaken call would nil-pointer-panic this test instead of silently
// starting something.
func TestServe_NotLocal_ReturnsErrorWithoutCallingSupervisor(t *testing.T) {
	err := RunServe(context.Background(), false, nil)

	if !errors.Is(err, errServeRequiresLocal) {
		t.Fatalf("RunServe() error = %v, want errServeRequiresLocal", err)
	}
}
