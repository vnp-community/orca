# issue-tracking-service

The Go home for Jira and Linear integration — see
[`specs/backend-go/services/issue-tracking-service.md`](../../../specs/backend-go/services/issue-tracking-service.md)
for the full design. Follows the exact package layout and conventions
established by `usage-service` (`../usage-service/README.md`), the Phase 0
reference implementation.

Per the design doc, this is a **faithful port of already-correct
architecture, not a gap-fixing rebuild** — the TS `jira.*`/`linear.*` RPC
namespaces were already "cleanly backend-local: direct HTTPS REST (Jira) /
GraphQL SDK (Linear) calls," per-user credentials, no CLI shell-out. What
changes here is the implementation language, credential storage (Vault via
`credential-broker-service` instead of `WebCredentialStore` files), and one
side effect that becomes an async event instead of a direct cross-service
write (design doc §7).

The generated proto (`orca.issuetracking.v1`) implements a 3-RPC subset of
the design doc's full sketch — `ListIssues`, `CreateIssue`, `LinkIssue` —
this service implements exactly that subset.

## What's implemented

- `internal/domain/` — `Issue` value object and the `Provider` (Jira/Linear)
  enum, with invariant-enforcing constructors, pure unit tests.
- `internal/usecase/` — `ListIssues`, `CreateIssue`, `LinkIssue`, each tested
  against in-memory fakes (`IssueTrackerProvider`, `ProviderRegistry`,
  `CredentialResolver`, `EventPublisher`) — no real Jira/Linear/NATS needed.
- `internal/adapter/jira/` — **real** Jira Cloud REST API v3 client:
  `ListIssues` does a real `GET /rest/api/3/search` and parses the actual
  response shape; `CreateIssue` does a real `POST /rest/api/3/issue`,
  including a minimal Atlassian Document Format (ADF) wrapper for the
  description field, which Jira Cloud v3 requires instead of plain text.
  Basic Auth, `base64(email:apiToken)`, per design doc §9.
- `internal/adapter/linear/` — **real** hand-rolled GraphQL client (no
  official Linear Go SDK exists, design doc §4): `ListIssues` and
  `CreateIssue` both do real `POST` requests with a GraphQL query/mutation
  string and `Authorization: Bearer <token>`. `CreateIssue` first resolves
  the team key to a team ID via a real `teams` query, since Linear's
  `issueCreate` mutation takes `teamId`, not a team key.
- `internal/adapter/eventbus/` — **real** implementation of `EventPublisher`
  via `common/eventbus`, publishing `orca.issuetracking.link.created`.
  Unlike `usage-service`'s best-effort event publish, `LinkIssue`'s publish
  IS this service's persisted side effect (it owns no database) — a publish
  failure fails the RPC, not a fire-and-forget afterthought.
- `internal/adapter/providerregistry/` — trivial static
  `domain.Provider -> usecase.IssueTrackerProvider` map, populated once in
  `cmd/server/main.go`; not an external-system adapter, just composition-root
  wiring kept out of `main.go` for readability.
- `internal/adapter/grpc/` — implements the generated
  `issuetrackingv1.IssueTrackingServiceServer`, pure wire<->usecase
  translation.
- `cmd/server/main.go` — a real, working composition root: config load,
  NATS connection (degrades gracefully if unavailable — but see the
  `LinkIssue` note above), gRPC server with the shared interceptor chain,
  health/readiness HTTP server, graceful shutdown on SIGTERM. No Postgres
  pool — this service owns no database (design doc §2/§5).

## Known gaps / stubs (tracked, not silently skipped)

- **`internal/adapter/credential` is a STUB, not production-ready.** The
  design doc (§7, §9) requires per-tenant Jira/Linear credentials to come
  from `credential-broker-service`'s `ResolveCredential` RPC, backed by
  Vault KV v2 — never read from environment variables in production. This
  scaffold's `StubResolver` reads `JIRA_BASE_URL`/`JIRA_EMAIL`/
  `JIRA_API_TOKEN`/`LINEAR_API_TOKEN` from the process environment purely so
  `ListIssues`/`CreateIssue` are exercisable in local dev without
  `credential-broker-service` running. **Must be replaced with a real
  `credential-broker-service` gRPC client before this service is deployed
  anywhere real tenant secrets exist.**
- **`ListIssuesRequest`/`CreateIssueRequest`'s `tenant_id` proto field is
  intentionally ignored.** The generated proto carries an explicit
  `tenant_id` field on these requests, but design doc §9 is explicit that
  "tenant_id comes from gRPC metadata (interceptor-enforced), never a
  request body field." The gRPC adapter pulls tenant identity from context
  (`common/tenant`, populated by `common/grpcmw`'s interceptor) instead of
  forwarding `req.GetTenantId()` — consistent with every other service in
  this repo, at the cost of one unused proto field.
- **Jira `CreateIssue` hardcodes issue type `"Task"`.** The RPC surface has
  no issue-type field yet (`ListIssueTypes`/`ListCreateFields` from the
  design doc's full API sketch, §3, aren't implemented — this service only
  implements the 3 RPCs the generated proto defines). Real Jira sites vary
  in which issue-type names exist; wire a real issue-type lookup once that
  RPC exists.
- **No per-provider rate limiting or circuit breaking** (design doc §8) —
  the Jira/Linear adapters are plain HTTP/GraphQL clients with no
  token-bucket limiter or `(provider, tenant_id)`-keyed circuit breaker yet.
- **No retries on idempotent reads** (design doc §8) — `ListIssues` calls
  the provider once; a jittered-backoff retry wrapper is a follow-up.
- **`internal/adapter/postgres/` and `migrations/` are empty.** Design doc
  §5's two thin operational tables (`issuetracking_connections`,
  `issuetracking_webhook_deliveries`) aren't needed by `ListIssues`/
  `CreateIssue`/`LinkIssue` and are left for whenever `Connect`/
  `GetConnectionStatus`/webhook ingestion RPCs are added to the proto.
- **`common/tracing` has no OTLP exporter configured** — same as
  `usage-service`; spans are created but not shipped anywhere until a
  collector endpoint is wired in.
- Only the 3 RPCs the generated proto defines (`ListIssues`, `CreateIssue`,
  `LinkIssue`) are implemented. The design doc's full API sketch (§3) is
  much larger (`Connect`, `SearchIssues`, `GetIssue`, `UpdateIssue`,
  `ListProjects`, etc.) — extend `proto/orca/issuetracking/v1/issuetracking.proto`
  and this service's usecase/adapter layers together as more of the surface
  is needed.

## Running locally

```sh
# from backend-go/
docker compose up -d nats   # see ../../docker-compose.yml

cd services/issue-tracking-service
NATS_URL=nats://localhost:4222 \
JIRA_BASE_URL=https://your-domain.atlassian.net \
JIRA_EMAIL=you@example.com \
JIRA_API_TOKEN=... \
LINEAR_API_TOKEN=... \
  go run ./cmd/server
```

## Testing

```sh
go test ./...   # unit tests (domain/, usecase/) — no external deps, no network calls
```
