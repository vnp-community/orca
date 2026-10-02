# mcp-service

Holds the durable state and decisions of the MCP layer (tenant settings, and
later sessions, OAuth clients, grants, tool policies, approvals, external
servers) behind a gRPC API that `api-gateway` calls. It is **not** the MCP wire
protocol: that lives in the gateway (BE-MCP-SOL-002/003).

Spec: [`BE-MCP-SOL-001`](../../../specs/backend-go/crs/v5/mcp-service-foundation/solutions/BE-MCP-SOL-001-scaffold-mcp-service.md) ·
CR: [`CR-MCP-001`](../../../docs/crs/v5/mcp-service-foundation/CR-MCP-001-scaffold-mcp-service.md).
Layout follows [`usage-service`](../usage-service/README.md).

## Real vs stub

| Item | Status |
|---|---|
| gRPC `McpService.GetServerInfo` (tenant settings + scope catalog) | **Real**. Lazily inserts the tenant's `mcp.tenant_settings` row on first call. |
| Tenant settings get/update (`GetOrCreateTenantSettings`, `UpdateTenantSettings` in the Postgres repo; `UpdateTenantSettings` usecase) | **Real**, tested. The update usecase has **no RPC** yet (admin channel arrives with BE-MCP-SOL-003/013). |
| Postgres RLS (`ENABLE` + `FORCE`, per-transaction `set_config('app.tenant_id')`) | **Real**, proven by integration tests with a non-superuser role. |
| Outbox: `EnqueueOutbox` + `common/outbox` relay to `orca.mcp.>` (stream `MCP`) | **Real**; no usecase emits an event yet (first producer: BE-MCP-SOL-004). |
| `processed_events` table (consumer dedupe) | Table only; no consumer yet. |
| Kill switch | Column and read path only; nothing sets it yet (BE-MCP-SOL-013). The settings update never writes it. |
| `Clock` port | Not created (DB sets `updated_at`); add when a usecase needs time. |
| OAuth consent and grants (BE-MCP-SOL-005): `CreateConsentRequest`, `GetConsentRequest`, `DecideConsent`, `ListGrants`, `RevokeGrant`, `ListOAuthClients`, `SetOAuthClientStatus`; tables `consent_requests`, `grants` (migration `0002`); outbox events `orca.mcp.grant.{created,updated,revoked}`, `orca.mcp.client.status_changed`; revocation reconcile job | **Real**, tested (unit + integration). Off unless `MCP_PUBLIC_BASE_URL` (or `PUBLIC_BASE_URL`) is set. Calls auth-service `OAuth*` RPCs; the HTTP/WS surface is api-gateway's. |
| Sessions (SOL-004), tool policies / approvals / kill switch / audit (SOL-012/013), custom prompts, external MCP server registry (SOL-014) | **Implemented** with unit and (build tag `integration`) Postgres tests; tables in migrations `0003`-`0007`. Not verified end-to-end against a full running dev stack by this README; see the rollout runbook below for the known gaps. |
| `/metrics` (BE-MCP-SOL-015) | **Real**, minimal: `orca_mcp_sessions_active` (DB sample every 30s) and `orca_mcp_sessions_count_errors_total` plus Go/process collectors, on `HTTP_PORT`. Request/tool/auth series are recorded by the gateway, not here. |
| MySQL | **Not supported** (D2): `main` exits if `DATABASE_DSN` is not `postgres://`/`postgresql://`. |

## Config

| Env | Default | Meaning |
|---|---|---|
| `GRPC_PORT` / `HTTP_PORT` | 9090 / 8080 | gRPC and `/healthz` `/readyz` |
| `DATABASE_DSN` / `DATABASE_CREDENTIALS_FILE` | - / `/vault/secrets/database-credentials` | Postgres only |
| `NATS_URL` | `nats://localhost:4222` | Outbox relay; startup continues without NATS |
| `MCP_TENANT_DEFAULT_ENABLED` | `true` | `enabled` of a newly created tenant row (D6). Existing rows are never changed by it. It does not alter risk defaults, hard-denies, scopes or the kill switch. The process-wide switch is the gateway's `MCP_ENABLED`. |
| `MCP_DEFAULT_MAX_TOKEN_DAYS` | `90` | Token lifetime ceiling for new tenant rows (1..90) |

