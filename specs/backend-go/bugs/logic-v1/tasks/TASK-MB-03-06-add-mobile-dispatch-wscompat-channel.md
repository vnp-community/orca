# TASK-MB-03-06: Add `mobile.dispatch` `wscompat` channel (E2E decrypt + `DispatchPrompt` relay)

**From Solution:** SOL-MB-03
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_mobile_dispatch.go` (new), `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go`, `backend-go/services/api-gateway/cmd/server/main.go`
**Depends on:** TASK-MB-03-05, SOL-MB-01 (`ResolveDeviceSharedSecret`, TASK-MB-01-06/07), SOL-MB-02's `authclient` (TASK-MB-02-07)
**Status:** `[ ]` TODO

---

## Context

`api-gateway.md` §2 keeps business logic (the gate/queue decision) out of
`wscompat` — this channel only decrypts, then relays to
`infra-fleet-service.DispatchPrompt`, which owns BR-MB-09..12. `Identity`
(`registry.go`) currently carries only `TenantID`/`UserID` — this task adds
`DeviceID`, threaded from the mobile JWT's claims (SOL-MB-01's
`CompleteDevicePairing`/`IssueToken.ExecuteForDevice` mints it). A plain
browser session's `Identity` has no `DeviceID`, so `mobile.*` channels stay
unreachable from a browser session.

## Changes to make

In `registry.go`, extend `Identity`:

```go
type Identity struct {
	TenantID string
	UserID   string
	DeviceID string // non-empty only for a mobile-paired-device JWT (SOL-MB-01) — mobile.* channels require this
}
```

Thread `DeviceID` through wherever `Identity` is constructed from a
validated JWT today (the same JWT-validation path `api-gateway.md` §9
already uses for mobile/CLI tokens) — read the device_id claim if present.

`backend-go/services/api-gateway/internal/adapter/wscompat/channels_mobile_dispatch.go`:

```go
// mobile.* channels — kept in this SEPARATE file, following
// channels_push.go's established convention, so these edits never touch
// the shared, high-churn channels.go.
package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

var errNotAMobileSession = errors.New("wscompat: this channel requires a paired-device (mobile) identity")

type DeviceSecretResolver interface {
	ResolveSharedSecret(ctx context.Context, deviceID string) ([]byte, error)
}

type mobileDispatchArgs struct {
	PtyID         string `json:"ptyId"`
	EncryptedBody string `json:"encryptedBody"` // base64 NaCl secretbox ciphertext
	Nonce         string `json:"nonce"`
	Overwrite     bool   `json:"overwrite"`
}

type dispatchOutcomeView struct {
	Outcome                     string `json:"outcome"`
	ExistingQueuedPromptPreview string `json:"existingQueuedPromptPreview,omitempty"`
}

func RegisterMobileChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient, devices DeviceSecretResolver) {
	registerMobileDispatchChannel(r, client, devices)
}

func registerMobileDispatchChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient, devices DeviceSecretResolver) {
	r.Register("mobile.dispatch", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[mobileDispatchArgs](args, 0)
		if err != nil {
			return nil, err
		}
		if id.DeviceID == "" {
			return nil, errNotAMobileSession
		}
		secret, err := devices.ResolveSharedSecret(ctx, id.DeviceID)
		if err != nil {
			return nil, err
		}
		prompt, err := unsealMobilePayload(in.EncryptedBody, in.Nonce, secret)
		if err != nil {
			return nil, fmt.Errorf("wscompat: decrypting mobile dispatch payload: %w", err)
		}

		ctx = gatewaygrpc.AttachIdentity(ctx, Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.DispatchPrompt(ctx, &infrafleetv1.DispatchPromptRequest{
			PtyId: in.PtyID, Prompt: prompt, Overwrite: in.Overwrite, DispatchedByDeviceId: id.DeviceID,
		})
		if err != nil {
			return nil, err
		}
		return dispatchOutcomeView{
			Outcome:                     resp.GetOutcome().String(),
			ExistingQueuedPromptPreview: resp.GetExistingQueuedPromptPreview(),
		}, nil
	})
}
```

`unsealMobilePayload` should live alongside (or reuse) whatever NaCl
secretbox-open helper `channels_mobile_status.go` (TASK-MB-04-06) also
needs — factor a shared `wscompat`-internal helper rather than duplicating
the base64-decode + open call in two files.

In `cmd/server/main.go`'s composition root, call
`wscompat.RegisterMobileChannels(registry, infraFleetClient, deviceSecretResolver)`
alongside the existing `RegisterRealChannels`/`RegisterPushChannels` calls.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/... && go vet ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/wscompat/... -run MobileDispatch
```

Test cases: a session with no `DeviceID` (plain browser JWT) rejected
before any decrypt attempt (assert `ResolveSharedSecret` NOT called).
Malformed/garbage ciphertext → decode error, `DispatchPrompt` never called
(assert fake gRPC client received zero calls). Valid encrypted payload
round-trips to a `DispatchPromptRequest` with the correctly decrypted
plaintext prompt.
