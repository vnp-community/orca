# TASK-047: Test `emulator.*` stub channels

**From Solution:** SOL-008 (Test plan — "Shippable now")
**Priority:** P2
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_test.go`
**Depends on:** TASK-046
**Status:** `[ ]` TODO

---

## Context

Regression guard distinguishing "permanently excluded" (this namespace)
from "temporarily missing" (`notImplementedHandler`'s generic message) —
asserts all 8 `emulator.*` channels resolve to a real registered handler
that returns `errEmulatorNotSupported`, never the generic not-implemented
error and never a panic.

---

## Changes to make

**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_test.go`

Add a new table-driven test:

```go
func TestRegisterEmulatorChannels_AllReturnHonestNotSupportedError(t *testing.T) {
	r := NewRegistry()
	registerEmulatorChannels(r)

	channels := []string{
		"emulator.attach", "emulator.availability", "emulator.button",
		"emulator.gesture", "emulator.listDevices", "emulator.rotate",
		"emulator.shutdown", "emulator.tap",
	}

	for _, channel := range channels {
		t.Run(channel, func(t *testing.T) {
			result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, channel, nil)
			if result != nil {
				t.Errorf("expected nil result, got %v", result)
			}
			if !errors.Is(err, errEmulatorNotSupported) {
				t.Errorf("expected errEmulatorNotSupported, got %v", err)
			}
			// Regression guard: must not fall through to
			// notImplementedHandler's generic "not yet implemented" message.
			if err != nil && strings.Contains(err.Error(), "is not yet implemented in backend-go") {
				t.Errorf("channel %q fell through to notImplementedHandler, want a registered honest-stub handler", channel)
			}
		})
	}
}
```

Ensure the test file's imports include `"errors"` and `"strings"` (add if
missing alongside the existing `"context"`/`"testing"` imports).

---

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go test ./internal/adapter/wscompat/... -run TestRegisterEmulatorChannels -v
go vet ./internal/adapter/wscompat/...
```

Expected: all 8 subtests pass.
