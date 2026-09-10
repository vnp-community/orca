# TASK-002: Regression tests — `Dispatch` attaches identity for every registered channel

**From Solution:** SOL-001
**Priority:** P0
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/registry_test.go` (new)
**Depends on:** TASK-001
**Status:** `[x]` DONE (2/3 tests) — `TestDispatch_AttachesIdentityToContext` and `TestDispatch_AppliesTimeoutToContext` implemented and passing in `registry_test.go`, plus one extra regression test not in the original plan: `TestDispatch_DoesNotClipLongerHandlerOwnedTimeout` (added after TASK-001's implementation surfaced the 60s-vs-5s timeout issue — see that task's Status note). `TestDispatch_EveryRegisteredChannel_AttachesIdentity` left `t.Skip`'d exactly as this doc anticipated — full `RegisterRealChannels` fake-client wiring is a larger follow-up. `go test ./services/api-gateway/... -count=1` clean.

---

## Context

TASK-001 fixes BUG-001 by moving identity attachment into `Dispatch`. The
existing `channels_*_test.go` fake-client tests assert on request-struct
shape, not on `ctx` — they would not have caught BUG-001 (the request
struct was always correct; only the context was missing identity). This
task adds the test class BUG-001's own report flagged as the actual gap.

## Changes to make

**File:** `services/api-gateway/internal/adapter/wscompat/registry_test.go` (new file)

```go
package wscompat

import (
	"context"
	"encoding/json"
	"testing"

	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/grpcmw"
)

// TestDispatch_AttachesIdentityToContext is the direct regression test for
// BUG-001 (specs/backend-go/bugs/missing-v2/) — a handler that reads
// identity back out of ctx via the same outgoing-metadata keys
// gatewaygrpc.AttachIdentity writes must see it, for EVERY registered
// channel, without that channel's own handler needing to attach it.
func TestDispatch_AttachesIdentityToContext(t *testing.T) {
	r := NewRegistry()
	var gotMD metadata.MD
	r.Register("test.echo-identity", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		gotMD, _ = metadata.FromOutgoingContext(ctx)
		return nil, nil
	})

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "test.echo-identity", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := gotMD.Get(grpcmw.MetadataTenantID); len(got) != 1 || got[0] != "tenant-1" {
		t.Errorf("expected tenant_id metadata %q, got %v", "tenant-1", got)
	}
	if got := gotMD.Get(grpcmw.MetadataUserID); len(got) != 1 || got[0] != "user-1" {
		t.Errorf("expected user_id metadata %q, got %v", "user-1", got)
	}
}

// TestDispatch_AppliesTimeoutToContext guards the second half of TASK-001's
// fix — a handler whose ctx has no deadline at all (Dispatch's caller
// passed context.Background()) must see one after Dispatch, matching
// 08-inter-service-communication.md's "deadlines are mandatory."
func TestDispatch_AppliesTimeoutToContext(t *testing.T) {
	r := NewRegistry()
	var hadDeadline bool
	r.Register("test.echo-deadline", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		_, hadDeadline = ctx.Deadline()
		return nil, nil
	})

	if _, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "test.echo-deadline", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hadDeadline {
		t.Error("expected Dispatch to attach a deadline to ctx even when the caller passed none")
	}
}

// TestDispatch_EveryRegisteredChannel_AttachesIdentity is the structural
// guard against a FUTURE BUG-001 — enumerates every channel actually
// registered by RegisterRealChannels (not a hand-maintained list) and
// dispatches each through a shared identity-echoing wrapper, so a new
// channel added later is covered automatically, not only if someone
// remembers to add it to this list by name.
//
// NOTE for implementer: RegisterRealChannels takes real gRPC clients as
// arguments (see channels.go's signature) — wire this test against fake/nil
// clients the same way channels_*_test.go's existing per-group tests
// already do (see e.g. channels_repo_ssh_status_workspace_test.go's
// fakeRepoSshStatusWorkspaceInfraFleetClient pattern), NOT real ones. Every
// dispatched call is expected to either succeed or fail with a
// handler-specific error — what this test checks is ONLY that outgoing
// metadata carries identity by the time the handler's own gRPC client call
// would fire, regardless of what that call then does.
func TestDispatch_EveryRegisteredChannel_AttachesIdentity(t *testing.T) {
	t.Skip("TODO(implementer): wire RegisterRealChannels with the existing " +
		"fake-client fixtures from each channels_*_test.go file, then " +
		"iterate r.handlers (channels.go may need to export a ChannelNames() " +
		"method, or this test can live in package wscompat to reach the " +
		"unexported map directly) dispatching each through a context that " +
		"records whether AttachIdentity's metadata keys were ever set by the " +
		"time the fake client's method was invoked, not just present on the " +
		"top-level ctx Dispatch returns into the handler.")
}
```

The third test is intentionally left `t.Skip`'d with a concrete TODO —
writing it for real requires wiring every existing fake client from every
`channels_*_test.go` file in one place, which is a larger, mechanical
follow-up better done by directly reading the current state of all those
fakes at implementation time rather than guessed here. The first two tests
are the actual, complete regression coverage for BUG-001/TASK-001 and
should be fully passing, not skipped.

## Verify

```bash
cd backend-go
go test ./services/api-gateway/internal/adapter/wscompat/... -count=1 -v -run 'TestDispatch_'
```

Expected: `TestDispatch_AttachesIdentityToContext` and
`TestDispatch_AppliesTimeoutToContext` PASS;
`TestDispatch_EveryRegisteredChannel_AttachesIdentity` SKIP (until a
follow-up task un-skips it per its own TODO).

Then re-run the full suite to confirm TASK-001's identity-attach removal
pass didn't break any existing fake-client test:

```bash
go test ./services/api-gateway/... -count=1
```
