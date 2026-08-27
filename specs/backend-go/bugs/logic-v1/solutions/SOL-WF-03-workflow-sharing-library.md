# SOL-WF-03: Visibility state machine, admin-approval, share-link import, library search, and rating/usage tracking for `workflow-service`

**Resolves:** [BUG-WF-03](../BUG-WF-03-workflow-sharing-not-implemented.md)
**Service:** `workflow-service` (primary) + `auth-service` (admin-approval identity check) + `api-gateway` (REST/public-preview wiring)
**Affected files (proposed):**
- `backend-go/services/workflow-service/migrations/0008_template_visibility_sharing.{up,down}.sql`
- `backend-go/proto/orca/workflow/v1/workflow.proto`
- `backend-go/services/workflow-service/internal/domain/template.go` (extended further by this solution, on top of SOL-WF-01's fields), `visibility.go` (new), `approval.go` (new)
- `backend-go/services/workflow-service/internal/usecase/ports.go` (extend `TemplateRepository`, new `ApprovalRepository`)
- `backend-go/services/workflow-service/internal/usecase/publish_template.go`, `request_approval.go`, `resolve_approval.go`, `generate_share_link.go`, `preview_shared_template.go`, `import_shared_template.go`, `search_templates.go`, `rate_template.go` (new)
- `backend-go/services/workflow-service/internal/usecase/execute.go` (usage-count increment)
- `backend-go/services/workflow-service/internal/adapter/postgres/` (extend template repo, new approval repo)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/workflow_routes.go` (new routes, incl. unauthenticated share-link preview)
- Corresponding `_test.go` files
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

- This bug and BUG-WF-01 share a root cause (both bug files say so
  explicitly): `workflow.templates`' schema was narrowed at scaffold time
  and never grew visibility/sharing/rating columns. SOL-WF-01 already adds
  `owner_id`, `description`, `tags`, and — critically for this solution —
  `usage_count` (needed there for its version-bump-on-breaking-change
  policy). **This solution builds on SOL-WF-01's schema rather than
  re-adding those columns** — its own migration (`0008_...`) only adds the
  columns SOL-WF-01 has no reason to touch: `visibility`, `share_token`,
  `rating_sum`/`rating_count`. Implementation order matters: SOL-WF-01's
  `0007_...` migration must land first.
- `02-microservices-decomposition.md` design principle 1 ("cross-service
  references are logical FKs, validated by calling the owning service's
  API") governs the admin-approval gate: "requester is a lead" and
  "approver is an admin" are `auth-service`/`tenant-service` facts
  (roles), not something `workflow-service` can determine from its own
  tables. `orchestration-service.md` §9's security note gives the exact
  precedent to follow — `ResolveDecisionGate` "requires the caller to be a
  member of the project owning the originating task, enforced via
  in-process OPA policy" — this solution's approval-resolution usecase
  uses the identical pattern: an OPA policy check against the caller's
  role, not a workflow-service-local admin flag.
- `05-data-architecture.md`'s two cross-service-consistency patterns
  (transactional outbox for eventual consistency, synchronous saga for
  "caller needs to know the outcome now") directly decide two of this
  bug's flows:
  - **Usage-count increment on execute** is the outbox case: it's a
    "service A does something, doesn't block on anything else" write —
    incrementing `templates.usage_count` happens in the SAME transaction
    that persists the execution's `running` status
    (`execute.go:114`'s `CreateExecution` call), no cross-service call
    involved at all (both rows are in `workflow-service`'s own database),
    so this is actually simpler than the outbox pattern — a single local
    transaction, not even an event needed for this specific increment (an
    event MAY still be published for `notification-service`'s benefit,
    per `workflow-service.md` §7's existing `workflow.execution.started`
    publish, but that's orthogonal to the counter itself).
  - **"Import to My Workflows"** is the saga case in miniature: the
    caller (the visitor importing someone else's shared template) is
    waiting for a result, and the operation is a single write inside
    `workflow-service`'s own database (create a new template row) — no
    actual multi-service saga is needed here either, since sharing data
    lives entirely in one service's tables. Flagged as a place this
    solution initially over-applied `05-data-architecture.md`'s saga
    framing before realizing the operation is intra-service; kept in this
    doc as a worked example of applying that decision correctly.
- `orchestration-service.md` §8's "Correctness under concurrent access"
  discipline (an explicit Postgres transaction per usecase, doc'd with a
  "why atomic" column) is the model this solution's `PublishTemplate`/
  `ResolveApproval` usecases follow for the same reason
  `orchestration-service`'s `CreateDecisionGate`/`ResolveDecisionGate` do:
  a template's `visibility` transition and its (possible) approval-request
  row must commit together, or a template can end up "pending company
  publish" with no matching approval row to resolve, or vice versa.

### Flagged as genuine extensions beyond the TDD

- **The entire approval/visibility/share/rating domain model** —
  `workflow-service.md` does not describe any of it (its §4/§5 stop at
  template/execution/step-execution). This solution designs the whole
  shape from BL-WF-03's spec (per this task's instruction to design
  against the bug's own spec summary when the TDD is silent), grounded
  only in the cross-cutting architecture docs (data-consistency patterns,
  service-boundary principles) rather than any service-specific document
  that already specifies it. This is the largest "extension" flag among
  the three bugs solved in this pass — worth a reviewer's explicit sign-off
  before implementation, not just a rubber-stamp.

---

## Design — schema (migration `0008_template_visibility_sharing`, depends on SOL-WF-01's `0007_...`)

```sql
-- 0008_template_visibility_sharing.up.sql
ALTER TABLE templates
  ADD COLUMN visibility TEXT NOT NULL DEFAULT 'private'
    CHECK (visibility IN ('private','team','company','public')),
  ADD COLUMN share_token TEXT UNIQUE,           -- NULL until visibility reaches 'public'; see generate_share_link.go
  ADD COLUMN rating_sum   INT NOT NULL DEFAULT 0,
  ADD COLUMN rating_count INT NOT NULL DEFAULT 0; -- average = rating_sum::float / NULLIF(rating_count,0), computed at read time, not stored — same "don't persist a derived value" posture as usage_count's own non-derived nature (usage_count IS the primary fact, not derived)

CREATE INDEX idx_templates_visibility ON templates(tenant_id, visibility);
CREATE UNIQUE INDEX idx_templates_share_token ON templates(share_token) WHERE share_token IS NOT NULL;
-- Trending sort needs both usage_count (SOL-WF-01's 0007_...) and
-- rating; a composite index keeps ListTemplates(sort=trending) an
-- index-only scan rather than a sort-in-memory over every matching row.
CREATE INDEX idx_templates_trending ON templates(tenant_id, visibility, usage_count DESC, rating_sum DESC);

CREATE TABLE approvals (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  template_id UUID NOT NULL REFERENCES templates(id) ON DELETE CASCADE,
  requested_by UUID NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected')),
  resolved_by UUID,
  resolved_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_approvals_pending ON approvals(tenant_id, status) WHERE status = 'pending';
CREATE UNIQUE INDEX idx_approvals_one_pending_per_template ON approvals(template_id) WHERE status = 'pending';
```

`approvals` mirrors `orchestration-service.md` §5's `decision_gates` table
shape deliberately (`id/tenant_id/.../status CHECK/resolved_at`) — same
problem shape (a row gating a state transition until a human resolves it),
same schema idiom, for a reviewer who already knows that table's pattern.

```sql
-- 0008_template_visibility_sharing.down.sql
DROP TABLE approvals;
DROP INDEX IF EXISTS idx_templates_trending;
DROP INDEX IF EXISTS idx_templates_share_token;
DROP INDEX IF EXISTS idx_templates_visibility;
ALTER TABLE templates
  DROP COLUMN rating_count, DROP COLUMN rating_sum, DROP COLUMN share_token, DROP COLUMN visibility;
```

RLS enabled on `approvals`, `tenant_id`-scoped, matching every other table
in this service's database (`workflow-service.md` §5's RLS paragraph) and
`05-data-architecture.md`'s blanket policy.

---

## Design — domain

```go
// internal/domain/visibility.go (new)
type Visibility string

const (
    VisibilityPrivate Visibility = "private"
    VisibilityTeam    Visibility = "team"
    VisibilityCompany Visibility = "company"
    VisibilityPublic  Visibility = "public"
)

// rank orders visibility for escalation-only transitions — BL-WF-03's
// state machine is escalate-forward (private -> team -> company -> public)
// with company requiring approval; de-escalation (public -> private) is a
// separate, always-allowed "unpublish" operation, not a state-machine
// transition through the intermediate tiers.
var rank = map[Visibility]int{VisibilityPrivate: 0, VisibilityTeam: 1, VisibilityCompany: 2, VisibilityPublic: 3}

func (v Visibility) Valid() bool { _, ok := rank[v]; return ok }

// CanEscalateTo reports whether moving from v to next is a valid single
// forward step (only one tier at a time — private cannot jump straight to
// public in one call, matching BL-WF-03's escalation framing) OR any
// direct de-escalation back to private (unpublish, always one step, any
// distance).
func (v Visibility) CanEscalateTo(next Visibility) bool {
    if next == VisibilityPrivate {
        return true // unpublish, always allowed
    }
    return rank[next] == rank[v]+1
}
```

```go
// internal/domain/approval.go (new)
type ApprovalStatus string
const (
    ApprovalPending  ApprovalStatus = "pending"
    ApprovalApproved ApprovalStatus = "approved"
    ApprovalRejected ApprovalStatus = "rejected"
)

type Approval struct {
    ID, TemplateID, RequestedBy string
    Status                      ApprovalStatus
    ResolvedBy                  string
    ResolvedAt                  *time.Time
}
```

`domain.WorkflowTemplate` (already extended by SOL-WF-01) gains:
`Visibility domain.Visibility`, `ShareToken string`, `RatingSum`,
`RatingCount int32`. `AverageRating() float64` is a method, not a stored
field, matching the migration comment above.

---

## Design — publish/approval usecases

```go
// internal/usecase/publish_template.go
//
// PublishTemplate is BL-WF-03's visibility-escalation entry point.
// Atomic per orchestration-service.md §8's discipline: the template's
// visibility transition and (for company scope) the approval row's
// creation must commit together — a torn write could leave a template
// silently already-company-visible with no pending approval to gate it,
// or vice versa (an orphaned approval nobody can act on).
type PublishTemplate struct {
    templates TemplateRepository // extended with a Postgres-transaction-capable method, see below
    approvals ApprovalRepository
    opa       OPAChecker // caller-role check, mirrors orchestration-service's ResolveDecisionGate
}

func (uc *PublishTemplate) Execute(ctx context.Context, in PublishTemplateInput) (domain.WorkflowTemplate, error) {
    tmpl, err := uc.templates.GetTemplate(ctx, tenantID, in.TemplateID)
    // ownership check: only owner_id (or an org admin) may publish — SOL-WF-01's OwnerID field
    if tmpl.OwnerID != callerUserID && !uc.opa.IsAdmin(ctx, callerUserID) {
        return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindPermissionDenied, ...)
    }
    if !tmpl.Visibility.CanEscalateTo(in.NewVisibility) {
        return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_VISIBILITY_TRANSITION", ...)
    }

    if in.NewVisibility == domain.VisibilityCompany && !uc.opa.IsAdmin(ctx, callerUserID) {
        // Lead (non-admin) requesting company scope: create a pending
        // approval, template's visibility STAYS AT ITS CURRENT TIER until
        // resolved — BL-WF-03's "lead-requires-admin-approval gate."
        return uc.templates.WithTx(ctx, func(tx TemplateRepositoryTx) (domain.WorkflowTemplate, error) {
            approval := domain.Approval{ID: uuid.NewString(), TemplateID: tmpl.ID, RequestedBy: callerUserID, Status: domain.ApprovalPending}
            if err := uc.approvals.CreateTx(ctx, tx, approval); err != nil { return domain.WorkflowTemplate{}, err }
            return tmpl, nil // unchanged visibility — pending, not yet published
        })
    }

    // Admin publishing directly, OR any non-company tier (team/public
    // escalation, or private unpublish) — no approval gate.
    tmpl.Visibility = in.NewVisibility
    return uc.templates.WithTx(ctx, func(tx TemplateRepositoryTx) (domain.WorkflowTemplate, error) {
        return tx.UpdateVisibility(ctx, tmpl)
    })
}
```

```go
// internal/usecase/resolve_approval.go
//
// ResolveApproval — admin-only (OPA-checked, orchestration-service.md §9's
// exact pattern: "enforced via in-process OPA policy," not a
// workflow-service-local role table). Atomic: approval.status transition
// + (if approved) the template's visibility bump to 'company' commit
// together — an approved-but-not-applied approval, or an applied bump with
// no record of who approved it, are both the "torn write" failure mode
// orchestration-service.md §8 names for its own ResolveDecisionGate.
func (uc *ResolveApproval) Execute(ctx context.Context, in ResolveApprovalInput) (domain.Approval, error) {
    if !uc.opa.IsAdmin(ctx, callerUserID) {
        return domain.Approval{}, apperrors.New(apperrors.KindPermissionDenied, "WORKFLOW_APPROVAL_ADMIN_ONLY", ...)
    }
    return uc.approvals.WithTx(ctx, func(tx ApprovalRepositoryTx) (domain.Approval, error) {
        approval, err := tx.Get(ctx, in.ApprovalID)
        approval.Status, approval.ResolvedBy, approval.ResolvedAt = in.Decision, callerUserID, ptr(time.Now())
        if err := tx.Update(ctx, approval); err != nil { return domain.Approval{}, err }
        if in.Decision == domain.ApprovalApproved {
            if err := tx.Templates().SetVisibility(ctx, approval.TemplateID, domain.VisibilityCompany); err != nil {
                return domain.Approval{}, err
            }
        }
        return approval, nil
    })
}
```

`TemplateRepository`/`ApprovalRepository` gain `WithTx` methods (a
`pool.WithTransaction`-equivalent boundary helper, matching
`orchestration-service.md` §6's `adapter/postgres/` "txn boundary helper"
package note) — this is the one piece of infrastructure this solution
needs that `workflow-service`'s current `adapter/postgres` doesn't yet
have (every existing usecase there is single-statement; this is the first
multi-statement-atomic requirement in this service). Flag this explicitly:
adding a transaction boundary helper to `workflow-service`'s postgres
adapter is new plumbing, not just new queries.

---

## Design — share link, preview, import

```go
// internal/usecase/generate_share_link.go
//
// Only meaningful once Visibility == public (a non-public template has no
// business being reachable by an anonymous share_token — the token IS the
// access-control boundary for a public template, nothing else gates it).
func (uc *GenerateShareLink) Execute(ctx context.Context, templateID string) (string, error) {
    tmpl, err := uc.templates.GetTemplate(ctx, tenantID, templateID)
    if tmpl.Visibility != domain.VisibilityPublic {
        return "", apperrors.New(apperrors.KindFailedPrecondition, "WORKFLOW_TEMPLATE_NOT_PUBLIC", ...)
    }
    if tmpl.ShareToken != "" {
        return tmpl.ShareToken, nil // idempotent — re-requesting doesn't rotate the token
    }
    token := generateOpaqueToken() // crypto/rand, base64url, matching credential-broker-service's own opaque-token convention
    return token, uc.templates.SetShareToken(ctx, templateID, token)
}
```

```go
// internal/usecase/preview_shared_template.go
//
// PreviewSharedTemplate is the ONE read this service exposes with NO
// tenant/auth context at all — a deliberate, narrow exception to
// 05-data-architecture.md's "tenant scoping is never optional" rule,
// scoped as tightly as possible: looked up by share_token (a random,
// unguessable value, not a template id an attacker could enumerate), and
// returns a read-only projection (name/description/tags/dag_json/rating)
// — never owner_id, never any other tenant's other templates, never a
// list/search capability. Every other usecase in this file keeps the
// normal tenant.RequireTenantID(ctx) requirement; this is the sole,
// explicitly-reviewed exception, flagged here so it isn't mistaken for a
// missed tenant-scoping bug during review.
func (uc *PreviewSharedTemplate) Execute(ctx context.Context, shareToken string) (SharedTemplatePreview, error) {
    tmpl, err := uc.templates.GetByShareToken(ctx, shareToken) // no tenantID param — token IS the lookup key, by design
    if err != nil || tmpl.Visibility != domain.VisibilityPublic {
        return SharedTemplatePreview{}, apperrors.New(apperrors.KindNotFound, "WORKFLOW_SHARE_LINK_INVALID", ...) // same error for "not found" and "no longer public" — don't leak which
    }
    return toPreview(tmpl), nil
}
```

```go
// internal/usecase/import_shared_template.go
//
// ImportSharedTemplate — "one-click import to my personal workflows."
// Requires a real (authenticated, tenant-scoped) caller, unlike Preview.
// Reuses SOL-WF-01's CloneTemplate machinery (a personal-scope, no-parent
// copy is exactly Clone's shape) rather than re-implementing a second
// copy path — the only difference from a same-tenant Clone is that the
// SOURCE template may belong to a DIFFERENT tenant (a public template's
// tenant_id isn't necessarily the importer's), so the lookup goes through
// GetByShareToken (cross-tenant-safe, token-scoped) rather than
// GetTemplate (same-tenant-only).
func (uc *ImportSharedTemplate) Execute(ctx context.Context, shareToken string) (domain.WorkflowTemplate, error) {
    source, err := uc.templates.GetByShareToken(ctx, shareToken)
    if err != nil || source.Visibility != domain.VisibilityPublic {
        return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindNotFound, "WORKFLOW_SHARE_LINK_INVALID", ...)
    }
    resolved, err := uc.resolveTemplate.Execute(ctx, ResolveTemplateInput{TemplateID: source.ID}) // cross-tenant resolve — ResolveChain must accept an explicit templateID+tenantID pair here rather than deriving tenantID from ctx, a signature note for whoever implements this
    importerTenantID, importerUserID := tenant.RequireTenantID(ctx), /* caller user id */
    tmpl, err := domain.NewWorkflowTemplate(uuid.NewString(), importerTenantID, source.Name+" (imported)",
        resolved.Template.DAGJSON, domain.ScopePersonal, "" /* no parent */, importerUserID)
    tmpl.ClonedFromTemplateID = source.ID // provenance across the tenant boundary — SetVisibility/SetShareToken never copied, imported template starts private
    return tmpl, uc.templates.CreateTemplate(ctx, tmpl)
}
```

`ResolveTemplate`'s current signature takes only a `TemplateID` and pulls
`tenantID` from `ctx` (`resolve_template.go:56-64`) — cross-tenant import
needs it to resolve a template belonging to a DIFFERENT tenant than the
caller. Rather than weakening `ResolveTemplate`'s own tenant-scoping (used
by every other, same-tenant caller), `ImportSharedTemplate` calls
`ResolveChain` directly with the SOURCE template's own `tenant_id` (read
off the row `GetByShareToken` already returned), not through the
tenant-scoped `ResolveTemplate` usecase — flagged as a deliberate
usecase-boundary decision, not a workaround.

---

## Design — library search & sort (`ListTemplates` extended)

```protobuf
message ListTemplatesRequest {
  string scope = 1;
  string page_token = 2;
  int32 page_size = 3;
  string query = 4;           // full-text against name/description
  repeated string tags = 5;   // AND-filter — every listed tag must be present (SOL-WF-01's tags column)
  string sort = 6;            // "trending" | "recent" | "" (default: id, today's implicit order)
}
```

`ListTemplates`'s usecase (`list_templates.go:34-53`) gains `Query`,
`Tags`, `Sort` on `ListTemplatesInput`, threaded into
`TemplateRepository.ListTemplates`'s SQL:

- `query` → `to_tsvector('english', name || ' ' || coalesce(description,''))
  @@ plainto_tsquery('english', $query)` — a GIN `tsvector` index on that
  expression, added in this migration alongside the other new indexes
  (not shown above for brevity, same file).
- `tags` → `tags @> $tags::text[]` against SOL-WF-01's GIN-indexed column.
- `sort=trending` → `ORDER BY usage_count DESC, rating_sum DESC` (backed
  by `idx_templates_trending` above); `sort=recent` → `ORDER BY updated_at
  DESC`; default unchanged (keyset by `id`, todays's implicit order).
  **Keyset pagination and non-id sort don't compose for free**: a
  `trending`/`recent` sort needs its `page_token` to encode
  `(usage_count, rating_sum, id)` or `(updated_at, id)` as the composite
  cursor, not a bare last-seen `id` — flagged as a real implementation
  detail, not hand-waved; the opaque-token convention still holds, its
  decoded shape just varies by `sort`.

---

## Design — rating & usage-count

```go
// internal/usecase/rate_template.go
//
// RateTemplate — 1-5 stars, one rating per (user, template) enforced by a
// UNIQUE constraint this migration should also add (omitted above for
// brevity: `ratings(template_id, user_id) UNIQUE`, a separate small table
// rather than folding "who rated what" into templates itself, so a
// user's rating can be changed/withdrawn without losing the aggregate's
// history). RatingSum/RatingCount on templates are then a materialized
// aggregate, updated via the SAME transaction as the ratings-table write
// (INSERT ... ON CONFLICT (template_id,user_id) DO UPDATE, paired with an
// UPDATE templates SET rating_sum = rating_sum + delta, ... in one txn) —
// same atomicity discipline as PublishTemplate/ResolveApproval above.
```

```go
// internal/usecase/execute.go — extended
//
// Usage-count increment happens in the SAME local transaction as
// CreateExecution (execute.go:114) — both rows live in workflow-service's
// own database, so this is a single Postgres statement pair in one
// transaction, not a cross-service call (see "Design rationale" above for
// why this is NOT an outbox-pattern case despite superficially looking
// like "an action that should notify something else").
func (uc *Execute) Execute(ctx context.Context, in ExecuteInput) (domain.WorkflowExecution, error) {
    // ... existing template fetch / DAG validate / wave build unchanged ...
    if err := uc.templates.WithTx(ctx, func(tx TemplateRepositoryTx) error {
        if err := tx.CreateExecution(ctx, exec); err != nil { return err } // moved inside the same tx as the increment below
        return tx.IncrementUsageCount(ctx, tmpl.ID)
    }); err != nil { /* ... */ }
    // ... unchanged async dispatch ...
}
```

---

## Design — wiring (REST)

`workflow_routes.go` gains:

```go
sub.Post("/templates/{id}/publish", handlePublishTemplate(client))         // body: {new_visibility}
sub.Get("/templates/approvals", handleListPendingApprovals(client))        // admin-only, enforced server-side via OPA
sub.Post("/templates/approvals/{id}/resolve", handleResolveApproval(client)) // body: {decision: approved|rejected}
sub.Post("/templates/{id}/share-link", handleGenerateShareLink(client))
sub.Post("/templates/{id}/rate", handleRateTemplate(client))               // body: {stars}
```

Plus two routes deliberately mounted **outside** the tenant-authenticated
group, matching `PreviewSharedTemplate`'s no-auth design above:

```go
// mounted at the router root, not under an authenticated middleware chain
// — matching api-gateway's existing pattern for any genuinely public
// endpoint (e.g. health checks); this is this service's first
// tenant-context-free business endpoint, flagged for whoever wires the
// middleware chain to confirm it's excluded from the JWT-required group
// deliberately, not by omission.
r.Get("/v1/workflows/shared/{token}", handlePreviewSharedTemplate(client))
r.Post("/v1/workflows/shared/{token}/import", handleImportSharedTemplate(client)) // this one DOES require auth (the importer's own identity) — only the PATH is public, the handler itself calls identityFromContext and 401s if absent, same as every other authenticated handler
```

`ListTemplates`'s existing REST handler (`handleListTemplates`,
`workflow_routes.go:77-104`) gains `query`/`tags`/`sort` query-param
passthrough.

`wscompat`'s `workflow.*` namespace registration (BUG-030, cross-referenced
by BUG-WF-03's own "See also") should include these new RPCs too when that
gap is picked up — not re-designed here.

---

## Test plan

- `domain/visibility_test.go` — `CanEscalateTo` allows exactly one tier
  forward, rejects skipping a tier, always allows any-tier→private.
- `usecase/publish_template_test.go` — non-owner/non-admin publish attempt
  rejected; owner escalating private→team succeeds with no approval row
  created; lead escalating to company creates a pending `Approval` and
  leaves `Visibility` unchanged; admin escalating to company applies
  immediately, no approval row; a second company-publish request while one
  is already pending is rejected by `idx_approvals_one_pending_per_template`
  (assert the usecase surfaces this as a clean, typed conflict error, not
  a raw constraint-violation leak).
- `usecase/resolve_approval_test.go` — non-admin resolve attempt rejected;
  approve applies `VisibilityCompany` atomically with the approval's
  `status` flip (assert both persisted or both rolled back — inject a
  failure between the two writes in the fake tx and assert the whole
  usecase call errors with neither side effect applied); reject leaves
  `Visibility` unchanged.
- `usecase/generate_share_link_test.go` — non-public template rejected;
  repeated calls on an already-public template return the SAME token
  (idempotency); token round-trips through `GetByShareToken`.
- `usecase/preview_shared_template_test.go` — valid public token returns
  the read-only projection; an unknown token AND a valid token whose
  template has since been unpublished both return the identical
  `WORKFLOW_SHARE_LINK_INVALID` error (assert the error is
  indistinguishable, closing an enumeration side-channel); response never
  includes `owner_id`/`tenant_id` (assert via reflection over the returned
  struct's field set, a regression guard against a future field addition
  accidentally leaking).
- `usecase/import_shared_template_test.go` — importing a cross-tenant
  public template creates a new personal-scope row under the IMPORTER's
  tenant, with the resolved (not raw) DAG baked in, `ParentTemplateID`
  empty, `ClonedFromTemplateID` set; importing a token for a
  since-unpublished template fails the same as Preview.
- `usecase/list_templates_test.go` — `query` full-text matches
  name/description; `tags` filter requires ALL listed tags present, not
  any; `sort=trending` orders by usage_count then rating; `sort=recent`
  orders by updated_at; keyset pagination with `sort=trending` returns a
  stable, non-overlapping second page (regression guard against the
  composite-cursor implementation detail flagged above).
- `usecase/rate_template_test.go` — a second rating from the same user
  updates (not duplicates) their prior rating, and the aggregate
  `rating_sum`/`rating_count` reflect the update, not a stale double-count.
- `usecase/execute_test.go` — `Execute` increments `usage_count` exactly
  once per call, atomically with `CreateExecution` (inject a failure after
  the increment but before `CreateExecution` commits in the fake tx,
  assert neither side effect landed).
- `adapter/postgres/` — migration `up`→`down`→`up` round trip; `WithTx`
  boundary helper rollback-on-error test (the new plumbing this solution
  requires, flagged above).
- `httpgateway/workflow_routes_test.go` — the two public routes are
  reachable with NO `Authorization` header and never call
  `identityFromContext` in a way that 401s a Preview request; the Import
  route DOES 401 without one.

**Needs `agent/` (Dev Server Agent) changes:** No. This bug is entirely
schema/domain/usecase/proto/REST surface inside `workflow-service` (plus
an OPA policy check against `auth-service`'s role model) — no execution-
plane interaction anywhere in the sharing/library/rating flow.

## References

- `specs/backend-go/tdd/services/orchestration-service.md:115-117` (`DecisionGate` shape, the model `approvals` mirrors), `:264-278` (§8 atomicity table/rationale, the pattern `PublishTemplate`/`ResolveApproval` follow), `:295-297` (§9, `ResolveDecisionGate`'s OPA-policy precedent for `ResolveApproval`'s admin check)
- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md:13-18` (design principle 1, logical-FK/cross-service-API rule underpinning the OPA-not-local-flag decision)
- `specs/backend-go/tdd/architecture/05-data-architecture.md:75-112` (outbox vs. saga decision framework, applied and found to be "neither, it's intra-service" for usage-count/import — see rationale above), `:34-53` (tenant-scoping rule, and `PreviewSharedTemplate`'s narrow, flagged exception to it)
- `backend-go/services/workflow-service/internal/usecase/list_templates.go:11-53` — current scope-only filter, extended above
- `backend-go/services/workflow-service/internal/usecase/execute.go:70-126` — current `Execute`, usage-count increment inserted above
- `backend-go/services/api-gateway/internal/adapter/httpgateway/workflow_routes.go:25-49,77-104` — REST route table extended above
- `specs/backend-go/bugs/logic-v1/BUG-WF-03-workflow-sharing-not-implemented.md` — problem statement
- `specs/backend-go/bugs/logic-v1/solutions/SOL-WF-01-template-authoring-fields.md` — sibling solution this one's schema (`usage_count`, `tags`, `owner_id`) and `CloneTemplate` usecase build directly on top of; implementation order dependency (SOL-WF-01's `0007_...` migration before this solution's `0008_...`)
