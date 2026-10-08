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
| `POST/GET/DELETE /mcp` | **Real, off by default** (`MCP_ENABLED=false` -> not mounted, 404) | MCP Streamable HTTP built on the official Go SDK (`internal/adapter/mcpserver`): initialize/initialized lifecycle, `ping`, `logging/setLevel`, `tools/list` with HMAC-signed cursors, Origin check, body limit, cookie auth rejected. **Authentication is fail-closed**: the default `TokenVerifier` denies everything, so every call is `401` with `WWW-Authenticate: Bearer resource_metadata=...` until BE-MCP-SOL-005/006 plug in real token validation. `tools/list` is empty and `tools/call` answers unknown-tool until BE-MCP-SOL-007 supplies the `ToolCatalog`/`ToolExecutor`. Sessions are in-memory per replica (BE-MCP-SOL-004 makes them durable). No SSE resume (`Last-Event-ID`) yet |
| `GET /.well-known/oauth-protected-resource[/mcp]` | **Real** (when MCP enabled) | RFC 9728 document built from config only (`MCP_PUBLIC_BASE_URL`, `MCP_ISSUER_URL`); no token logic |
| `/.well-known/oauth-authorization-server`, `/oauth/*` | **Not mounted** | Owned by BE-MCP-SOL-005/006 (auth-service); nginx already proxies them |
| WS channel `mcp.server.info` | **Real** | Process flag `MCP_ENABLED` + `mcp-service.GetServerInfo`; always answers (`{enabled:false}` when off). Other `mcp.*` channels are **not registered** yet (fall through to "not implemented"); the `mcpHandler`/`mcpChannelError` wrappers (`MCP_DISABLED`, `MCP_NOT_ADMIN`, `CODE: msg` errors) are ready for them |
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

## MCP tool catalog (BE-MCP-SOL-007/008)

`internal/adapter/mcpserver/tools` turns `wscompat.Registry` channels into MCP
tools: hand-declared `ToolSpec`s (input schema and `Args()` come from one
field table), a catalog filtered by token scope and tenant policy, and an
executor (validate, scope, `PolicyGate.Decide`, approval, dispatch,
camelCase/redact/truncate, `PolicyGate.Complete`).

- Schema library: `github.com/google/jsonschema-go` v0.4.3 (the library go-sdk uses).
- Packs: `MCP_TOOL_PACKS_ENABLED` (default `1`; `1,2` adds reversible writes;
  3 exec and 4 destructive/admin stay hidden unless listed). `MCP_TOOL_TIMEOUT` (default 55s).
- **Fail-closed default:** until a governance `PolicyGate` is wired
  (`buildMCPToolStack(reg, gate, ...)` in `cmd/server`), `mcpserver.FailClosedGate`
  allows only tools with `risk=read` and denies everything else; approvals never resolve.
- Every registered channel must have a ToolSpec or an entry in
  `tools/excluded_channels.yaml` (with a reason): `go test ./internal/adapter/mcpserver/tools -run TestChannelInventory -v`
  prints `MCP_CHANNEL_INVENTORY ...` and fails on uncovered channels.
- `mcp.admin.tool.list` (admin) returns `McpToolView[]` including hard-denied channels.

## MCP rollout, observability and conformance (BE-MCP-SOL-015)

