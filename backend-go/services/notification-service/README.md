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
  subjects. A binding with `Durable` set (currently
  `orca.infrafleet.terminal.closed` on stream `INFRAFLEET`, durable
  `notification-service-infrafleet-terminal-closed`, and the legacy
  `orca.mcp.terminal.idlestopped` on stream `MCP`, durable
  `notification-service-mcp-terminal-idle-stopped`, kept only to drain in-flight
  messages) uses a named
  durable consumer instead of an ephemeral one, so an event published while the
  service was down is delivered after it starts. That rule is `Locked`: title,
  body and deep link come from the rule (+ a sanitized `client_name`), never from
  the payload, WS-only, deep link `/?section=mcp&tab=connect`. Its event id must be
  a UUID (`processed_events.event_id`). Shared durable cursor: only one replica
  broadcasts live; the stored notification is visible to all.
  The lossless source is infra-fleet-service: it writes
  `orca.infrafleet.terminal.closed` to its outbox in the same transaction that
  closes the terminal, so the event survives gateway crashes and NATS outages.
  Only `reason=="idle"` with `origin.type=="mcp"` translates to
  `mcp.terminal.idle_stopped` (same locked rule, `domain.ErrNotNotifiable` acks
  every other close without a notification); `processed_events` makes redelivery
  idempotent. Remaining gap: if the `INFRAFLEET` stream does not exist when this
  service starts, that binding gives up until restart.
- `internal/adapter/postgres/` — real `pgx`-backed repository implementing
  both `SubscriptionRepository` (upsert-on-endpoint) and
  `VapidKeyRepository`. Hand-written SQL, same rationale as
  `usage-service`.
- `internal/adapter/vaultsigner/` — implements the `VaultSigner` port
  against a real `credential-broker-service` connection's `SignVapidPayload`
  RPC (Epic B, 2026-08-17 — previously called `common/secrets.Client.TransitEncrypt`
  directly; see "credential-broker-service is wired" below). Called by
  `external/webpush.VapidAuthorizer` (CR-NOTIF-002): only the JWT signing input
  goes to the broker; the Vault signature (ASN.1 DER) is converted to raw
  R||S locally. No VAPID private key ever enters this process.
- `internal/adapter/grpc/` — implements the generated
  `notificationv1.NotificationServiceServer`, including
  `StreamNotifications`: a real, working server-streaming handler that
  registers the request as a broadcaster subscriber and loops sending
  frames until the client disconnects or the server shuts down.
- `migrations/postgres/` and `migrations/mysql/` — dialect-specific DDL for
  `push_subscriptions`, `vapid_key_metadata` (no `private_key` column,
  ever), `processed_events`, `buffered_notifications`,
  `notification_preferences`, `notification_events`. RLS policies exist on
  the Postgres side only — no equivalent exists in MySQL, see the mysql
  migrations' own comments and `TASK-BE-DB-003`'s finding that this was
  never an active backstop on Postgres either (CR-DB-002/CR-DB-003 rollout,
  `BE-DB-SOL-008`).
- `cmd/server/main.go` — composition root: config load, Postgres pool,
  NATS connection (degrades gracefully if unavailable, same as
  `usage-service`), the event-consumer loop started as a background
  goroutine sharing the server's shutdown `ctx` (its `WaitGroup` is waited
  on during graceful shutdown, after `grpcServer.GracefulStop()`), gRPC
  server with the shared interceptor chain, health/readiness HTTP server.

## `credential-broker-service` is wired (Epic B, 2026-08-17)

This service used to call `common/secrets.Client.TransitEncrypt` directly
against Vault for VAPID signing — the one service in this scaffold with a
documented exception to
[`architecture/06-secrets-vault-architecture.md`](../../../specs/backend-go/architecture/06-secrets-vault-architecture.md)'s
"no other service talks to Vault directly for tenant secret material" rule
(that doc's own "What goes in Vault" table said as much, in direct
contradiction with its "`credential-broker-service`'s role" section a few
paragraphs down — a real, documented inconsistency, not a hypothetical
one). `internal/adapter/vaultsigner.Signer` now calls
`credential-broker-service`'s `SignVapidPayload` RPC instead — this
service no longer constructs a Vault client or imports `common/secrets` at
all. The Transit key name convention (`"vapid-signing-" + tenantID`) is
unchanged, so existing tenant keys keep working; only the process
boundary moved. See `credentialbroker.proto`'s doc comment on
`SignVapidPayload` for why it's a narrow, purpose-named RPC rather than a
generic Transit passthrough.

## Running locally

```sh
# from backend-go/
docker compose up -d postgres nats   # see ../../docker-compose.yml
migrate -path services/notification-service/migrations/postgres \
  -database "$DATABASE_DSN" up       # golang-migrate; see architecture/05

cd services/notification-service
DATABASE_DSN=postgres://orca:orca@localhost:5432/notification?sslmode=disable \
NATS_URL=nats://localhost:4222 \
  go run ./cmd/server
```

MySQL/TiDB instead of Postgres: run `migrate -path
services/notification-service/migrations/mysql -database "$DATABASE_DSN" up`
against a database named `notification`, and set
`DATABASE_DSN=mysql://root:orca@tcp(localhost:3306)/notification` (or
`tidb://...`) before `go run ./cmd/server` — `cmd/server/main.go` picks the
adapter from the DSN scheme, same factory pattern as `usage-service` (the
pilot).

## Testing