### OAuth consent config

| Env | Default | Meaning |
|---|---|---|
| `MCP_PUBLIC_BASE_URL` (falls back to `PUBLIC_BASE_URL`) | - (feature off) | public origin; echoed as `iss` (RFC 9207), never taken from a request |
| `AUTH_SERVICE_ADDR` | `auth-service:9090` | |
| `OAUTH_INTERNAL_CALLER_TOKEN` | - (required when on) | must equal auth-service's value |
| `MCP_CONSENT_TTL` | `10m` | lifetime of a consent request (1m..1h) |
| `MCP_GRANT_RECONCILE_INTERVAL` | `15s` | retry of revocations auth-service has not acknowledged |

## Governance and safety (BE-MCP-SOL-012 / 013)

mcp-service makes every tool-call decision: tenant policies (versioned,
optimistic concurrency), the static Rego gate `orca.authz.mcp`
(`policy/orca-authz/mcp.rego`, hard-deny list not loosenable by any tenant
policy), single-use approvals bound to a canonical `paramsHash`, kill switch
(tenant / client / grant / session), a Postgres sliding-window limiter, taint
tracking after untrusted reads, and one audit event per call
(`orca.mcp.audit.appended`, ingested by auth-service as `actor_type=agent`).
Any engine, database or bus failure is a deny.

| Env | Default | Meaning |
|---|---|---|
| `OPA_BUNDLE_PATH` | `../../policy/orca-authz` (image: `/policy/orca-authz`) | Rego bundle; a broken bundle stops startup |
| `MCP_MAX_DEPTH` | `1` | recursion depth at which process-spawning tools are denied |
| `MCP_POLICY_CACHE_TTL` | `5s` | cache of policy/settings/client inputs (decisions are never cached) |
| `MCP_KILLSWITCH_POLL` | `5s` (max 30s) | kill-state cache TTL; bounds propagation when NATS is down |
| `MCP_TAINT_TTL` | `30m` | how long a successful untrusted read taints (user, client) |
| `MCP_TOOL_CALL_MAX_SECONDS` | `900` | calls stuck in `started` are finalized as interrupted |
| `MCP_TOOL_CALLS_RETENTION` | `720h` | journal retention |
| `MCP_RATE_EXEC_PER_MIN` / `_WRITE_` / `_READ_` | `30` / `120` / `300` | per (tenant,user,client) |
| `MCP_RATE_TOTAL_PER_MIN` / `MCP_RATE_DAILY_PER_USER` | `300` / `5000` | |
| `MCP_MAX_PENDING_APPROVALS` / `MCP_MAX_APPROVALS_PER_HOUR` | `10` / `30` | per (user, client) anti-spam |
| `MCP_INTERNAL_CALLER_TOKEN` | empty (WARN) | shared secret required on gateway-only RPCs (`AuthorizeToolCall`, `CompleteToolCall`, `WaitApproval`, `DecideApproval`, `EvaluateToolCall`, `FilterTools`, `GetKillState`); the gateway sends the same variable |

`MCP_TENANT_DEFAULT_ENABLED` only sets `enabled` of a newly created settings
row; it can never change risk defaults, the hard-deny list, scopes or the kill
switch (tested).

## RLS requirements

RLS only protects when the connecting role is neither a superuser nor
`BYPASSRLS`. The dev compose stack connects as `orca` (a superuser), so RLS is
**not** effective there; production must use a plain role, e.g.:

```sql
CREATE ROLE mcp_app LOGIN NOSUPERUSER NOBYPASSRLS;
GRANT USAGE ON SCHEMA mcp TO mcp_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA mcp TO mcp_app;
```

Every repository method runs inside `withTenantTx` (or `withRelayTx` for the
outbox relay); a unit test enforces this.

## Database

The `mcp` database is created by `deploy/postgres-init-databases.sh` and
`deploy/dev/docker/postgres/init-databases.sh`, which only run on a fresh
Postgres volume. On an existing environment run `CREATE DATABASE mcp OWNER orca;`
by hand, then `deploy/dev/scripts/migrate.sh mcp`.

