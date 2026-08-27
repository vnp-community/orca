# `annotation-service`

## 1. Overview & responsibility

`annotation-service` owns inline code-review comments: a note anchored to a
specific file+line in a diff or PR. It replaces
`backend/src/main/code-review/annotation-store.ts` and the `annotation.*`
RPC namespace (`annotation.list` / `annotation.create` — 2 methods; see
[`business-capabilities.md`](../../backend/api/business-capabilities.md)'s
"Annotation (code review comments)" entry).

This is deliberately the **simplest service in the catalog**: one table, no
cross-service writes, no agent involvement, pure Postgres CRUD behind a thin
gRPC face. The TS implementation is already the target shape — this is a
faithful port, not a gap-fix. Its value is as a low-risk **pilot** alongside
`usage-service`: proving the Go/Clean-Architecture/Postgres/Vault pattern
end-to-end on a domain simple enough that any friction found is
attributable to the pattern itself, not unrelated domain complexity.
Simplicity does not mean exemption from the standard, though: own database,
own Vault-issued DB credentials, full Clean Architecture layering, and the
full [production-readiness checklist](../standards/production-readiness-checklist.md)
all still apply.

**Category**: Supporting (service #10 in
[`02-microservices-decomposition.md`](../architecture/02-microservices-decomposition.md#service-catalog)).
**Migration phase**: 1 — pilot tier, lowest complexity in the whole catalog.

## 2. Bounded context

An annotation is a **logical reference**, not a copy of reviewed content: it
anchors to `(repository, file path, line number, commit-or-ref)` plus
comment text, author, and timestamps. This service does not own or cache
file/diff content — that's `git-gateway-service`'s (repo content) and
`project-service`'s (project/repo identity) concern. `annotation-service`
only knows "this text was said about that anchor"; resolving the anchor
against live diff content is a caller concern. `project_id` (§5) is a
logical FK in the ADR-021 sense — validated via `project-service`, never a
cross-database join.

## 3. API surface (gRPC service sketch)

Proto package `orca.annotation.v1`, per
[`08-inter-service-communication.md`](../architecture/08-inter-service-communication.md)
(tenant context via metadata, mandatory deadlines):

```proto
service AnnotationService {
  rpc CreateAnnotation(CreateAnnotationRequest) returns (Annotation);
  rpc GetAnnotation(GetAnnotationRequest) returns (Annotation);
  rpc UpdateAnnotation(UpdateAnnotationRequest) returns (Annotation);   // author-only; see §9
  rpc DeleteAnnotation(DeleteAnnotationRequest) returns (google.protobuf.Empty); // author-only; see §9
  rpc ListAnnotationsByFile(ListAnnotationsByFileRequest) returns (ListAnnotationsResponse);   // project + file + line
  rpc ListAnnotationsByReview(ListAnnotationsByReviewRequest) returns (ListAnnotationsResponse); // project + review/PR id
}
```

TS shipped only `list`/`create`; `UpdateAnnotation`/`DeleteAnnotation` are
added here as the natural rest of CRUD for an author-owned entity, not a
gap being closed. `ListAnnotationsByFile` (point lookup on
`project+file+line`, the review-panel line-click case) and
`ListAnnotationsByReview` (`project+review`, "show all comments on this
PR") split TS's single overloaded `list` into its two real query shapes.

## 4. Domain model

```go
// internal/domain/annotation.go
type Annotation struct {
    ID        AnnotationID
    ProjectID ProjectID  // logical FK -> project-service
    ReviewID  *ReviewID  // optional PR/diff context; nil for a plain file comment
    Anchor    Anchor
    Content   string
    AuthorID  ActorID
    CreatedAt time.Time
    UpdatedAt time.Time
}

type Anchor struct {
    FilePath   string
    LineNumber int
    Ref        string // commit SHA/ref the line was resolved against, so a
                       // later diff rebase doesn't silently misattach it
}
```

No resolved/unresolved thread state — TS's `Annotation` type has none, and
adding it would invent complexity the source domain doesn't have. If
review-thread resolution becomes a requirement later, it's an additive
`ResolvedAt`/`ResolvedBy` column, not a redesign. Constructor invariants
(`LineNumber > 0`, non-empty `FilePath`/`Content`) are enforced in
`domain/`, per
[`03-clean-architecture-guidelines.md`](../architecture/03-clean-architecture-guidelines.md).

## 5. Data model

One table, mirroring TS migration 0018 (`orca_annotations`):

```sql
-- migrations/0001_annotations.sql
CREATE TABLE annotations (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   UUID    NOT NULL,  -- logical FK -> project-service
    review_id    UUID,              -- nullable: PR/diff context
    file_path    TEXT    NOT NULL,
    line_number  INTEGER NOT NULL,
    ref          TEXT    NOT NULL,  -- commit SHA/ref the anchor resolved against
    content      TEXT    NOT NULL,
    author_id    UUID    NOT NULL,  -- no local FK; auth-service is system of record
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_annotations_file_lookup ON annotations (project_id, file_path, line_number);
CREATE INDEX idx_annotations_review ON annotations (project_id, review_id) WHERE review_id IS NOT NULL;
```

`author_id` has no local FK, same rationale TS's migration 0018 gave for
`orca_task_grants.granted_by`: the authenticated actor id isn't guaranteed
to be a clean users-table row for every transport, so a hard FK would
reject valid inserts.

## 6. Package layout notes

Standard layout, nothing unusual — see
[`03-clean-architecture-guidelines.md`](../architecture/03-clean-architecture-guidelines.md#standard-package-layout):
`domain/`, `usecase/` (one file per RPC in §3), `adapter/grpc/` +
`adapter/postgres/`. No `eventbus/` (no events to publish), no `external/`
(no third-party integration), no service-specific use of `vault/` beyond the
standard DB-credential lease every service has.

## 7. Dependencies

- **Called by `api-gateway`** for all annotation CRUD from the review UI —
  the only synchronous caller.
- **May call `project-service`** (`GetProject`) to validate `project_id`
  refers to a real, accessible project before persisting a new annotation —
  the only outbound service dependency this service has.
- **No dependency on `git-gateway-service`**: this service never reads file
  or diff content, only the anchor description of where a comment points
  (§2).
- **No agent/execution-plane involvement** — unchanged from TS, which is
  "pure Postgres, zero agent involvement" per `business-capabilities.md`.

## 8. Non-functional requirements

Nothing beyond the architecture-wide SLO floor in
[`09-observability-reliability.md`](../architecture/09-observability-reliability.md)
applies. No service-specific latency/throughput/availability target is
worth calling out for a single-table CRUD service with no external call in
its hot path beyond the optional §7 lookup, and no background processing.
Standard `pgxpool` sizing, OTel tracing/metrics, and structured logging via
`orca-go-common` — same defaults as every service in
[`04-tech-stack.md`](../architecture/04-tech-stack.md).

## 9. Security notes

- **Tenant isolation**: `project_id` scoping plus the standard tenant
  context from gRPC metadata (interceptor-enforced, per
  `08-inter-service-communication.md`) — no annotation-specific mechanism.
- **Author-only edit/delete, admin override, via OPA**: `UpdateAnnotation`/
  `DeleteAnnotation` require the caller's actor ID to match `author_id`, or
  an admin/project-owner policy grant, evaluated as an OPA policy (per
  [`04-tech-stack.md`](../architecture/04-tech-stack.md)'s "Auth & policy"
  row) rather than an inline check in the usecase. TS's store had no
  update/delete path to protect, so this is new — but it's the obvious
  policy for an author-owned comment, not a judgment call worth belaboring.
- **Own Vault-issued DB credentials**, scoped to this service's own
  database only, same as every service — see
  [`06-secrets-vault-architecture.md`](../architecture/06-secrets-vault-architecture.md).
  No other secret material is involved.

## 10. Migration notes

- **Phase 1, pilot tier** — see
  [`00-service-catalog.md`](./00-service-catalog.md). Recommended as an
  early pilot alongside `usage-service`: no cross-service dependency to
  stand up first, no legacy TS behavior gap to design around.
- **Backfill**: one-time copy from `orca_annotations` (TS migration 0018)
  into `annotations` (§5). Direct column mapping (`body`→`content`,
  `file_path`, `line_number`, `author_id`, `created_at`), with one gap:
  `ref` has no TS source column (TS never captured which commit/ref a line
  number resolved against), so backfilled rows get a sentinel `ref` (e.g.
  the project's default branch at backfill time); new rows populate it
  properly from creation onward. `updated_at` backfills to `created_at`
  (TS never supported editing an annotation).
- A maintenance-window batch copy plus cutover of the `annotation.*` route
  in `api-gateway` is sufficient — no dual-write or phased-read cutover is
  warranted for a domain this size.
