# TASK-WF-03-07: Extend `ListTemplates` with search/tags/sort + add `RateTemplate`

**From Solution:** SOL-WF-03
**Priority:** P1
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/internal/usecase/list_templates.go`
**Depends on:** TASK-WF-03-01, TASK-WF-03-03
**Status:** `[ ]` TODO

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