## Testing

```sh
go test ./...                                              # unit, no Docker
go test -tags=integration ./internal/adapter/postgres/...  # testcontainers Postgres (incl. governance tables, RLS, limiter)
go test ./internal/redteam/... -count=1                    # prompt-injection scenarios (RT-01..17)
```

## Rollout runbook (BE-MCP-SOL-015)

Spec: [`BE-MCP-SOL-015`](../../../specs/backend-go/crs/v5/mcp-quality-rollout/solutions/BE-MCP-SOL-015-conformance-e2e-observability-rollout.md).
The edge half of this runbook is in [`api-gateway/README.md`](../api-gateway/README.md#mcp-rollout-observability-and-conformance-be-mcp-sol-015).

### Flags

| Flag | Where | Default | Effect |
|---|---|---|---|
| `MCP_ENABLED` | api-gateway process | `false` | master switch: `false` => `/mcp`, `/.well-known/oauth-*` answer 404 and every `mcp.*` channel except `mcp.server.info` answers `MCP_DISABLED`; `mcp.server.info` returns `enabled:false` without an error |
| `MCP_TENANT_DEFAULT_ENABLED` | mcp-service | `true` (D6) | initial `enabled` of a tenant row created from now on. It never changes risk defaults (exec/destructive => require approval, admin => deny), hard-deny, scopes or the kill switch (`TestTenantDefaultEnabledTrue_DoesNotChangeRiskDefaults`) |
| tenant `enabled`, kill switch | admin channels `mcp.admin.settings.set`, `mcp.admin.killswitch.set` | per tenant | immediate per-tenant off / stop; clearing the kill switch restores the earlier decisions (`TestKillSwitchRollbackRestoresDecisions`) |
| `MCP_TOOL_PACKS_ENABLED` | api-gateway | `1` | pack 1 read-only; `1,2` adds reversible writes; 3 (exec) and 4 (destructive/admin) stay hidden unless listed |

### Order of enabling

1. Create the database (existing environments, see below) and run `deploy/dev/scripts/migrate.sh mcp`; deploy `mcp-service` (it is inert until the gateway calls it).
2. auth-service: set `OAUTH_RESOURCE_URL=<public base>/mcp`, `OAUTH_INTERNAL_CALLER_TOKEN`; mcp-service: same `OAUTH_INTERNAL_CALLER_TOKEN`, `MCP_PUBLIC_BASE_URL`, `AUTH_SERVICE_ADDR`; keep `MCP_TENANT_DEFAULT_ENABLED=true` for dev/dogfood.
3. Set `MCP_INTERNAL_CALLER_TOKEN` on mcp-service AND api-gateway (same value) before exposing the gateway.
4. api-gateway: `MCP_SERVICE_ADDR`, `MCP_PUBLIC_BASE_URL`, `WS_ALLOWED_ORIGINS` (or `MCP_ALLOWED_ORIGINS`), `NATS_URL`, then `MCP_ENABLED=true` last.
5. Scrape `/metrics` of both services and load the alert rules before onboarding anyone beyond the dev team.

### Rollback

Fastest first: tenant kill switch (`mcp.admin.killswitch.set`, effect within `MCP_KILLSWITCH_POLL`, max 30s with NATS down, immediate via the event bus) -> `MCP_ENABLED=false` on the gateway (restart; routes 404, channels `MCP_DISABLED`) -> stop mcp-service. Migrations only add tables, so nothing has to be reverted; do not run the `down` migrations in an incident.

### Operator checklist

- [ ] `OAUTH_RESOURCE_URL` (auth-service) equals `<MCP_PUBLIC_BASE_URL>/mcp` exactly (https; `http` only for loopback). A mismatch makes every token fail the `aud` check.
- [ ] `MCP_PUBLIC_BASE_URL` set on api-gateway and mcp-service (same origin; used for metadata and the RFC 9207 `iss`; never derived from the request Host).
- [ ] `OAUTH_INTERNAL_CALLER_TOKEN` identical on auth-service and mcp-service; `MCP_INTERNAL_CALLER_TOKEN` identical on mcp-service and api-gateway. An empty value only logs a WARN and leaves those RPCs protected by the network alone.
- [ ] `WS_ALLOWED_ORIGINS` set (otherwise `/ws` keeps the legacy no-Origin-check behaviour and `/mcp` rejects every request that carries an Origin).
- [ ] NetworkPolicy: only api-gateway (and infra-fleet for `ResolveAgentMcpConfig`) may reach mcp-service gRPC; only api-gateway and mcp-service may reach auth-service OAuth/MCP RPCs. Be aware that `ResolveMcpPrincipal` (auth-service) and `StreamEvents` (mcp-service) are NOT behind the shared-secret guard, and a NetworkPolicy cannot restrict a single RPC, so any pod allowed to reach those services can call them. Treat that as a known gap until they get the guard.
- [ ] Vault Transit key `credential-broker-mcp_external_secret` exists (`vault write -f transit/keys/credential-broker-mcp_external_secret`) and the broker's policy may use it; broker migration for category `mcp_external_secret` applied. Without it `mcp.externalServer.setSecret` fails.
- [ ] Database `mcp` exists: `deploy/postgres-init-databases.sh` and the dev docker init script only run on a fresh volume. On an existing environment: `CREATE DATABASE mcp OWNER orca;` then `deploy/dev/scripts/migrate.sh mcp`.
- [ ] Ingress equivalent of the dev nginx (`deploy/dev/docker/nginx/orca.conf`): `/mcp` proxied to api-gateway with buffering off and long read timeout (SSE, `ci/check-nginx-mcp-routing.sh` is the reference); `/.well-known/oauth-protected-resource*` and `/.well-known/oauth-authorization-server` public; `/oauth/authorize|token|register|revoke` to the gateway; `/oauth/consent` served by the SPA, NOT proxied to the gateway.
- [ ] Prometheus scrapes `HTTP_PORT/metrics` of api-gateway and mcp-service (internal port only). There is no Helm chart or ServiceMonitor in this repository (location of `orca-go-common-chart` not verified), so wiring the scrape and `deploy/alerts/mcp.rules.yaml` is an operator task.

### Metrics

Gateway (`HTTP_PORT/metrics`, registry private to the process): `orca_mcp_requests_total{method,result}`, `orca_mcp_tool_calls_total{tool,decision,result}`, `orca_mcp_tool_duration_seconds{risk,namespace}`, `orca_mcp_sessions_active` (this replica), `orca_mcp_sse_streams_active`, `orca_mcp_resume_total{result}`, `orca_mcp_approvals_total{outcome}`, `orca_mcp_policy_denials_total{reason}`, `orca_mcp_auth_failures_total{reason}`, `orca_mcp_principal_resolve_total{result}`, `orca_mcp_session_identity_mismatch_total`, `orca_mcp_sse_events_dropped_total{reason}`, `orca_mcp_rate_limited_total`, `orca_mcp_requests_cancelled_total`, `orca_mcp_sessions_closed_total{reason}`. No tenant, user, session, client or trace id is ever a label; tool names are only taken from the catalog (unknown => `unknown`). mcp-service: `orca_mcp_sessions_active` (cluster-wide, sampled from the database every 30s).

Known gaps: no `orca_mcp_killswitch_active` gauge (the kill_switches RLS policy lets the relay read only cleanup rows, so a cross-tenant gauge needs a new policy); approval `expired` is counted as `denied` at the gateway; `expired`/`audience`/`revoked` auth failure reasons are not distinguished by the bearer verifier (all `invalid_token`); the alert rules in `deploy/alerts/mcp.rules.yaml` are not loaded by anything in this repository.

### Tracing and audit

The gateway starts `mcp.request` (per JSON-RPC request) -> `mcp.policy` (PolicyGate.Decide) -> `mcp.dispatch` (channel call). The gateway's mcp-service connection propagates W3C trace context (otelgrpc), the server side continues it (`grpcmw.StatsHandler`), and the audit event of a finished tool call carries `metadata.trace_id`. Span attributes are an allow-list (no arguments, tokens, cookies or secrets).
