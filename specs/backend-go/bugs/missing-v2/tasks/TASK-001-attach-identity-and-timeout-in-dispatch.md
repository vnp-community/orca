# TASK-001: Attach caller identity + deadline once in `Registry.Dispatch`/`DispatchStreamChannel`, remove now-redundant per-handler calls

**From Solution:** SOL-001
**Priority:** P0 — do first; SOL-005 (TASK-010) and SOL-006 (TASK-012) touch the same file and are easiest to land as one coordinated change
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/registry.go`, plus every `channels_*.go` file with a per-handler `gatewaygrpc.AttachIdentity` call (removal pass)
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

`folderWorkspace.*`'s 5 channels (`channels_emulator_folderworkspace_host.go`)
never call `gatewaygrpc.AttachIdentity` before their gRPC client calls,
unlike every sibling handler — see BUG-001. The fix moves identity
attachment (and the mandatory per-outbound-call deadline
`08-inter-service-communication.md` requires) into `Registry.Dispatch`
itself, the one place every `ChannelHandler` invocation already passes
through, so no handler needs to remember to do it — and no future handler
can forget.

## Changes to make

### Step 1 — `registry.go`: attach identity + deadline in `Dispatch`

Current code (`registry.go:117-125`):

```go
// Dispatch resolves and invokes the handler for channel, falling back to
// notImplementedHandler when nothing is registered.
func (r *Registry) Dispatch(ctx context.Context, id Identity, channel string, args []json.RawMessage) (any, error) {
	h, ok := r.handlers[channel]
	if !ok {
		return notImplementedHandler(ctx, id, channel)
	}
	return h(ctx, id, args)
}
```

Replace with:

```go
// dispatchRPCTimeout is the default deadline applied to every outbound
// gRPC call a ChannelHandler makes, attached here so no handler needs its
// own context.WithTimeout — matches 08-inter-service-communication.md's
// "Deadlines are mandatory on every outbound call... no unbounded gRPC
// call exists anywhere in the system; default 5s for intra-cluster calls."
// Several existing handler groups defined their own, slightly different
// per-group constants (e.g. repoSSHStatusWorkspaceRPCTimeout) — verify
// those don't encode a deliberately-different budget for a documented
// reason before deleting them in favor of this one; if none is found,
// consolidate on this single constant everywhere.
const dispatchRPCTimeout = 5 * time.Second

// Dispatch resolves and invokes the handler for channel, falling back to
// notImplementedHandler when nothing is registered. Attaches the caller's
// identity onto ctx as outbound gRPC metadata and applies dispatchRPCTimeout
// ONCE here — see BUG-001 (specs/backend-go/bugs/missing-v2/) for why this
// must not be left to each handler to do individually.
func (r *Registry) Dispatch(ctx context.Context, id Identity, channel string, args []json.RawMessage) (any, error) {
	h, ok := r.handlers[channel]
	if !ok {
		return notImplementedHandler(ctx, id, channel)
	}
	ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
	ctx, cancel := context.WithTimeout(ctx, dispatchRPCTimeout)
	defer cancel()
	return h(ctx, id, args)
}
```

Add the two new imports:

```go
import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)
```

### Step 2 — `registry.go`: apply the same fix to `DispatchStreamChannel`

For consistency (a `StreamChannelHandler` like `terminal.create` makes
outbound gRPC calls too, via the same `AttachIdentity` pattern in its own
`channels_*.go` file — verify this before assuming, but the shape is
identical enough to fix alongside `Dispatch` rather than leave as a
second, un-audited instance of the same bug class):

```go
func (r *Registry) DispatchStreamChannel(ctx context.Context, id Identity, channel string, args []json.RawMessage) (ack any, events <-chan PushEvent, ok bool, err error) {
	h, found := r.streamChannelHandlers[channel]
	if !found {
		return nil, nil, false, nil
	}
	ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
	// Why: NO context.WithTimeout here, unlike Dispatch — a stream channel's
	// ack/events lifecycle is long-lived by design (e.g. a terminal session),
	// not a single bounded RPC. Confirm this against how the existing
	// StreamChannelHandler implementations currently manage their own
	// lifetimes before deciding whether a (much longer, or absent) deadline
	// belongs here too — do not blindly reuse dispatchRPCTimeout, which
	// would break any handler that legitimately outlives 5s.
	ack, events, err = h(ctx, id, args)
	return ack, events, true, err
}
```

### Step 3 — Remove now-redundant per-handler `AttachIdentity` calls

Every existing `ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})`
line inside a `channels_*.go` handler body is now dead weight (harmless —
`AttachIdentity` overwrites rather than merges — but confusing duplication
if left in). Find every occurrence:

```bash
cd backend-go/services/api-gateway
grep -rln 'gatewaygrpc.AttachIdentity' internal/adapter/wscompat/*.go | grep -v _test
```

For each match, delete the `ctx = gatewaygrpc.AttachIdentity(...)` line
from inside the handler closure. **Do not** touch:
- The `rpcCtx, cancel := context.WithTimeout(ctx, <groupTimeout>)` lines
  that follow — those still apply a (possibly different, see Step 1's
  constant-consolidation note) deadline on top of the now-already-identity-attached
  `ctx`; harmless to keep both timeouts stacked, but if consolidating onto
  `dispatchRPCTimeout` per Step 1, these become redundant too and should be
  removed in the same pass, not left half-migrated.
- Any file where `gatewaygrpc`/`usecase` imports become unused after
  deletion — remove the now-unused import in the same file, or `go build`
  will fail.

After this step, `folderWorkspace.*`'s 5 handlers need **no changes at
all** — they never had the line to remove, and now correctly inherit
identity from `Dispatch`. This is the actual fix for BUG-001.

## Verify

```bash
cd backend-go
go build ./services/api-gateway/...
go vet ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/wscompat/... -count=1 -v
```

Expected: clean build, no unused imports, all existing tests still pass
(TASK-002 adds the new regression tests this fix specifically needs — this
task's own verify step only proves nothing existing broke).
