# TASK-MB-02-08: Add APNs/FCM adapters (own credential, not VAPID) + drain buffered notifications on `StreamNotifications` reconnect

**From Solution:** SOL-MB-02
**Priority:** P1
**Service:** `notification-service`
**File:** `backend-go/services/notification-service/internal/adapter/external/apns/client.go` (new), `backend-go/services/notification-service/internal/adapter/external/fcm/client.go` (new), `backend-go/services/notification-service/internal/adapter/grpc/server.go`
**Depends on:** TASK-MB-02-07
**Status:** `[x]` DONE — real `apns.Client`/`fcm.Client` implemented: ES256 provider-JWT (APNs) / RS256 service-account JWT assertion (FCM), both Transit-signed via `*secrets.Client.TransitSign` directly (no wrapper needed — same credential class as `SharedSecretSealer`/`VaultSigner`, distinct Transit key names `apns-provider-key`/`fcm-service-account-key`, never VAPID), real HTTP/2 (APNs) and HTTP v1 (FCM OAuth2 token exchange + send) request/response handling, error classification (permanent vs transient status codes documented). `DeliverPush.deliverOne` dispatches ios/android to their own credential, never `vapidSigner` (regression-tested). `webpush.Client` implements genuine RFC 8291 aes128gcm encryption (ECDH P-256 + HKDF-SHA256 + AES-128-GCM) — round-trip-verified against an independent receiver-side re-derivation in `client_test.go`. `StreamNotifications` reconnect-drain implemented and tested (`server_test.go`). **Remains genuinely externally-dependent**: needs a real Apple Push Auth Key (.p8) provisioned as Vault Transit ES256 key `apns-provider-key`, plus a real Firebase service-account key provisioned as Vault Transit RSA key `fcm-service-account-key`, to actually deliver a push in production — both left unset in this environment (`APNS_TEAM_ID`/`APNS_KEY_ID`/`APNS_TOPIC`/`FCM_PROJECT_ID`/`FCM_SERVICE_ACCOUNT_EMAIL` all empty by default); the Go-side call path (JWT construction, DER→JWS conversion verified against `ecdsa.Verify`, Vault Transit signing, HTTP request/response handling) is complete, real, and unit-tested with fake `TransitSigner`/HTTP transports plus real cryptographic round-trip assertions — not a stub.

---

## Context

`notification-service.md` §1's diagram labels the push path uniformly
"VAPID-signed Web Push -> APNs/FCM" — an imprecision: native APNs needs a
provider JWT signed with an ES256 `.p8` key, FCM needs an OAuth2
service-account token, neither of which is VAPID. VAPID only applies to
the `web` channel. This task adds `ios`/`android` as their own
credential class, mediated through the same Transit-backed pattern
`SharedSecretSealer` (SOL-MB-01) and `VaultSigner` (existing) already
use — never a raw signing key in this service's process.

**Real APNs/FCM delivery needs actual Apple/Google push credentials
provisioned in Vault** — an operational/product prerequisite outside
backend-go code. This task wires the Go-side call path against those
credential names; it does not provision the credentials themselves.

## Changes to make

`backend-go/services/notification-service/internal/adapter/external/apns/client.go`:

```go
// Package apns implements usecase.APNsClient — signs a provider JWT with
// an ES256 .p8 key via Vault Transit (own credential, distinct from the
// web channel's VAPID key — see this task's Context), then POSTs to
// Apple's HTTP/2 push gateway.
package apns

const apnsSigningKeyName = "apns-provider-key" // Vault Transit ES256 key; provisioning is an operational prerequisite, not this task's scope

type Client struct {
	signer     TransitSigner // same Vault Transit mediation as vaultsigner.VaultSigner, a distinct key name
	httpClient *http.Client
	endpoint   string // configurable: sandbox vs production APNs gateway
}

func New(signer TransitSigner, endpoint string) *Client { return &Client{signer: signer, httpClient: &http.Client{}, endpoint: endpoint} }

func (c *Client) Send(ctx context.Context, deviceToken string, ciphertext, nonce []byte) error {
	jwt, err := c.signer.Sign(ctx, apnsSigningKeyName, apnsProviderClaims())
	if err != nil {
		return fmt.Errorf("apns: signing provider jwt: %w", err)
	}
	// POST to c.endpoint/3/device/{deviceToken}, Authorization: bearer {jwt},
	// body: {"ciphertext": base64(ciphertext), "nonce": base64(nonce)} —
	// the mobile app decrypts client-side with the same shared secret
	// (SOL-MB-01), same as the web channel's E2E layer.
	return nil // fill in real HTTP/2 call
}
```

