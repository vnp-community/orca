# Inter-Service Communication

## Three channels, chosen per interaction shape

| Channel | Used for | Why not the others |
|---------|----------|---------------------|
| **gRPC (sync)** | Request/response where the caller needs a result before proceeding — `api-gateway` → any service, `git-gateway-service` → `infra-fleet-service` to resolve a `connectionId`, `project-service` → `tenant-service` to validate a company exists | REST/JSON would work but loses compile-time-checked contracts and codegen; async messaging doesn't fit a call that needs an immediate answer |
| **NATS JetStream (async)** | Domain events other services react to eventually, not immediately — `task.completed`, `workflow.step_failed`, `credential.rotated` | Sync gRPC would couple the publisher's availability to every subscriber's availability; this is exactly the coupling event-driven architecture avoids |
| **Dev Server Agent relay protocol** | `infra-fleet-service`/`git-gateway-service` ↔ the execution plane (`agent/`) | A different system boundary entirely — see below |

## gRPC conventions

- Proto packages namespaced `orca.<service>.v1` (e.g. `orca.task.v1`),
  managed in a central `buf` module so every service's client stubs are
  generated from the same source of truth and `buf breaking` runs in CI
  against every PR touching a `.proto` file.
- Every RPC takes a request message with an explicit `tenant_id`-bearing
  context (via gRPC metadata, not a message field — this keeps tenant
  scoping enforcement at the interceptor level, not per-message, matching
  [`05-data-architecture.md`](./05-data-architecture.md)'s "never optional,
  never inferred from body" rule).
- Server-side interceptors (shared via `orca-go-common`) handle: JWT
  validation, tenant-context extraction, OpenTelemetry span creation,
  structured request logging, panic recovery → gRPC status mapping. No
  service hand-rolls this per-RPC.
- Deadlines are mandatory on every outbound call (`context.WithTimeout`) —
  no unbounded gRPC call exists anywhere in the system; default 5s for
  intra-cluster calls, overridable per call site with a documented reason.

## Event conventions (NATS JetStream)

- Subject naming: `orca.<service>.<entity>.<event>` (e.g.
  `orca.task.task.completed`, `orca.workflow.execution.started`).
- Every event payload includes: event ID (for consumer-side dedup), tenant
  ID, occurred-at timestamp, and a schema version — consumers must handle
  receiving an older schema version gracefully (additive-only event schema
  evolution, same expand/contract discipline as the DB migration policy).
- Publishing goes through the transactional outbox pattern
  ([`05-data-architecture.md`](./05-data-architecture.md)) — never a direct
  publish call inside a request handler, which would risk publishing an
  event for a transaction that later rolls back.
- Consumers are idempotent by construction (dedup on event ID, stored in
  the consuming service's own "processed events" table or a short-TTL
  cache) — JetStream's at-least-once delivery means every consumer *will*
  see duplicates eventually.

## API Gateway responsibilities

`api-gateway` is the only service exposed outside the cluster network. It:

1. Terminates TLS.
2. Validates the session cookie or JWT (calling `auth-service`'s JWKS
   endpoint, cached).
3. Extracts tenant/user context, attaches it to outbound gRPC metadata.
4. Routes REST requests to the appropriate service via `grpc-gateway`
   (generated from the same `.proto` files the services already expose —
   no separately maintained REST layer).
5. Manages WebSocket sessions for real-time surfaces (terminal streams,
   agent status, notifications) — accepts a WS connection, opens a
   corresponding gRPC server-streaming call to the owning service
   (`infra-fleet-service` for terminal, `notification-service` for push/WS
   events), pipes frames both directions. This is the direct successor to
   the TS system's `WsSessionRouter` + per-user session-process fork model,
   but stateless-by-design: any `api-gateway` replica can handle any
   connection, session affinity isn't required because there's no per-user
   forked process to route back to — state that mattered lives in the
   owning service, not in the gateway.
6. Rate limiting (per-tenant and per-user), request size limits, and basic
   WAF-style input sanitization before anything reaches an internal
   service.

## Service discovery

- Kubernetes-native: services address each other by
  `<service>.<namespace>.svc.cluster.local` DNS names — no separate
  service-registry (Consul/etcd) needed at 17 services, consistent with
  ADR-021's own "chưa cần Consul/etcd ở quy mô hiện tại" judgment for the TS
  system, carried forward here since the scale argument still holds.
- The service mesh (see
  [`10-deployment-infrastructure.md`](./10-deployment-infrastructure.md))
  layers retry/circuit-breaking/load-balancing on top of that DNS-based
  discovery — not reimplemented per-service.

## Talking to the Dev Server Agent (execution plane)

`infra-fleet-service` and `git-gateway-service` are the only two Go services
that talk to the execution plane. Two options, to be decided before
implementation starts (not decided by this document — a genuine open
question flagged here rather than papered over):

- **Option A — keep the existing wire protocol**: the TS system's 3
  connection modes (relay-ssh, relay-websocket, direct-websocket) and
  13-byte-framed JSON-RPC, unchanged. Lowest risk — the execution plane
  (`agent/`) doesn't need to change at all, only the Go services need a
  client implementation of the existing protocol.
- **Option B — modernize to gRPC streaming**: define an `orca.agent.v1`
  proto (bidirectional streaming for PTY I/O, unary for git/fs ops), and
  the execution plane adopts a gRPC server alongside or instead of its
  current WS server. Cleaner, consistent with every other inter-service
  contract in this system, but requires changes to `agent/`, which is
  explicitly out of scope for "the Go rewrite of `backend/`" as scoped by
  the user's request.

**Default recommendation: Option A for the initial Go rewrite** — preserves
the existing execution-plane contract, keeps this redesign scoped to
`backend/` as requested, and defers the (larger, cross-repo) decision to
modernize the agent protocol to a follow-up effort once the Go backend is
stable. Revisit if/when `agent/` itself is redesigned.
