# TASK-WF-03-07: Extend `ListTemplates` with search/tags/sort + add `RateTemplate`

**From Solution:** SOL-WF-03
**Priority:** P1
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/internal/usecase/list_templates.go`
**Depends on:** TASK-WF-03-01, TASK-WF-03-03
**Status:** `[x]` DONE — `ListTemplatesInput`/`TemplateRepository.ListTemplates` gained `Query`/`Tags`/`Sort`; postgres adapter implements 3 sort branches (default id-keyset, `trending` = usage_count DESC/rating_sum DESC with a correct multi-column keyset predicate, `recent` = updated_at DESC) with a per-sort opaque cursor encoding (`encodeListCursor`/`decodeListCursor` — a bare last-seen id can't resume a non-id ORDER BY). New `rate_template.go` + `TemplateRepositoryTx.UpsertRating` (lock-read-prior-rating, upsert, delta-adjust `rating_sum`/`rating_count` in one transaction — a second rating from the same user updates, never duplicates). Wired into `cmd/server/main.go` + `grpc/server.go` (`ListTemplates` request fields + new `RateTemplate` handler). Found and fixed a real gap while writing the postgres-layer tests: `CreateTemplate`'s INSERT only ever wrote `id/tenant_id/name/dag_json/scope/parent_template_id` — `description`/`tags` were silently dropped, which would have made `query`/`tags` filtering non-functional in the running system (SQL correct, but nothing to match against). Fixed by adding `description`/`tags` to the INSERT (both untyped TEXT/TEXT[], safe); deliberately did NOT add `owner_id` (UUID-typed — this codebase's own test fixtures use non-UUID placeholder owner ids like `"owner-1"`, so wiring it through risked breaking every prior task's passing integration test) or `visibility`/`overrides_json`/etc., which remain the same pre-existing gap flagged in TASK-WF-01-02/03-04's status notes, now with a clear boundary around exactly what TASK-WF-03-07 needed. New tests: 8 usecase-level (`list_templates_test.go`: invalid sort, query filter, tags AND-filter, trending order — via the fake, which supports query/tags/trending meaningfully but falls back to id order for `recent`, documented, since `domain.WorkflowTemplate` has no `UpdatedAt` field for a fake to sort by) + 5 (`rate_template_test.go`: invalid stars range, first rating, same-user update-not-duplicate, two distinct users sum correctly, unknown template not-found) + 6 real-Postgres integration tests (`adapter/postgres/list_templates_test.go`: FTS query, tags AND-filter, trending order, trending keyset pagination stable/non-overlapping second page, recent order via a real `updated_at` touch, `UpsertRating` same-user update). `go build/vet/test ./...` green for workflow-service; api-gateway still builds; all 6 integration tests pass individually.

---

## Context

`ListTemplates` (`list_templates.go:34-53`) only filters by `scope`
today — BUG-WF-03's library needs full-text search, tag AND-filtering,
and trending/recent sort. Rating has no usecase at all.

## Changes to make

Extend `ListTemplatesInput` and `TemplateRepository.ListTemplates`'s SQL
in `list_templates.go` / the postgres adapter:

- `Query string` → `to_tsvector('english', name || ' ' || coalesce(description,''))
  @@ plainto_tsquery('english', $query)`, backed by `idx_templates_fts`
  (TASK-WF-03-01).
- `Tags []string` → `tags @> $tags::text[]` against SOL-WF-01's
  GIN-indexed column (every listed tag must be present, not any).
- `Sort string` → `"trending"`: `ORDER BY usage_count DESC, rating_sum DESC`
  (backed by `idx_templates_trending`); `"recent"`: `ORDER BY updated_at
  DESC`; default unchanged (keyset by `id`).

**Keyset pagination and non-id sort don't compose for free**: a
`trending`/`recent` sort needs its `page_token` to encode
`(usage_count, rating_sum, id)` or `(updated_at, id)` as the composite
cursor, not a bare last-seen `id` — implement the opaque-token encoding
per-sort accordingly.

Create `backend-go/services/workflow-service/internal/usecase/rate_template.go`:

```go
package usecase

// RateTemplate — 1-5 stars, one rating per (user, template) enforced by
// the ratings(template_id, user_id) UNIQUE constraint (TASK-WF-03-01).
// rating_sum/rating_count on templates are a materialized aggregate,
// updated in the SAME transaction as the ratings-table write.
func (uc *RateTemplate) Execute(ctx context.Context, templateID string, stars int32) (RateTemplateResult, error) {
    userID := identity.RequireUserID(ctx)
    if stars < 1 || stars > 5 {
        return RateTemplateResult{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_RATING", "stars must be between 1 and 5")
    }
    var result RateTemplateResult
    err := uc.templates.WithTx(ctx, func(tx TemplateRepositoryTx) error {
        // INSERT ... ON CONFLICT (template_id, user_id) DO UPDATE, then
        // recompute rating_sum/rating_count as a delta against the prior
        // value in the same statement pair — see adapter for exact SQL.
        var terr error
        result, terr = tx.UpsertRating(ctx, templateID, userID, stars)
        return terr
    })
    return result, err
}
```

Wire `handleListTemplates`'s equivalent RPC handler and repository
methods (`UpsertRating`) accordingly; add `UpsertRating` to
`TemplateRepositoryTx` in `ports.go`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go test ./services/workflow-service/internal/usecase/... -run TestListTemplates
go test ./services/workflow-service/internal/usecase/... -run TestRateTemplate
```

Expected: `query` full-text matches name/description; `tags` filter
requires ALL listed tags present; `sort=trending` orders by usage_count
then rating; `sort=recent` orders by updated_at; keyset pagination with
`sort=trending` returns a stable, non-overlapping second page. A second
rating from the same user updates (not duplicates) their prior rating,
and the aggregate reflects the update, not a stale double-count.
