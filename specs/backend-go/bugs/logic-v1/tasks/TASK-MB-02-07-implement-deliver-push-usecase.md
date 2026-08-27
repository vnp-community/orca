# TASK-MB-02-07: Implement `DeliverPush` usecase (web/VAPID channel, buffering, preferences)

**From Solution:** SOL-MB-02
**Priority:** P1
**Service:** `notification-service`
**File:** `backend-go/services/notification-service/internal/usecase/deliver_push.go` (new), `backend-go/services/notification-service/internal/adapter/grpcclient/authclient/device_secret_resolver.go` (new)
**Depends on:** TASK-MB-02-05, TASK-MB-02-06, SOL-MB-01 (auth-service `ResolveDeviceSharedSecret`, TASK-MB-01-06/07)
**Status:** `[x]` DONE — `DeliverPush` usecase implemented (web/VAPID + paired-device NaCl E2E seal + buffering + preferences), `authclient.DeviceSecretResolver` (real gRPC client to auth-service's `ResolveDeviceSharedSecret`), `nacl.Sealer` (real `secretbox`-based `E2ESealer`, round-trip-tested against `secretbox.Open`), wired into `HandleIncomingEvent` (previously WS-only) and `cmd/server/main.go`. `signer usecase.VaultSigner` is now actually invoked (previously wired-but-unused). Unit tests cover all 5 Verify-section scenarios (no-paired-device signs without sealing; paired-device seals-then-signs, ciphertext non-nil; delivery failure buffers once, success doesn't; disabled preference skips delivery) — all pass.

---

## Context

`notification-service.md` §6 already names `deliver_push.go` in its
planned package layout; `signer usecase.VaultSigner` is already a field on
`adapter/grpc.Server` today, wired but never called (its doc comment says
so explicitly) — this task is what finally calls it, for the `web` channel
only. iOS/Android are wired in TASK-MB-02-08 (their own credential class,
NOT VAPID — see that task).

## Changes to make

`backend-go/services/notification-service/internal/adapter/grpcclient/authclient/device_secret_resolver.go`:

```go
// Package authclient implements usecase.DeviceSecretResolver by dialing
// auth-service's internal-only ResolveDeviceSharedSecret RPC (SOL-MB-01) —
// never routed through api-gateway's REST facade.
package authclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

type DeviceSecretResolver struct {
	conn   *grpc.ClientConn
	client authv1.AuthServiceClient
}

func New(addr string) (*DeviceSecretResolver, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("authclient: dial auth-service at %q: %w", addr, err)
	}
	return &DeviceSecretResolver{conn: conn, client: authv1.NewAuthServiceClient(conn)}, nil
}

func (c *DeviceSecretResolver) Close() error { return c.conn.Close() }

func (c *DeviceSecretResolver) ResolveSharedSecret(ctx context.Context, deviceID string) ([]byte, error) {
	resp, err := c.client.ResolveDeviceSharedSecret(ctx, &authv1.ResolveDeviceSharedSecretRequest{DeviceId: deviceID})
	if err != nil {
		return nil, fmt.Errorf("authclient: resolve device shared secret: %w", err)
	}
	return resp.GetSharedSecret(), nil
}
```