`backend-go/services/notification-service/internal/adapter/external/fcm/client.go`:

```go
// Package fcm implements usecase.FCMClient — an OAuth2 service-account
// token (own credential, NOT VAPID), obtained via Vault Transit-mediated
// signing of the service-account JWT assertion, then a standard FCM HTTP
// v1 send call.
package fcm

const fcmServiceAccountKeyName = "fcm-service-account-key" // provisioning is an operational prerequisite, not this task's scope

type Client struct {
	signer     TransitSigner
	httpClient *http.Client
	projectID  string
}

func New(signer TransitSigner, projectID string) *Client { return &Client{signer: signer, httpClient: &http.Client{}, projectID: projectID} }

func (c *Client) Send(ctx context.Context, registrationToken string, ciphertext, nonce []byte) error {
	// Exchange a Transit-signed service-account JWT assertion for an OAuth2
	// access token, then POST to
	// https://fcm.googleapis.com/v1/projects/{projectID}/messages:send with
	// {"message": {"token": registrationToken, "data": {"ciphertext":
	// base64(ciphertext), "nonce": base64(nonce)}}}.
	return nil // fill in real HTTP call
}
```

Extend `DeliverPush.deliverOne` (TASK-MB-02-07) to dispatch on channel:

```go
func (uc *DeliverPush) deliverOne(ctx context.Context, event domain.NotificationEvent, sub domain.PushSubscription) error {
	plaintext := mustJSON(event)
	deviceID, err := uc.subscriptions.DeviceIDFor(ctx, sub.ID)
	if err != nil || deviceID == "" {
		if sub.Channel != domain.ChannelWeb {
			return domain.ErrUnsupportedChannel // iOS/Android always require a paired device — no non-E2E fallback for native push
		}
		// ... existing web fallback from TASK-MB-02-07 ...
	}
	secret, err := uc.devices.ResolveSharedSecret(ctx, deviceID)
	if err != nil {
		return err
	}
	ciphertext, nonce, err := uc.sealer.Seal(plaintext, secret)
	if err != nil {
		return err
	}
	switch sub.Channel {
	case domain.ChannelWeb:
		jwt, err := uc.vapidSigner.SignVapidPayload(ctx, event.TenantID, ciphertext)
		if err != nil {
			return err
		}
		return uc.webpush.Send(ctx, sub.Endpoint, sub.P256dhKey, sub.AuthKey, ciphertext, nonce, jwt)
	case domain.ChannelIOS:
		return uc.apns.Send(ctx, sub.Endpoint, ciphertext, nonce) // own APNs credential — NOT vapidSigner
	case domain.ChannelAndroid:
		return uc.fcm.Send(ctx, sub.Endpoint, ciphertext, nonce) // own FCM credential — NOT vapidSigner
	default:
		return domain.ErrUnsupportedChannel
	}
}
```

Add `apns APNsClient` / `fcm FCMClient` fields + constructor params to
`DeliverPush`.

**`StreamNotifications` reconnect draining** — in
`internal/adapter/grpc/server.go`'s `StreamNotifications` handler, before
entering the live `broadcaster.Subscribe` loop:

```go
pending, err := s.buffer.ListPending(stream.Context(), tenantID, req.GetUserId()) // TASK-MB-02-06
if err == nil {
	delivered := make([]string, 0, len(pending))
	for _, row := range pending {
		if err := stream.Send(row.ToStreamResponse()); err == nil {
			delivered = append(delivered, row.ID)
		}
	}
	_ = s.buffer.MarkDelivered(stream.Context(), delivered)
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/notification-service/... && go vet ./services/notification-service/...
go test ./services/notification-service/internal/usecase/... -run DeliverPush
go test ./services/notification-service/internal/adapter/grpc/... -run StreamNotifications
```

Test: iOS/Android channels call `apns.Send`/`fcm.Send`, never
`vapidSigner.SignVapidPayload` (regression guard against the VAPID/APNs
conflation this task resolves). `StreamNotifications` reconnect: buffered
rows for the reconnecting user are sent before the live loop starts, each
marked `delivered_at` afterward.
