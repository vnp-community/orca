# `notification-service`

Turns events that happen elsewhere in the system into things a user sees:
an in-app WebSocket update for a connected browser client (via
`api-gateway`'s `StreamNotifications` stream), or a mobile push
notification via APNs/FCM. See
[`specs/backend-go/services/notification-service.md`](../../../specs/backend-go/services/notification-service.md)
for the full design, and
[`services/usage-service/README.md`](../usage-service/README.md) for the
package-layout conventions this service follows.

This service is the **primary consumer of the async event bus** — the
inverse of `usage-service`'s publish-only role. See "What's implemented"
below for how `internal/adapter/eventbus/` differs from a typical outbox
publisher.

## What's implemented

- `internal/domain/` — `PushSubscription` (invariant-enforcing
  constructor: Web Push keys required iff `Channel == web`),
  `VapidKeyMetadata` (public key + Vault key-name pointer, no private key
  field ever), `NotificationEvent` plus `TranslateEvent` — a pure,
  subject-driven mapping from a consumed bus event to a user-facing
  notification, with a generic fallback rule for any subject not in the
  known table (so a new publisher doesn't require a code change here).
  Unit-tested without touching NATS/Postgres.
- `internal/usecase/` — `Subscribe`, `GetVapidPublicKey`, and
  `HandleIncomingEvent` (the consumer-side usecase: decode payload,
  translate, broadcast), each tested against in-memory fakes for
  `SubscriptionRepository`/`VapidKeyRepository`/`NotificationBroadcaster`.
  `ports.go` also defines `VaultSigner`, the port `adapter/vaultsigner`
  implements.
- `internal/adapter/broadcaster/` — a **real, working** in-process
  channel-based fan-out registry (`Broadcaster`), keyed by
  tenant+user, safe for concurrent use. `broadcaster_test.go` verifies
  per-user and per-tenant isolation directly (two subscribers for
  different users, one broadcast, only the intended recipient's channel
  receives it) and that unsubscribe correctly removes a dead channel from
  the registry. **Known scaling limitation: see "Known gaps" below.**
- `internal/adapter/eventbus/` — a real `commoneventbus.Consumer.Subscribe`
  loop, one JetStream durable consumer per subject in
  notification-service.md §3's subject table (task/workflow/automation/
  credential/orchestration), each forwarding delivered messages to
  `HandleIncomingEvent`. A subject whose stream doesn't exist yet (the
  publishing service hasn't started) logs a warning and only that one
  binding gives up — it doesn't fail service startup or the other
  subjects.
- `internal/adapter/postgres/` — real `pgx`-backed repository implementing
  both `SubscriptionRepository` (upsert-on-endpoint) and
  `VapidKeyRepository`. Hand-written SQL, same rationale as
  `usage-service`.
- `internal/adapter/vaultsigner/` — implements the `VaultSigner` port
  against `common/secrets.Client.TransitEncrypt`. **Not yet called from
  any RPC path** — wired into the gRPC `Server` composition (stored, ready
  for the future `DeliverPush` usecase) but no mobile-push delivery
  usecase exists in this slice. See "Known gaps".
- `internal/adapter/grpc/` — implements the generated
  `notificationv1.NotificationServiceServer`, including
  `StreamNotifications`: a real, working server-streaming handler that
  registers the request as a broadcaster subscriber and loops sending
  frames until the client disconnects or the server shuts down.
- `migrations/0001_init.{up,down}.sql` — real DDL:
  `notification.push_subscriptions`, `notification.vapid_key_metadata`
  (no `private_key` column, ever), RLS policies matching `usage-service`'s
  pattern.
- `cmd/server/main.go` — composition root: config load, Postgres pool,
  NATS connection (degrades gracefully if unavailable, same as
  `usage-service`), the event-consumer loop started as a background
  goroutine sharing the server's shutdown `ctx` (its `WaitGroup` is waited
  on during graceful shutdown, after `grpcServer.GracefulStop()`), gRPC
  server with the shared interceptor chain, health/readiness HTTP server
  (`/readyz` includes a `vault` checker reporting whether a Vault client
  is configured, without making a live Vault call on every poll).

## Running locally