`backend-go/services/notification-service/internal/usecase/deliver_push.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

type DeviceSecretResolver interface {
	ResolveSharedSecret(ctx context.Context, deviceID string) ([]byte, error)
}

// E2ESealer NaCl-secretbox-encrypts a push payload with a paired device's
// shared secret (SOL-MB-01) — BR-MB-05: encrypted before it ever crosses
// the network.
type E2ESealer interface {
	Seal(plaintext []byte, sharedSecret []byte) (ciphertext, nonce []byte, err error)
}

type WebPushClient interface {
	Send(ctx context.Context, endpoint, p256dh, auth string, ciphertext, nonce []byte, vapidJWT string) error
}

type DeliverPush struct {
	subscriptions SubscriptionRepository
	devices       DeviceSecretResolver
	sealer        E2ESealer
	vapidSigner   VaultSigner
	webpush       WebPushClient
	buffer        BufferedNotificationRepository
	preferences   NotificationPreferenceRepository
}

func NewDeliverPush(subs SubscriptionRepository, devices DeviceSecretResolver, sealer E2ESealer, vapidSigner VaultSigner, webpush WebPushClient, buffer BufferedNotificationRepository, preferences NotificationPreferenceRepository) *DeliverPush {
	return &DeliverPush{subscriptions: subs, devices: devices, sealer: sealer, vapidSigner: vapidSigner, webpush: webpush, buffer: buffer, preferences: preferences}
}

func (uc *DeliverPush) Execute(ctx context.Context, event domain.NotificationEvent) error {
	hasPush := false
	for _, ch := range event.Channels {
		if ch == domain.ChannelDeliveryPush {
			hasPush = true
		}
	}
	if !hasPush {
		return nil
	}
	for _, userID := range event.RecipientUserIDs {
		subs, err := uc.subscriptions.ListByUser(ctx, event.TenantID, userID)
		if err != nil {
			continue
		}
		for _, sub := range subs {
			allowed, err := uc.preferences.IsEnabled(ctx, event.TenantID, userID, event.Type, string(sub.Channel)) // BR-MB-08
			if err != nil || !allowed {
				continue
			}
			if err := uc.deliverOne(ctx, event, sub); err != nil {
				payloadJSON := mustJSON(event)
				_ = uc.buffer.Enqueue(ctx, event.TenantID, userID, sub.ID, payloadJSON) // BR-MB-07
			}
		}
	}
	return nil
}

func (uc *DeliverPush) deliverOne(ctx context.Context, event domain.NotificationEvent, sub domain.PushSubscription) error {
	if sub.Channel != domain.ChannelWeb {
		return domain.ErrUnsupportedChannel // iOS/Android handled by TASK-MB-02-08's extended deliverOne
	}
	plaintext := mustJSON(event)
	deviceID, err := uc.subscriptions.DeviceIDFor(ctx, sub.ID) // add this method to SubscriptionRepository (TASK-MB-02-06's postgres store) — empty/error means "no paired device"
	if err != nil || deviceID == "" {
		// Standard Web Push (RFC 8291) encryption only, no BL-MB-01 E2E
		// layer — a web-channel subscription with no paired device is not a
		// mobile-companion flow.
		jwt, err := uc.vapidSigner.SignVapidPayload(ctx, event.TenantID, plaintext)
		if err != nil {
			return err
		}
		return uc.webpush.Send(ctx, sub.Endpoint, sub.P256dhKey, sub.AuthKey, plaintext, nil, jwt)
	}
	secret, err := uc.devices.ResolveSharedSecret(ctx, deviceID)
	if err != nil {
		return err
	}
	ciphertext, nonce, err := uc.sealer.Seal(plaintext, secret)
	if err != nil {
		return err
	}
	jwt, err := uc.vapidSigner.SignVapidPayload(ctx, event.TenantID, ciphertext)
	if err != nil {
		return err
	}
	return uc.webpush.Send(ctx, sub.Endpoint, sub.P256dhKey, sub.AuthKey, ciphertext, nonce, jwt)
}
```

`mustJSON` should reuse whatever JSON-framing helper `StreamNotifications`'s
gRPC handler already uses to shape a `NotificationEvent` for the wire
(check `adapter/grpc/server.go`/`adapter/broadcaster` before adding a new
one). Add `DeviceIDFor(ctx, subscriptionID string) (string, error)` to
`SubscriptionRepository`'s interface and its `postgres` implementation
(requires a `device_id` column on `push_subscriptions`, nullable — a
migration this task also adds, `0004_push_subscriptions_device_id.up.sql`).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/notification-service/... && go vet ./services/notification-service/...
go test ./services/notification-service/internal/usecase/... -run DeliverPush
```

Test cases: web channel with no paired device signs+sends without E2E seal
(assert `sealer.Seal` NOT called); web channel with a paired device seals
first, then signs the ciphertext (assert `webpush.Send` receives non-nil
`ciphertext`/`nonce`, never the plaintext event body — BR-MB-05); delivery
failure → `buffer.Enqueue` called once; success → not called; a disabled
preference row → `deliverOne` never called for that subscription (BR-MB-08).
