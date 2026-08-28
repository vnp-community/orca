# TASK-MB-04-06: Add `mobile.status` (pull) + `mobile.statusSubscribe` (poll-and-diff push) `wscompat` channels

**From Solution:** SOL-MB-04
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_mobile_status.go` (new), `backend-go/services/api-gateway/cmd/server/main.go`
**Depends on:** TASK-MB-04-05, TASK-MB-03-06 (`RegisterMobileChannels`/`DeviceSecretResolver`/`errNotAMobileSession`/`Identity.DeviceID`)
**Status:** `[x]` DONE — added `channels_mobile_status.go` (`mobile.status` pull + `mobile.statusSubscribe` poll-and-diff push, `mobileStatusPollInterval` as a `var` per `channels_accounts.go`'s testability precedent), extended `RegisterMobileChannels` (TASK-MB-03-06) to also call `registerMobileStatusChannels` — no second composition-root call site; `sealMobileEnvelope` lives in the shared `mobile_envelope.go`. `go build`/`go vet` clean; `channels_mobile_status_test.go` covers non-mobile-Identity rejection before any RPC, the sealed-envelope-always response shape (decrypts back to the exact worktree), and the two-identical-polls-produce-exactly-one-`PushEvent` regression guard — all pass.

---

## Context

BR-MB-13 requires the response always be the E2E-sealed envelope, never
raw JSON. BR-MB-14 (live update while foregrounded) reuses
`channels_push.go`'s `RegisterStream` pattern (`notifications.subscribe`'s
proven shape) rather than inventing a new bridging primitive — poll-and-diff
against `project-service`, not a true event stream, since worktree/agent
status changes aren't themselves published as domain events. The mobile
client enforces "foreground only" by closing the stream on backgrounding;
the server does no foreground-detection of its own, per `api-gateway.md`
§2's stateless-by-design principle.

## Changes to make

`backend-go/services/api-gateway/internal/adapter/wscompat/channels_mobile_status.go`:

```go
package wscompat

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
)

const mobileStatusPollInterval = 5 * time.Second

func registerMobileStatusChannels(r *Registry, client projectv1.ProjectServiceClient, devices DeviceSecretResolver) {
	// BR-MB-16's pull-to-refresh.
	r.Register("mobile.status", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		if id.DeviceID == "" {
			return nil, errNotAMobileSession // mirrors mobile.dispatch (TASK-MB-03-06)
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.GetMobileWorktreeStatus(ctx, &projectv1.GetMobileWorktreeStatusRequest{})
		if err != nil {
			return nil, err
		}
		secret, err := devices.ResolveSharedSecret(ctx, id.DeviceID) // BR-MB-13: encrypt in transit
		if err != nil {
			return nil, err
		}
		return sealMobileEnvelope(resp, secret) // shared helper with channels_mobile_dispatch.go's unsealMobilePayload — factor into one file
	})

	// BR-MB-14's live update while foregrounded. Poll-and-diff: only sends
	// a frame when the computed view actually changed since the last poll.
	r.RegisterStream("mobile.statusSubscribe", func(ctx context.Context, id Identity, args []json.RawMessage) (<-chan PushEvent, error) {
		if id.DeviceID == "" {
			return nil, errNotAMobileSession
		}
		secret, err := devices.ResolveSharedSecret(ctx, id.DeviceID)
		if err != nil {
			return nil, err
		}
		out := make(chan PushEvent)
		go pollAndDiffMobileStatus(ctx, client, id, secret, out)
		return out, nil
	})
}

func pollAndDiffMobileStatus(ctx context.Context, client projectv1.ProjectServiceClient, id Identity, secret []byte, out chan<- PushEvent) {
	defer close(out)
	ticker := time.NewTicker(mobileStatusPollInterval)
	defer ticker.Stop()
	var lastSent *projectv1.GetMobileWorktreeStatusResponse
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			gctx := gatewaygrpc.AttachIdentity(ctx, Identity{TenantID: id.TenantID, UserID: id.UserID})
			resp, err := client.GetMobileWorktreeStatus(gctx, &projectv1.GetMobileWorktreeStatusRequest{})
			if err != nil {
				continue // best-effort poll — a transient error just skips this tick
			}
			if lastSent != nil && worktreeStatusesEqual(lastSent, resp) {
				continue // no change — regression guard: two consecutive identical polls must produce exactly one PushEvent, not two
			}
			envelope, err := sealMobileEnvelope(resp, secret)
			if err != nil {
				continue
			}
			select {
			case out <- PushEvent{Channel: "mobile.statusEvent", Args: []any{envelope}}:
				lastSent = resp
			case <-ctx.Done():
				return
			}
		}
	}
}

func worktreeStatusesEqual(a, b *projectv1.GetMobileWorktreeStatusResponse) bool {
	return reflect.DeepEqual(a.GetWorktrees(), b.GetWorktrees()) // GeneratedAtUnixMs deliberately excluded from the comparison
}
```

`sealMobileEnvelope(resp proto.Message, secret []byte) (any, error)` should
marshal `resp` to JSON, NaCl-secretbox-seal it with `secret`, and return
`{ciphertext, nonce}` (base64) — factor as a shared helper with
`channels_mobile_dispatch.go`'s decrypt-side counterpart (TASK-MB-03-06),
in one small shared file (e.g. `mobile_envelope.go`) rather than duplicated
in both channel files.

Call `registerMobileStatusChannels(r, projectClient, deviceSecretResolver)`
from `RegisterMobileChannels` (extend TASK-MB-03-06's function, don't add a
second composition-root call site).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/... && go vet ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/wscompat/... -run MobileStatus
```

Test cases: non-mobile `Identity` (no `DeviceID`) rejected before any RPC
call. Response body is always the sealed envelope, never raw
`MobileWorktreeStatus` JSON (assert on the channel's raw return value
shape, not just that encryption was "called"). `mobile.statusSubscribe`:
two consecutive identical polls produce exactly one `PushEvent` (the
first), not two.