The service half of the runbook (flags, order, rollback, checklist, metrics
list, known gaps) is in [`mcp-service/README.md`](../mcp-service/README.md#rollout-runbook-be-mcp-sol-015).
Edge-specific points:

- **Flag behaviour** (tests): `MCP_ENABLED=false` => `/mcp`, `/mcp/` and
  `/.well-known/oauth-*` answer 404 (`cmd/server` `TestMCPDisabledRoutesAre404`)
  and every registered `mcp.*` channel but `mcp.server.info` answers
  `MCP_DISABLED` without calling mcp-service
  (`wscompat` `TestEveryMcpChannelIsDisabledWhenProcessFlagIsOff`).
  `MCP_TENANT_DEFAULT_ENABLED` is read by mcp-service only; the gateway holds
  no policy, so "default-on never changes risk defaults" is proven in
  mcp-service (`TestTenantDefaultEnabledTrue_DoesNotChangeRiskDefaults`), not here.
- **Env of this process**: `MCP_ENABLED`, `MCP_PUBLIC_BASE_URL` (or `PUBLIC_BASE_URL`),
  `MCP_ISSUER_URL`, `MCP_SERVICE_ADDR`, `MCP_INTERNAL_CALLER_TOKEN`,
  `WS_ALLOWED_ORIGINS` / `MCP_ALLOWED_ORIGINS`, `MCP_CURSOR_KEY[_PREVIOUS|_FILE]`,
  `MCP_MAX_REQUEST_BYTES`, `MCP_SESSION_IDLE_TTL`, `MCP_MAX_SSE_STREAMS_PER_USER|_TENANT`,
  `MCP_SSE_BUFFER_MAX_BYTES`, `MCP_PAT_MAX_PER_USER`, `OAUTH_DCR_ENABLED`,
  `MCP_TOOL_PACKS_ENABLED`, `MCP_TOOL_TIMEOUT`, `NATS_URL`. auth-service needs
  `OAUTH_RESOURCE_URL` (= `<MCP_PUBLIC_BASE_URL>/mcp`) and `OAUTH_INTERNAL_CALLER_TOKEN`.
- **Metrics**: `GET :HTTP_PORT/metrics` (the internal health port, never the
  public edge) = health mux + `promhttp` over a private registry
  (`internal/adapter/mcpmetrics`, wired in `cmd/server/mcp_metrics_wiring.go`).
  It is served even with `MCP_ENABLED=false` (Go/process collectors only then).
- **Tracing**: `mcp.request` -> `mcp.policy` -> `mcp.dispatch` spans under the
  `otelhttp` root; the mcp-service connection carries the trace to mcp-service,
  whose audit events include `trace_id`.
- **Conformance tiers** (see `backend-go/ci/mcp-conformance/README.md`): tier 1 Go
  in-process (`ci/mcp-conformance/run-go-conformance.sh`, blocking); tier 1b
  Python reference client with the official `mcp` SDK (non-blocking until it has
  run green against a real dev stack); tier 2 Inspector (not implemented).
  `cmd/mcpconformance-devserver` is a test-only binary that serves the real
  `/mcp` handler with a static token so the Python tier can run without the stack.

## Code intelligence channels (CR-CV-040)

Bridges desktop and web client WebSocket requests to downstream `code-intel-service` over gRPC.

- **Downstream service:** `code-intel-service` (gRPC port 9090).
- **Environment variables:**
  - `CODE_INTEL_SERVICE_ADDR`: Target gRPC address for `code-intel-service` (e.g. `localhost:9090` or `code-intel-service:9090`).
  - `CODE_INTEL_MAX_RESPONSE_BYTES`: Maximum response size in bytes (default `8 MiB` / `8388608`).
  - `CODE_INTEL_MAX_STREAMS`: Maximum concurrent active streaming subscriptions per gateway instance (default `128`).
  - `CODEINTEL_INTERNAL_CALLER_TOKEN`: Shared bearer token for gateway-to-service gRPC metadata authentication (`x-internal-caller-token`).
- **Channels:** 46 total channels registered under the `codeIntel.*` namespace:
  - 45 unary request-response channels (invoked via `invoke`, never `send` per invariant U2).
  - 1 server-push stream channel (`codeIntel.subscribe`).
- **Invariants & limits:**
  - `SetReadLimit(320 KiB)` (`327680` bytes): WebSocket frame read limit is capped at 320 KiB on `/ws` to accommodate payloads such as `reviewState.save` (up to 256 KiB plus envelope overhead). Larger frames cause WebSocket closure with status 1009 (`StatusMessageTooBig`).
  - **Error handling:** Channel error messages maintain canonical code intelligence prefixes (`CODE: message`), serializing errors into JSON error envelopes with `code` and `message`.
  - **Privacy & security:** Request arguments (which may include confidential file paths or source code snippets) are never logged in plaintext.



## Request flow channels, routes and MCP tools (CR-REQ-016, CR-REQ-017)

Bridges the UI, scripts and MCP clients to `request-service` (Request, Solution, Approval, Backlog).
Contract: `specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md`.

- **Downstream service:** `request-service` (gRPC). **Environment variables:**
  - `REQUEST_SERVICE_ADDR`: gRPC address. Empty means the client is not dialed; every Request channel answers `REQUEST_UNAVAILABLE` and the HTTP routes are not mounted.
  - `REQUEST_INTERNAL_CALLER_TOKEN`: shared secret sent as `x-orca-internal-token` on unary and stream calls. With the address set and no token, request-service rejects guarded calls (a warning is logged at start).
  - `MCP_REQUEST_CREATE_PER_HOUR`: creation budget per (tenant, user, MCP client) shared by `request_create` and `request_spawnChild` (default `20`, `0` disables; per gateway replica).
  - Writing MCP tools need `MCP_TOOL_PACKS_ENABLED=1,2`.
- **WS channels (28 + 4):** `request.*` (16, including the `request.subscribe` stream that exists only when NATS is connected), `solution.*` (3), `approval.*` (6), `backlog.*` (3); extra reads `request.links`, `request.flow`, `request.checks` and `request.planProposal`. Identity comes from the session only. Errors are `REQUEST_*: message` (`REQUEST_APPROVAL_*` for approvals); gRPC `Unimplemented` becomes `REQUEST_NOT_IMPLEMENTED`. `request.classify` and `request.generatePlan` use a 24s deadline (below the 25s WS cap).
- **HTTP routes (5):** `POST|GET /v1/requests`, `GET /v1/requests/{id}`, `GET /v1/approvals/pending`, `POST /v1/approvals/{id}/approve|reject`. They dispatch through the same channel handlers. Approve and reject need `{version, digest, comment}`; a body with only `{version, comment}` is refused with `INVALID_ARGUMENT`.
- **No OPA step here:** `request-service` checks permissions and the `request_flow_enabled` flag itself.
- **Source rule:** a client may claim `jira`, `github`, `gitlab` or `linear`; no source is `manual`; MCP calls are always `mcp` with the MCP client name as site (set from the verified session, not from input).
- **MCP:** 17 listed tools (11 read in pack 1, 6 write in pack 2) and 3 declared (`request_classify`, `request_generatePlan`, `request_startPhase`). Human gates (`request.confirmType`, `solution.choose`, `approval.approve|reject|cancel`, `request.cancel`, `request.flowSet`) are permanently excluded in `tools/excluded_channels.yaml`.
