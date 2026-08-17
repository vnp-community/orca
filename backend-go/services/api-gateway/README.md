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

## Auth: real JWKS-verified JWTs + real session cookies (Epic D)

Both halves of auth are now real, not placeholders:

1. **Bearer JWTs (mobile/CLI).** `internal/usecase/validate_identity.go`'s
   `AuthValidator` verifies a short-lived RS256 JWT's signature — and its
   `exp`/`iat`/`iss` — against `auth-service`'s published JWKS before
   trusting any claim, via `common/jwtauth` and the real
   `internal/adapter/authclient.JWKSClient` (implements the
   `internal/usecase/ports.go` `JWKSClient` port: `GetJWKS` -> parse ->
   cache ~5min -> resolve by `kid`). A stale-but-cached JWKS is served if a
   refetch fails, rather than failing every in-flight verification on a
   transient `auth-service` blip; a `kid`/signature/claims mismatch fails
   closed (`ErrKeyLookupFailed`/`ErrSignatureVerificationFailed`/
   `ErrMissingIdentityClaims`).
2. **Browser session cookies.** The `orca_session` cookie is a raw opaque
   session token, never a JWT — it's resolved against `auth-service`'s real
   `ValidateSession` RPC via `internal/adapter/authclient.SessionValidator`,
   not parsed as a JWT. `httpgateway.authMiddleware` and
   `wsbridge.Handler.ServeHTTP` (see below) both try this cookie path
   FIRST, falling back to `AuthValidator`'s bearer-JWT path only when no
   cookie validates — a cookie-authenticated browser session has no bearer
   JWT to present, so this ordering matters, not just the fallback itself.

**Fixed alongside this pass:** `wsbridge.Handler.ServeHTTP` previously
called `AuthValidator.Validate` directly with no cookie-first attempt
(unlike `authMiddleware`), which would have made `/v1/notifications/stream`
unreachable for real cookie-authenticated browser sessions once
`AuthValidator` stopped accepting unverified claims. It now takes a
`Cookie CookieValidator` dependency and tries it first, same as
`authMiddleware` (`cmd/server/main.go` wires the same `sessionValidator`
into both).

**Still not production-safe:**
- No OPA authorization check ahead of routing (§9) — see "Other known gaps".
- Outbound gRPC dials (including to `auth-service` for `GetJWKS`/
  `ValidateSession`) use insecure transport credentials (local-dev only).
- No CORS/origin allow-list on the WS upgrade (`InsecureSkipVerify: true`).

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

Covered: the rate limiter's per-tenant token-bucket behavior, real
JWKS-verified JWT validation (valid/tampered-signature/expired/
unknown-kid/missing-claims cases, plus `JWKSClient`'s caching and
stale-cache-survives-fetch-error behavior), `wsbridge.Handler`'s
cookie-then-bearer-JWT auth ordering, the WS<->gRPC frame pump loop, and
the router's 501-stub response shape and 401-unauthenticated behavior.
