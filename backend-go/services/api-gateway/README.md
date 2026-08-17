# api-gateway

See [`specs/backend-go/services/api-gateway.md`](../../../specs/backend-go/services/api-gateway.md)
for the full design. **This service deviates from the package layout every
other Go service follows** ([`usage-service`](../usage-service/README.md) is
the reference for the standard layout) — read the design doc's §6
"Package layout notes" for why, summarized here:

`api-gateway` owns no database and no business domain. It is the system's
one external listener and the one gRPC *client* to all 16 other services,
never a gRPC server itself. So `internal/domain/` and `internal/usecase/`
are intentionally thin (routing config and cross-cutting request handling,
not business entities/use cases), and `internal/adapter/` is the
overwhelming majority of the code — inbound HTTP/WS on one side, outbound
gRPC clients on the other. `internal/adapter/postgres/` and `migrations/`
exist as empty scaffold directories only, per the task that stood this
service up — never populated, matching §5's "no PostgreSQL database, no
`migrations/`, no `adapter/postgres/`".

## What's really wired vs. what's a stub

Per the design doc, only two downstream services are far enough along to
integrate against for real; every other route is a documented `501`.

| Path | Status | Notes |
|---|---|---|
| `GET /v1/usage/daily` | **Real** | Calls `usagev1.UsageServiceClient.GetDailyUsage` on usage-service |
| `POST /v1/usage/sessions` | **Real** | Calls `usagev1.UsageServiceClient.RecordUsageSession` |
| `GET /v1/usage/sessions` | **Real** | Calls `usagev1.UsageServiceClient.ListSessions` |
| `GET /v1/notifications/stream` (WS) | **Real** | Bridges a WS connection to notification-service's `StreamNotifications` gRPC server-stream — see `internal/adapter/wsbridge/handler.go` |
| Everything else (`/v1/tasks/*`, `/v1/projects/*`, `/v1/auth/*`, `/v1/tenants/*`, `/v1/infra/*`, `/v1/git/*`, `/v1/scm/*`, `/v1/issues/*`, `/v1/ai-providers/*`, `/v1/workflows/*`, `/v1/orchestration/*`, `/v1/automations/*`, `/v1/annotations/*`, `/v1/notifications/*` REST) | **501 stub** | Returns `{"error":{"code":"NOT_IMPLEMENTED","message":"this route will proxy to <service> once its gRPC contract stabilizes"}}` — see `internal/domain.NewDefaultServiceRegistry` for the full routing table and `internal/adapter/httpgateway/stub_routes.go` |
| Per-tenant rate limiting | **Real** | `internal/usecase/rate_limit.go` — an actual in-memory `golang.org/x/time/rate` token-bucket limiter, one bucket per tenant, applied ahead of every route |

`credential-broker-service` has no route at all (real or stubbed): per the
design doc §7, it's reached only indirectly via `infra-fleet-service`'s
credential path, never called through this gateway directly.

Production wiring for the real routes should eventually go through a
`grpc-gateway`-generated mux built from each service's `.proto`
`google.api.http` annotations (design doc §3), replacing the hand-written
JSON<->protobuf translation in `internal/adapter/httpgateway/usage_routes.go`
— that codegen step isn't wired in this scaffold, so the usage routes are
hand-written as the reference pattern instead.

## Auth: placeholder, NOT production safe

`internal/usecase/validate_identity.go`'s `AuthValidator` extracts
`tenant_id`/`sub` claims from a JWT (bearer token or session cookie value)
**without verifying its signature**. Any caller can forge these claims and
be trusted today. Before this service is production-ready it must:

1. Fetch `auth-service`'s JWKS and verify RS256 JWT signatures against it
   (the `JWKSClient` port in `internal/usecase/ports.go` is defined but has
   no real `internal/adapter/authclient` implementation yet).
2. Resolve the browser session cookie against `auth-service`'s session
   store, rather than parsing it as a JWT the way this placeholder does for
   simplicity.

This is the same gap the design doc's §9 already frames as needing a real
JWKS-verification path — it's called out here, in code comments on
`AuthValidator`, and in the router's auth middleware, not silently skipped.

## Other known gaps

- **No gRPC server of its own.** Per §1/§7, `api-gateway` is a pure client
  to every other service and is never called by one — `cmd/server/main.go`
  documents this choice rather than adding an unused gRPC listener.
- **Outbound gRPC dials use insecure transport credentials** (local-dev
  only). Production needs mTLS client credentials matching every internal
  service's mTLS expectation (§9) — `internal/adapter/grpc/dial.go`.
- **No OPA authorization check.** §9 describes a coarse-grained "can this
  JWT call this endpoint" OPA check ahead of routing — not implemented.
- **No request-size limits or WAF-style input sanitization** (§9) — not
  implemented in this pass.
- **No CORS/origin allow-list on the WS upgrade** —
  `internal/adapter/wsbridge/handler.go` accepts any origin
  (`InsecureSkipVerify: true`); needs the real frontend/mobile origins
  wired before production.
- **In-memory rate limiter is per-replica**, not shared across replicas —
  an acceptable choice per §5 ("per-replica or backed by a shared fast
  store... either way disposable counters"), but not cross-replica
  consistent. `usecase.RateLimitStore` (ports.go) is the seam for a
  Redis-backed implementation if that's ever needed.
- **The WS bridge is one-directional** (gRPC-to-WS only), matching
  notification-service's `StreamNotifications` (server-push only). A
  bidirectional bridge with the bounded-buffer backpressure policy §8
  describes (e.g. for a future terminal relay over `infra-fleet-service`)
  is not implemented here.

## Running locally

```sh
# from backend-go/
cd services/api-gateway
USAGE_SERVICE_ADDR=localhost:9101 \
NOTIFICATION_SERVICE_ADDR=localhost:9102 \
PUBLIC_PORT=8081 \
HTTP_PORT=8080 \
  go run ./cmd/server
```

`PUBLIC_PORT` serves the REST/WS edge (`Base.HTTPPort`'s usual role in
other services is repurposed here for `/healthz`/`/readyz` only, since this
service has no gRPC server to put on `Base.GRPCPort`).

## Testing

```sh
go test ./...   # unit tests only — no external deps; usage-service and
                 # notification-service calls are exercised via interfaces/
                 # fakes (internal/usecase), not real gRPC connections
```

Covered: the rate limiter's per-tenant token-bucket behavior, the
placeholder JWT claim extraction (valid/malformed/missing-claims cases),
the WS<->gRPC frame pump loop, and the router's 501-stub response shape and
401-unauthenticated behavior.
