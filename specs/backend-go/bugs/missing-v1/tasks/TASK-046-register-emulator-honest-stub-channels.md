# TASK-046: Register `emulator.*` honest-stub channels

**From Solution:** SOL-008 (Design — Part 1: shippable now)
**Priority:** P2
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** none
**Status:** `[x]` DONE (verified) — implemented in a new file
`channels_emulator_folderworkspace_host.go` (not `channels.go` directly —
see this pass's worktree brief: `channels.go` is off-limits while parallel
groups add channels there too), with `registerEmulatorChannels` called
from the new file's `registerEmulatorFolderWorkspaceHostChannels` entry
point. `go build`/`go vet` clean; not yet wired into `RegisterRealChannels`
— see TASK-047's test run and the integration-pass wiring note in the new
file's package doc comment.

---

## Context

All 8 `emulator.*` channels (`emulator.attach`, `emulator.availability`,
`emulator.button`, `emulator.gesture`, `emulator.listDevices`,
`emulator.rotate`, `emulator.shutdown`, `emulator.tap`) currently fall
through to `notImplementedHandler`'s generic "not yet implemented"
message, which implies "coming soon." That is inaccurate: per
`02-microservices-decomposition.md`'s "What's deliberately not a separate
service" section, mobile emulator/simulator control on the shared
`backend-go` host is explicitly excluded by design, not an oversight.

This task registers real handlers that return a clear, permanent, typed
error instead. It does **not** attempt any relay to a Dev Server Agent —
that half of SOL-008 (Part 2) is blocked on an `agent/` capability that
does not exist today; see TASK-048, which documents that target design
without implementing it. Only this task's honest-stub half is buildable
now.

---

## Changes to make

**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`

### Step 1: Add `errEmulatorNotSupported` and `registerEmulatorChannels`

Add at the end of the file (after `registerRateLimitChannels`):

```go
// ── emulator.* ──────────────────────────────────────────────────────────
//
// Mobile emulator/simulator control (ADB/xcrun simctl device driving) has
// no backend-go implementation and, per
// 02-microservices-decomposition.md's "What's deliberately not a separate
// service" section, is explicitly excluded from the Go server deployment
// by design — not a gap awaiting a future pass. The architecturally sound
// alternative (relay to the Dev Server Agent) requires a new agent/
// capability that does not exist today; agent/ changes are out of scope
// for this rewrite. See specs/backend-go/bugs/missing-v1/tasks/TASK-048
// for the blocked, documented-only relay design. Every emulator.* channel
// below returns this same typed, permanent answer instead of falling
// through to notImplementedHandler's generic "not yet" wording, which
// would incorrectly imply this is only temporarily missing.
var errEmulatorNotSupported = errors.New(
	"mobile emulator control is not supported by the Go backend — " +
		"see specs/backend-go/bugs/missing-v1/solutions/SOL-008-emulator-channels.md")

func registerEmulatorChannels(r *Registry) {
	for _, channel := range []string{
		"emulator.attach", "emulator.availability", "emulator.button",
		"emulator.gesture", "emulator.listDevices", "emulator.rotate",
		"emulator.shutdown", "emulator.tap",
	} {
		r.Register(channel, func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
			return nil, errEmulatorNotSupported
		})
	}
}
```

### Step 2: Check the `errors` import

`channels.go` may not already import `"errors"`. If it does not, add it to
the import block (standard library group, alongside `"context"`,
`"encoding/json"`, etc.).

### Step 3: Wire into `RegisterRealChannels`

Find:

```go
	registerCrashReportChannels(r)
	registerRateLimitChannels(r, rateLimits)
}
```

Replace with:

```go
	registerCrashReportChannels(r)
	registerRateLimitChannels(r, rateLimits)
	registerEmulatorChannels(r)
}
```

No new parameter is needed — `registerEmulatorChannels` takes only `r`.

---

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./internal/adapter/wscompat/...
go vet ./internal/adapter/wscompat/...
```

Expected: clean build. `RegisterRealChannels`'s call signature is
unchanged (no new parameter), so no downstream `main.go` build breakage.
