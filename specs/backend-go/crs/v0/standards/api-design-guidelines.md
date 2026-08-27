# API Design Guidelines

## Schema-first: the `.proto` is the contract

Every service's API starts as a `.proto` file in the central `buf` module
(`proto/orca/<service>/v1/...`). The gRPC service is generated from it; the
REST facade (`grpc-gateway`) and any client SDKs are generated from the
*same* file — there is exactly one hand-written schema per API, not a proto
plus a separately-maintained OpenAPI doc that can drift.

## Versioning

- Package-level versioning (`orca.task.v1`, and a hypothetical breaking
  change ships as `orca.task.v2` alongside, not instead of, `v1` until every
  consumer has migrated) — standard proto/gRPC practice, chosen over
  header-based or URL-path REST versioning because it's enforced by the
  package system itself, not convention.
- `buf breaking` runs in CI on every PR touching a `.proto` file, comparing
  against the last-released version — a breaking change to a released
  package fails CI, full stop. Adding a new field, a new RPC, or a new
  package version is fine; changing/removing a field or RPC signature in an
  already-released package is not.
- Deprecation: mark with `[deprecated = true]` in proto, keep serving for a
  minimum one release cycle, remove only in the next major package version.

## Request/response conventions

- Every RPC's request message name is `<Verb><Noun>Request` (e.g.
  `CreateTaskRequest`), response `<Verb><Noun>Response` — no bare
  primitives as top-level request/response types, even for simple calls,
  so a field can be added later without a breaking change.
- Pagination: cursor-based (`page_token`/`next_page_token`), not
  offset-based — stable under concurrent writes, standard for gRPC list
  APIs (matches Google's AIP-158).
- Idempotency: mutating RPCs that a client might reasonably retry
  (`CreateTask`, `WriteCredential`) accept an optional `request_id` field;
  the service deduplicates on it within a bounded window. Required for any
  RPC invoked by an at-least-once event consumer.
- Field validation constraints declared in-proto via `protovalidate`
  annotations, not hand-written validation code duplicated across services.

## Error model

- gRPC status codes used per their canonical meaning (`NOT_FOUND`,
  `ALREADY_EXISTS`, `PERMISSION_DENIED`, `FAILED_PRECONDITION`,
  `INVALID_ARGUMENT`, …) — not everything collapsed to `INTERNAL` or
  `UNKNOWN`. The domain-error → status-code mapping table lives once in
  `orca-go-common` (see [`go-coding-standards.md`](./go-coding-standards.md)).
- Every error response includes a machine-readable `error_code` (a stable
  string, e.g. `TASK_CYCLIC_DEPENDENCY`) in the gRPC status details, not
  just a human-readable message — clients (frontend, other services) should
  never parse error message strings to decide behavior.
- REST facade (via `grpc-gateway`) maps gRPC status codes to HTTP status
  codes using the standard mapping (`NOT_FOUND` → 404, `PERMISSION_DENIED`
  → 403, etc.) — no custom REST error shape invented separately from what
  gRPC already defines.

## REST-specific conventions (at the `api-gateway` edge)

- Resource-oriented URLs generated from proto (`grpc-gateway`'s
  `google.api.http` annotations) — `GET /v1/tasks/{task_id}`, not an RPC-style
  `/v1/tasks/getTask` — even though the underlying transport is gRPC, the
  public REST surface should read like a REST API to frontend/CLI
  consumers who never see the proto.
- WebSocket endpoints (real-time surfaces) are documented per-service in
  that service's own doc, since their framing (what messages flow over the
  socket) is domain-specific and doesn't fit the general REST/gRPC
  conventions here.

## Backward compatibility with existing TS clients (during migration)

While the strangler-fig migration
([`migration/ts-to-go-migration-strategy.md`](../migration/ts-to-go-migration-strategy.md))
is in progress, the REST facade for any already-cut-over service must
preserve the **response shape** the frontend/mobile/CLI already expect from
the equivalent TS RPC method wherever practical, to avoid needing a
coordinated frontend change on every single service cutover. Where the Go
service's data model has genuinely improved (e.g., proper relational task
grants instead of the TS system's ad hoc structures), a compatibility
mapping layer in `api-gateway` translates — not a requirement that the Go
internal model stay TS-shaped forever, just that the *external contract*
doesn't force simultaneous frontend and backend changes for every service
cutover.
