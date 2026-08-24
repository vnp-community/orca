package wscompat

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestNotImplementedChannelReturnsErrorFast verifies that an unregistered
// channel returns an error immediately (< 500ms), not after the 30s frontend
// INVOKE_TIMEOUT_MS. Regression guard for BUG-001 + BUG-002.
func TestNotImplementedChannelReturnsErrorFast(t *testing.T) {
	reg := NewRegistry() // empty registry — every channel falls to notImplementedHandler

	start := time.Now()
	_, err := reg.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "crashReports.getLatestPending", nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("want error for unregistered channel, got nil")
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("notImplementedHandler should be instant, took %s — possible context block", elapsed)
	}
}

// TestRegistryDispatch_UnregisteredChannelErrorContainsChannelName verifies
// that the notImplementedHandler's error message contains the channel name
// so the frontend (and logs) can identify which channel is missing.
func TestRegistryDispatch_UnregisteredChannelErrorContainsChannelName(t *testing.T) {
	reg := NewRegistry()

	_, err := reg.Dispatch(context.Background(), Identity{}, "rateLimits.get", nil)
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !strings.Contains(err.Error(), "rateLimits.get") {
		t.Errorf("error message should contain channel name 'rateLimits.get', got: %q", err.Error())
	}
}

// TestWriteTimeoutConstant_ShorterThanInvokeTimeout documents the required
// relationship between writeTimeout (SOL-001) and invokeTimeout. If someone
// accidentally sets writeTimeout >= invokeTimeout, the write would always
// race with the dispatch cancellation instead of running independently.
func TestWriteTimeoutConstant_ShorterThanInvokeTimeout(t *testing.T) {
	if writeTimeout >= invokeTimeout {
		t.Errorf("writeTimeout (%s) must be < invokeTimeout (%s); "+
			"writeTimeout is for the write-back step only, not the full dispatch",
			writeTimeout, invokeTimeout)
	}
}
