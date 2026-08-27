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
  `CredentialResolver`, `OutboxEnqueuer`) — no real Jira/Linear/Postgres/NATS
  needed.
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
- `internal/adapter/postgres/` — **new in Epic G** (`docs/execution-plan.md`,
  2026-08-17): this service's first database, added purely to host a
  transactional-outbox table. `LinkIssue` now enqueues
  `orca.issuetracking.link.created` as a durable `issuetracking.outbox_events`
  row (`Repository.Enqueue`) instead of publishing to NATS directly — the
  row IS this service's persisted side effect now, not the publish call.
  `common/outbox.Relay` (started in `cmd/server/main.go`) polls this table
  and publishes to NATS asynchronously. This is a single-write "outbox" —
  there's no separate domain-state write to be transactional with, since
  this service still stores no queryable copy of issue data itself (Jira/
  Linear remain the systems of record, design doc §2/§5) — see this
  package's doc comment for the full reasoning. The old
  `internal/adapter/eventbus` package (direct-NATS-publish `EventPublisher`)
  is gone, replaced by this.
- `internal/adapter/providerregistry/` — trivial static
  `domain.Provider -> usecase.IssueTrackerProvider` map, populated once in
  `cmd/server/main.go`; not an external-system adapter, just composition-root
  wiring kept out of `main.go` for readability.
- `internal/adapter/grpc/` — implements the generated
  `issuetrackingv1.IssueTrackingServiceServer`, pure wire<->usecase
  translation.
- `cmd/server/main.go` — a real, working composition root: config load,
  Postgres pool (new in Epic G — see `internal/adapter/postgres` above;
  `DATABASE_DSN` is now required to boot, unlike before), NATS connection
  for the outbox relay (degrades gracefully if unavailable at startup —
  `LinkIssue` itself no longer depends on NATS being reachable, only on the
  database), gRPC server with the shared interceptor chain,
  health/readiness HTTP server, graceful shutdown on SIGTERM (including the
  outbox relay goroutine).
- `migrations/0001_outbox.{up,down}.sql` — real DDL: this service's only
  table, `issuetracking.outbox_events`, RLS policy matching every other
  service's tenant-isolation convention.

## `credential-broker-service` is wired (Epic B, 2026-08-17)

`internal/adapter/credential.Resolver` dials `credential-broker-service` for
real (`cfg.CredentialBrokerAddr`) and calls
`credentialbrokerv1.ResolveCredentialByOwner` — the same by-owner lookup
pattern `scm-integration-service` uses (see that service's README for the
full rationale), since this service is only ever handed `(tenantID,
provider)`, never an opaque `credential_id`. `owner_id` is the provider
name (`"jira"`/`"linear"`). Because `usecase.Credential` here has three
fields (`BaseURL`/`Email`/`Token` — Jira needs all three, Linear only
`Token`) where `ResolveCredentialByOwnerResponse.value` is a single
plaintext byte slice, this adapter documents and implements a JSON-envelope
convention: `{"baseUrl":...,"email":...,"token":...}`, decoded on resolve.

**Known, not-yet-exercised gap:** exactly like scm-integration-service, no
"connect this Jira/Linear account" write flow exists anywhere in this
scaffold, so `Resolve` (and the JSON-envelope convention above) has never
been run against a real written-then-resolved credential end to end — only
unit-tested against `credential-broker-service`'s own fakes.

## Known gaps / stubs (tracked, not silently skipped)
- **This service gained a database purely to host an outbox table (Epic G,
  `docs/execution-plan.md`, 2026-08-17) — a deliberate, explicitly-requested
  exception to design doc §2/§5's "no database" statement, not a reversal
  of it.** `issuetracking.outbox_events` is the only table; issue data
  itself is still never persisted here (Jira/Linear remain the systems of
  record). This was built on explicit user direction after flagging the
  trade-off (a real architecture decision, giving a stateless-by-design
  service a database) — the alternative (documented but not chosen) was
  leaving `LinkIssue`'s previous direct-publish-or-fail-the-RPC behavior
  as-is, which was already correct given no DB existed to be transactional
  with. Also note Epic G's own stated precondition ("a second real
  consumer exists beyond notification-service") is still not met for
  `orca.issuetracking.link.created` as of this pass — no service actually
  subscribes to it yet.
- **`ListIssuesRequest`/`CreateIssueRequest`'s `tenant_id` proto field is
  intentionally ignored.** The generated proto carries an explicit
  `tenant_id` field on these requests, but design doc §9 is explicit that
  "tenant_id comes from gRPC metadata (interceptor-enforced), never a
  request body field." The gRPC adapter pulls tenant identity from context
  (`common/tenant`, populated by `common/grpcmw`'s interceptor) instead of
  forwarding `req.GetTenantId()` — consistent with every other service in
  this repo, at the cost of one unused proto field.
- ~~Jira `CreateIssue` hardcodes issue type `"Task"`.~~ — **closed**
  (`docs/execution-plan.md` §3 Phase 1): `internal/adapter/jira/client.go`'s
  `CreateIssue` now calls a real, internal `listIssueTypes` lookup (a
  genuine `GET /rest/api/3/issue/createmeta/{projectKey}/issuetypes`
  request — the current, non-deprecated Jira Cloud endpoint for this)
  before every create, and `resolveIssueType` picks the actual issue type
  to send: an exact case-insensitive match on `"Task"` among the real
  returned types, else the first non-subtask type, else a clear error if
  the project has none. This stays an unexported adapter-internal
  capability, not a new RPC — `ListIssueTypes`/`ListCreateFields` from the
  design doc's full API sketch (§3) still aren't exposed over gRPC
  (`CreateIssueRequest` has no issue-type field to carry a caller-requested
  name), so the only remaining gap is that callers still can't request a
  specific issue type themselves; that still needs the real RPC.
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