```sh
go test ./...                 # unit tests (domain/, usecase/, adapter/broadcaster/) — no external deps
go test -tags=integration ./internal/adapter/postgres/...   # requires Docker (testcontainers-go)
```

## Known gaps / follow-ups (tracked, not silently skipped)

- **The broadcaster is real and now correct across replicas — Epic F
  resolved this** (docs/execution-plan.md §2 Epic F, 2026-08-17).
  `internal/adapter/broadcaster.Broadcaster` itself is still purely
  in-process (`Broadcast` only ever reaches subscribers connected to *this*
  process) — that part didn't need to change. What was actually broken:
  `internal/adapter/eventbus.Consumer` used to give every replica the SAME
  durable JetStream consumer name for each subscribed subject, which
  JetStream treats as one shared work queue — only ONE replica ever
  received a given domain event, so only that replica's `Broadcast` call
  ever fired. Fixed by switching to
  `commoneventbus.Consumer.SubscribeEphemeral` (new in this pass): each
  replica gets its own private, unnamed JetStream consumer, so every
  replica independently receives every event and calls `Broadcast` against
  its own locally-connected subscribers — cluster-wide fan-out achieved at
  the NATS layer, no new subject, no republish hop, no sticky-routing
  requirement on `api-gateway`. Trade-off, documented rather than hidden: an
  ephemeral consumer has no durable cursor, so a replica that was down when
  an event was published never catches up on it after restarting — correct
  for this service's already-stated no-offline-replay-queue posture (§2),
  not a new gap.
- **VAPID Transit signing needs a real Vault instance behind
  `credential-broker-service` to actually sign.** `SignVapidPayload` returns
  a clearly wrapped error if `credential-broker-service` is unreachable or
  its own Vault connection fails — there is no local fallback for this key,
  by design (§9). See `credential-broker-service`'s own README for the
  Vault-side detail (`common/secrets.TransitEncrypt` standing in for a
  dedicated Transit "sign" operation is now that service's gap to track,
  not this one's).
- ~~**No `DeliverPush` usecase.**~~ — **closed** (CR-NOTIF-002).
  `usecase.DeliverPush` sends to every active subscription of an event's
  recipients when `Channels` contains `push`, after `Broadcast` and without
  failing the event. Web payload is exactly `{title, body, deepLink, tag}`
  (`domain.ToPushMessage`); `mcp.approval` bodies are pinned to a fixed string
  so tool arguments never reach a push service. RFC 8291 encryption and
  RFC 8030 `TTL`/`Urgency` live in `adapter/external/webpush` (stdlib
  `crypto/ecdh` + `x/crypto/hkdf`, no Web Push library: they sign VAPID with a
  local key, which the Vault-only rule forbids). 404/410 -> `MarkExpired`.
  Deliveries are bounded (`NOTIFICATION_PUSH_CONCURRENCY`, default 8) with a
  per-call deadline (`NOTIFICATION_PUSH_TIMEOUT`, default 5s); endpoints are
  logged as host only. Metrics: `orca_notification_push_deliveries_total`,
  `orca_notification_push_delivery_seconds` on the HTTP port's `/metrics`.
  Env: `VAPID_SUBJECT` (required for web push; mailto:/https: contact).
  Per-tenant VAPID keys are provisioned automatically (`usecase.EnsureVapidKey`):
  when `GetVapidPublicKey` or the VAPID authorizer used by `DeliverPush` finds no
  active key row, it calls credential-broker-service `EnsureVapidSigningKey`
  (creates the `ecdsa-p256` Transit key `vapid-signing-<tenant_id>`), then
  `InsertActiveIfAbsent` (Postgres `ON CONFLICT DO NOTHING`, MySQL plain INSERT
  ignoring duplicate-key on `idx_vapid_key_active`) and re-reads, so concurrent
  callers share one in-process attempt and replicas converge on one row. A Vault
  403 surfaces as `NOTIFICATION_NO_VAPID_KEY` with a message to apply
  `deploy/dev/orca-policy.hcl`; other failures as
  `NOTIFICATION_VAPID_KEY_FETCH_FAILED`. Failures are cached per tenant (30s for
  a Vault 403, 5s otherwise) so a missing policy does not hammer Vault. Metric:
  `orca_notification_vapid_provision_total{outcome=created|existing|forbidden|error}`.
  `deploy/dev/scripts/provision-vapid-key.sh` remains as an optional pre-warm;
  the Vault policy step is still required. Remaining gaps: failed non-410 sends are buffered (BR-MB-07) but there is
  no timed retry; no cross-event push dedup on restart (CR-NOTIF-001 item D).
- ~~**No `processed_events` dedup table.**~~ — **closed**
  (docs/execution-plan.md §3 Phase 1): `migrations/0002_processed_events.{up,down}.sql`
  adds `notification.processed_events` exactly per notification-service.md
  §5/§8 (`event_id UUID PRIMARY KEY`, `subject`, `processed_at`, indexed on
  `processed_at` for future pruning). `usecase.ProcessedEventRepository`'s
  `MarkProcessed` is implemented in `internal/adapter/postgres` as a single
  `INSERT ... ON CONFLICT DO NOTHING` — an atomic reserve, not a racy
  check-then-insert, because `SubscribeEphemeral` gives every replica its
  own independent JetStream consumer (Epic F), so a redelivered message can
  race across replicas, not just within one process. `HandleIncomingEvent.Execute`
  calls it first: a redelivery/race loser is logged at debug and returned
  as a successful no-op (not re-broadcast, not an error), so the eventbus
  adapter ACKs it instead of NAK-ing it back into another redelivery loop.
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