```sh
# from backend-go/
docker compose up -d postgres nats   # see ../../docker-compose.yml
migrate -path services/notification-service/migrations \
  -database "$DATABASE_DSN" up       # golang-migrate; see architecture/05

cd services/notification-service
DATABASE_DSN=postgres://orca:orca@localhost:5432/notification?sslmode=disable \
NATS_URL=nats://localhost:4222 \
  go run ./cmd/server
```

## Testing

```sh
go test ./...                 # unit tests (domain/, usecase/, adapter/broadcaster/) — no external deps
go test -tags=integration ./internal/adapter/postgres/...   # requires Docker (testcontainers-go)
```

## Known gaps / follow-ups (tracked, not silently skipped)

- **The broadcaster is real, but single-replica only.** `internal/adapter/broadcaster.Broadcaster`
  fans out correctly to every `StreamNotifications` subscriber connected to
  *this* process, but a subscriber connected to a different replica of a
  horizontally-scaled `notification-service` never sees the event — the
  registry lives entirely in one process's memory. Per
  notification-service.md §7, `api-gateway` is stateless-by-design and was
  described as needing "no session-affinity awareness"; that assumption
  breaks once this service itself is scaled beyond one replica. Solving it
  needs either sticky routing at `api-gateway` (pin a user's stream to the
  replica holding their subscription) or a distributed fan-out mechanism
  (e.g. this service also publishing translated `NotificationEvent`s to a
  NATS subject every replica subscribes to, so `Broadcast` becomes
  "publish, then locally fan out on receipt" instead of a direct in-memory
  call). Neither is implemented here — flag before this service is
  deployed with `replicas > 1`.
- **VAPID Transit signing needs a real Vault instance to actually sign.**
  `internal/adapter/vaultsigner.Signer` wraps
  `common/secrets.Client.TransitEncrypt` — construction never fails (Vault
  connectivity isn't checked until a call is made), so the service starts
  cleanly even with `VAULT_ADDR` unset or Vault unreachable. Calling
  `SignVapidPayload` in that state returns a clearly wrapped error rather
  than silently falling back to any local/plaintext signing path — there
  is no local fallback for this key, by design (§9). The `/readyz` `vault`
  checker only reports whether a client object was constructed, not live
  Vault reachability, to avoid making a Transit call on every health poll.
- **No `DeliverPush` usecase in this slice.** `VaultSigner` and the mobile
  push path (`deliver_push.go`, APNs/FCM clients, Web Push protocol
  framing per notification-service.md §6) aren't implemented — this slice
  covers subscription CRUD, VAPID public-key distribution, event
  consumption/translation, and WS fan-out only. `adapter/grpc.Server`
  holds a `VaultSigner` field ready for that usecase to be wired in later.
- **`common/secrets.TransitEncrypt` stands in for a dedicated Transit
  "sign" operation.** `common/secrets` exposes `TransitEncrypt`/
  `TransitDecrypt` today, not Vault's asymmetric-key `sign` endpoint.
  `vaultsigner.Signer` uses `TransitEncrypt` as the available equivalent —
  swap it for a real `transit/sign/<key>` call once `common/secrets` grows
  one.
- **No `processed_events` dedup table.** notification-service.md §5/§8
  describes a `notification.processed_events` table for JetStream
  redelivery dedup; this slice's migrations only cover
  `push_subscriptions`/`vapid_key_metadata`. `HandleIncomingEvent` is not
  currently idempotent against redelivery of the same event ID — add the
  dedup table and a check in `HandleIncomingEvent` before relying on
  at-least-once JetStream delivery not double-broadcasting.
- **No per-user notification-preference filtering** — explicitly out of
  scope per notification-service.md §2; every translated event is
  broadcast to every recipient the payload names, unconditionally.
- **`common/tracing` has no OTLP exporter configured** — same gap
  `usage-service` documents; spans are created but not shipped anywhere
  until a collector endpoint is wired in.
- Full RPC surface: only `Subscribe`, `GetVapidPublicKey`, and
  `StreamNotifications` are implemented, matching the generated proto in
  this repo today (`UnregisterPushSubscription`/`ListPushSubscriptions`
  from the design doc's illustrative API surface aren't in the current
  `.proto` and so aren't implemented here — extend the proto and this
  service's usecase/adapter layers together if that surface is needed).
