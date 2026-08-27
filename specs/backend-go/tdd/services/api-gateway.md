# api-gateway

Category: Edge · Owns: no persistent business data — routing, session/JWT
validation, ephemeral WS session state · Migration phase: 5 (last) ·
Replaces: `runtime/rpc/dispatcher.ts`, `WsSessionRouter`, `SessionManager`'s
per-user `fork()` model, `http-server.ts`, and the routing responsibility
(not the business logic) of the ~60 TS RPC method-namespace handlers, per
[`00-service-catalog.md`](./00-service-catalog.md) and
[`02-microservices-decomposition.md`](../architecture/02-microservices-decomposition.md).

## 1. Overview & responsibility

`api-gateway` is the only service with an external-facing listener. Every
browser, mobile, and CLI client talks to it and nothing else; it talks to
the other 16 services over internal gRPC (mTLS, mesh-only ingress), never
the reverse. Per
[`08-inter-service-communication.md`](../architecture/08-inter-service-communication.md)'s
"API Gateway responsibilities" section, which this doc expands on, it
terminates TLS, validates the session cookie or JWT, extracts tenant/user
context onto outbound gRPC metadata, routes REST via `grpc-gateway`
(generated from every service's `.proto`), bridges WebSocket connections to
gRPC server-streaming calls, rate-limits/caps request size/applies
WAF-style sanitization ahead of every internal call, and — during
migration only — dual-routes some namespaces to the still-TS backend and
others to cut-over Go services (§10).

What it does not do: implement a business rule, own a row of business
data, or run AI inference. An `if` deciding business behavior, rather than
routing/auth/policy behavior, belongs downstream.

## 2. Bounded context — pure edge, zero business logic

`api-gateway` sits outside every domain boundary in
[`02-microservices-decomposition.md`](../architecture/02-microservices-decomposition.md):
no database, no domain entities with invariants (routing config is closer
to static config than a business concept), and no cross-service response
orchestration — if a REST call needs data from two services, that
composition belongs to the calling client or to a service exposing a
composed read, not to a gateway orchestration layer.

**Why this can be stateless when the TS system was not.** The TS system
isolates users by forking a whole Node.js process per user
(`SessionManager.getOrSpawnUserProcess()`: Unix socket per user,
heap-limited, idle-timeout + auto-respawn, routed to by `WsSessionRouter`
— `backend/src/main/session/{session-manager,ws-session-router}.ts` per
[`backend-hld-c4.md`](../../backend/api/backend-hld-c4.md)'s C2/C3). That
isolation exists because **all business logic for a user runs inside that
one process** — every domain service, DB access, credential handling is
in-process TypeScript, so a runaway operation for one user could starve
every other user sharing it. Forking a process was the only isolation tool
available at that cost.

In the Go model the same safety property comes from elsewhere: business
logic lives in the 16 backend services, each already isolated as a
separate deployable with its own database, resource limits, and scaling
(decomposition doc's principles 1–2). `api-gateway` runs no business
logic, so no user-specific mutable state or computation ever lives in a
gateway replica needing isolation. It needs to **correctly propagate
tenant/user identity on every call**, via a validated JWT/session
translated into gRPC metadata (§9), not an OS process boundary — a
sufficient substitute for what process forking bought the TS system. Any
replica can therefore handle any connection: a normal stateless service
behind a normal load balancer (§8).

## 3. API surface

Unlike every other service, `api-gateway` defines no `.proto` package of
its own — its surface is **derived** from every other service's proto.

**REST facade (via `grpc-gateway`).** Every backend service's `.proto`
carries `google.api.http` annotations (`04-tech-stack.md`'s "Public edge
API" row); `grpc-gateway` reads those at build time and generates the
REST↔gRPC translation this service serves. E.g. `task-service` declaring
`rpc CreateTask(...) { option (google.api.http) = { post:
"/v1/projects/{project_id}/tasks" body: "*" }; }` yields a route with no
hand-written handler — the generated stub marshals JSON↔protobuf and
forwards over gRPC. Routes are resource-oriented, plural nouns, nested
along the logical-FK relationships already established (e.g.
`/v1/projects/{project_id}/tasks/{task_id}/comments`, not a flat
`?taskId=`). No separately maintained OpenAPI spec — the proto is the one
schema for both contracts, per `04-tech-stack.md`'s rejection of
maintaining both by hand. The one hand-written piece is a thin
request-context-extraction middleware ahead of every generated route:
validates the session/JWT, resolves tenant/user, applies rate limiting and
size limits (§9), and injects resolved identity into outbound gRPC
metadata — the only non-generated logic of real complexity on the REST
path.

**WebSocket endpoint set.** Real-time surfaces are WS at the edge, backed
by gRPC server-streaming to the owning service (`04-tech-stack.md`'s
"Real-time" row):

| WS endpoint (indicative) | Owning service | TS equivalent |
|---|---|---|
| `/ws/terminal/{connection_id}` | `infra-fleet-service` | Dev Server Agent PTY relay via `WsSessionRouter` |
| `/ws/notifications` | `notification-service` | `WebPushManager` in-app WS fan-out |
| `/ws/agent-status/{execution_id}` | `orchestration-service` / `workflow-service` | polling + WS push in `runtime/orchestration/` |

Each WS endpoint maps to exactly one owning service; a connection is never
fanned out to more than one. See §8 for the bridging mechanism.

## 4. Domain model

Minimal — nothing here carries business invariants:

- **`RoutingRule` / `ServiceRegistry`** — which proto package
  (`orca.<service>.v1`) and upstream address a route or WS endpoint maps
  to. Mostly static config (compiled from the `buf` module's package list
  plus Kubernetes DNS names, `08-inter-service-communication.md`'s
  service-discovery section), not a rich domain model — one typed
  structure so the generated mux, the WS bridge, and the dual-routing
  behavior during migration (§10) share a source of truth.
- **`WSSession`** — ephemeral state for one live WS↔gRPC-stream bridge:
  connection ID, resolved tenant/user, the owning service's stream handle,
  and the bounded frame buffer used for backpressure (§8). No identity
  beyond the connection's lifetime; if it drops, `WSSession` is discarded
  — nothing to fail over, per §2.

## 5. Data model

**None persisted.** No PostgreSQL database, no `migrations/`, no
`adapter/postgres/` — the same deliberate omission
[`git-gateway-service.md`](./git-gateway-service.md) §5 makes, for the
same reason: bounded to own no state.

Ephemeral, in-memory, per-replica structures exist and are explicitly
**not durable state**: a **connection registry** of the live `WSSession`s
a given replica holds (not shared across replicas; a restart drops its
connections and clients reconnect to any replica, no affinity to
preserve, §2); a **JWKS cache** of `auth-service`'s public keys (short
TTL, safe to lose and re-fetch); and **rate-limit counters**
(per-tenant/per-user token-bucket state, §9), per-replica or backed by a
shared fast store (Redis) as a build-time choice — either way disposable
counters, not business data. If any of these ever needs to survive a
restart or be visible cross-replica for a real product reason, that data
belongs in one of the 16 services, not in added persistence here.

## 6. Package layout notes

This service departs most from
[`03-clean-architecture-guidelines.md`](../architecture/03-clean-architecture-guidelines.md)'s
standard layout, intentionally. It is almost entirely **adapter**: inbound
HTTP/WS on one side, outbound gRPC clients to all 16 other services on the
other, with a thin `usecase/` layer that exists only for cross-cutting
request handling (auth validation, rate-limit decisioning, WS-bridge
lifecycle) — not per-domain business use cases, because there are none.

```
api-gateway/
├── cmd/server/main.go
├── internal/
│   ├── domain/                    # RoutingRule, ServiceRegistry, WSSession — value objects, no invariants (§4)
│   ├── usecase/                   # cross-cutting request handling, not per-domain business use cases
│   │   ├── validate_identity.go   # session cookie / JWT -> resolved tenant/user context
│   │   ├── rate_limit.go          # token-bucket decision, per tenant/user
│   │   ├── bridge_ws_session.go   # WS connection lifecycle -> gRPC stream lifecycle (§8)
│   │   └── ports.go               # JWKSClient, RateLimitStore, ServiceRegistry lookup
│   └── adapter/
│       ├── http/                  # chi router: generated grpc-gateway mux + context-extraction middleware
│       ├── ws/                    # inbound: WS upgrade handling, frame read/write loops
│       ├── grpcclients/           # outbound: one generated client per upstream service (all 16)
│       ├── authclient/            # outbound: auth-service JWKS fetch/cache
│       └── config/                # ServiceRegistry static config, rate-limit thresholds
├── proto/                         # none owned — imports every other service's proto package for client codegen
└── go.mod
```

No `adapter/postgres/`, `adapter/vault/`, or `adapter/eventbus/` — no owned
state (§5) and no domain events of its own (routing a request is not a
fact other services react to asynchronously).

This is the second service to deviate, but for a different reason than
the other: `git-gateway-service` ([`git-gateway-service.md`](./git-gateway-service.md)
§6) has a thin `domain/`/`usecase/` because it owns no data and its only
logic is resolve→dispatch→translate for one well-defined domain (git) —
still the standard three-tier shape, just shallow. `api-gateway` inverts
the *proportions* entirely: `adapter/` is the overwhelming majority of the
codebase, and `usecase/` holds only request-level concerns. Both
deviations are intentional, not evidence of failing to follow
`03-clean-architecture-guidelines.md` — a data-owning service this thin
would be a smell; here it's the correct shape for an edge-only,
no-business-logic service.

## 7. Dependencies

`api-gateway` calls every other service — the one node in the
decomposition doc's dependency graph with an edge to all 16 others (`gw
--> auth`, `tenant`, `proj`, `infra`, `git`, `scm`, `issue`, `aiprov`,
`wf`, `task`, `orch`, `auto`, `annot`, `notif`, `usage`;
`credential-broker-service` is reached only indirectly via
`infra-fleet-service`'s credential path).

It is called by `frontend` (REST + WS, browser session cookie), `mobile`
(REST + WS, JWT + refresh token), and CLI clients (REST, JWT) — all three
out of this doc set's scope per the [README](../README.md). No internal
service calls `api-gateway`; it is a pure entry point, never a downstream
dependency.

## 8. Non-functional requirements

`api-gateway` is the system's single point of external exposure, so its
availability/latency/DDoS-resilience requirements are the highest in the
system — every other service's SLOs are conditioned on requests actually
arriving through this one. **Availability** target is the highest in the
catalog (recommend 99.95%+) — an outage here is total, regardless of
downstream health. **Latency overhead** should be single-digit
milliseconds p50 on top of the routed-to service's own budget (e.g.
`git-gateway-service`'s p50 < 150ms target is a target for *that* service;
this gateway must not erode it). **Horizontal scaling is trivial;
downstream capacity is the real constraint** — stateless (§2, §5) means
scaling out is just adding replicas, no pool-sizing ceiling, no affinity
to preserve; adding gateway replicas past what the 16 backend services
(and their DB pools, `04-tech-stack.md`) can absorb doesn't help, it moves
the bottleneck one hop in, so capacity planning here is as much a
downstream-capacity exercise as a replica-count one. **DDoS resilience**
relies on TLS termination, rate limiting, and request-size limits (§9) as
the first line of defense; production layers network-level protection
(WAF/CDN ahead of cluster ingress) in front of this, out of scope here but
assumed given the "enterprise level" bar in `04-tech-stack.md`.

### WS ↔ gRPC-stream bridging (detail)

The trickiest mechanism in the service. For each WS endpoint (§3): the
client opens a WS connection (e.g. `/ws/terminal/{connection_id}`); the
gateway validates the session/JWT (§9) and resolves the owning service
from `ServiceRegistry` (§4); it opens a corresponding gRPC streaming call
to that service, passing resolved tenant/user identity as gRPC metadata —
the same identity-propagation mechanism §2 describes for unary calls;
frames then pipe both directions (WS frame in → gRPC stream message out,
gRPC stream message in → WS frame out), with the gateway never
interpreting frame contents — a transport bridge, not a protocol
translator.

**Backpressure**: each `WSSession` holds a bounded buffer per direction.
If the client's WS socket is a slow consumer, gRPC-to-WS frames queue up
to a fixed bound; beyond it the gateway applies a documented
drop/slow-consumer policy (drop oldest non-critical frames, e.g.
intermediate terminal output chunks; never drop connection-lifecycle or
final frames) rather than growing the buffer unboundedly — the same shape
as the TS system's `ws-outbound-backpressure-queue.ts`, per
[`09-observability-reliability.md`](../architecture/09-observability-reliability.md)'s
"Backpressure on WS/streaming" row. The reverse direction (WS-to-gRPC,
e.g. keystrokes) is bounded the same way — an unbounded buffer in either
direction is a memory-exhaustion vector under many concurrent slow
connections, unaffordable at the system's single point of exposure. Either
side closing (WS close, or the gRPC stream ending/erroring) tears down the
other and discards the `WSSession`; nothing is recorded for another
replica to pick up — per §2, a dropped connection is just a dropped
connection, not a failover event.

```mermaid
sequenceDiagram
    participant C as Client (frontend/mobile)
    participant GW as api-gateway
    participant Svc as Owning service<br/>(infra-fleet-service / notification-service)

    C->>GW: WS upgrade /ws/terminal/{connection_id}
    GW->>GW: validate session/JWT, resolve tenant/user
    GW->>GW: ServiceRegistry lookup -> owning service
    GW->>Svc: gRPC streaming call (tenant/user in metadata)
    Svc-->>GW: stream established

    loop connection lifetime
        C->>GW: WS frame (e.g. keystrokes)
        GW->>Svc: gRPC stream message
        Svc-->>GW: gRPC stream message (e.g. PTY output)
        alt WS socket keeping up
            GW-->>C: WS frame
        else WS socket is slow consumer
            GW->>GW: buffer bounded; over bound -> drop per policy
        end
    end

    alt client closes WS
        C->>GW: WS close
        GW->>Svc: cancel gRPC stream
    else owning service ends/errors stream
        Svc-->>GW: stream end/error
        GW-->>C: WS close
    end
    GW->>GW: discard WSSession
```

## 9. Security notes

`api-gateway` is the primary security perimeter — every control below runs
before a request reaches any internal service: **TLS termination** (the
only service with an external listener, §1; every hop past it is
internal-cluster mTLS gRPC, `07-security-architecture.md`'s
"Service-to-service transport security"); **session/JWT validation**
(`07-security-architecture.md`'s AuthN table) — HTTP-only session cookie
(`SameSite=strict`, `Secure` always on, not gated by environment; the TS
system's `NODE_ENV==='production'`-gated `Secure` flag is named there as a
bug not to repeat) for browser, short-lived RS256 JWT validated against
`auth-service`'s JWKS for mobile/CLI; **identity propagation, not process
boundaries** (§2) — the gateway resolves tenant/user from the validated
credential and attaches it to outbound gRPC metadata, never trusting a
tenant ID from a request body, matching `07-security-architecture.md`'s
multi-tenancy layer 4; **coarse-grained authorization via OPA** — the
gateway calls OPA (sidecar or embedded Go SDK) for "can this JWT call this
endpoint" checks before routing, while fine-grained domain-specific checks
(e.g. task-grant ancestor inheritance) stay in-process in each downstream
service, not duplicated here; **rate limiting**, per-tenant and per-user,
ahead of routing, the first line of defense against abusive clients and
downstream overload (§8); **request size limits**, rejecting oversized
bodies before forwarding; and **basic WAF-style input sanitization** — a
coarse, edge-level structural rejection, not a substitute for the
proto-level `protovalidate` each downstream service already applies per
`07-security-architecture.md`'s "Input validation" section.

**No secrets of its own** — beyond mTLS service identity and possibly a
Redis connection for shared rate-limit state (§5), no
`adapter/vault/secretStore` for business secret material — the same
posture as [`git-gateway-service.md`](./git-gateway-service.md) §9, for the
same reason: it never needs one.

## 10. Migration notes

Phase 5 — last. Per
[`ts-to-go-migration-strategy.md`](../migration/ts-to-go-migration-strategy.md)'s
Phase 5 section, `api-gateway` is stood up only once every other service
is running in Go and stable; cutting the system's single point of external
exposure over any earlier would mean routing through downstream
dependencies before they're trustworthy — the highest-blast-radius mistake
available in this migration.

**Dual-routing during Phases 1–4.** Before Phase 5, the real `api-gateway`
does not exist in production. Its role is played by the **TS RPC
dispatcher itself, modified to act as a thin proxy** — per the migration
doc's Phase 1 description ("TS `backend/`'s RPC handler for that namespace
becomes a thin proxy to the new Go service's gRPC API"), reusing the
pattern ADR-021 §5 already proposed for the TS system's own eventual
extraction ("mỗi namespace handler trở thành 1 thin proxy"). During this
window the effective gateway does **dual routing**: some RPC namespaces
are still handled in-process by the TS dispatcher (not yet cut over),
others are forwarded over gRPC to their new Go service (already cut over,
per whatever phase is current — migration doc's Phase 1–4 ordering). This
is expected, temporary, and per-namespace — a namespace flips from
"handle in TS" to "proxy to Go" independently, gated by its own dual-write
soak period and cutover criteria. `ServiceRegistry` (§4) is the natural
place this per-namespace routing target lives even before the real
`api-gateway` exists, since the TS-side thin-proxy needs the same "which
target owns this namespace right now" lookup the real gateway needs
permanently — keeping that config shape consistent reduces what Phase 5
has to reconcile. Only at Phase 5 does routing become 100%
Go-service-targeted and the TS proxy layer retired: "replace the TS RPC
dispatcher entirely with the real `api-gateway`... Decommission
`backend/`'s TS codebase for server-mode deployment" (migration doc,
Phase 5).

**Electron desktop mode is out of scope.** Per the migration doc's Phase 5
section and this doc set's [README](../README.md) scope statement,
Electron desktop mode never routes through `api-gateway` at all —
`desktop/`/`frontend/` in single-user Electron mode talk to the TS backend
directly today, unaffected by this migration. `api-gateway` is exclusively
the server-mode multi-user deployment's edge; a Go rewrite of the
Electron-local path is a separate, unscoped effort.
