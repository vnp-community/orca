# TASK-WF-03-01: Add visibility/sharing/rating schema migration `0008`

**From Solution:** SOL-WF-03
**Priority:** P0 — everything else in this solution reads/writes these columns
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/migrations/0008_template_visibility_sharing.up.sql`
**Depends on:** TASK-WF-01-01 (this migration builds on the `templates.owner_id`/`usage_count`/`tags` columns SOL-WF-01's `0007_template_authoring_fields` migration adds — `0007` must land first)
**Status:** `[x]` DONE — migration 0008 added (schema-qualified `workflow.templates`/`workflow.ratings`/`workflow.approvals`, spec's bare table names fixed, same as 0007); `ratings` RLS via `template_id` join (no direct `tenant_id`, mirroring 0004's `execution_id`-join idiom), `approvals` RLS direct on its own `tenant_id`. Verified with a throwaway integration test (removed after passing): `up`/`down`/`up` round-trips cleanly against a real Postgres testcontainer, and `idx_workflow_approvals_one_pending_per_template` genuinely rejects a second concurrent pending approval row for the same template. `go test -tags=integration -run TestRepository_CreateAndGetTemplate` (which runs every migration via the shared harness) also passes, confirming 0008 applies cleanly on top of 0007.

---

## Context

BUG-WF-03's sharing/library/rating feature set has no schema at all
today. This migration adds `visibility`, `share_token`,
`rating_sum`/`rating_count` to `templates`, and a new `approvals` table
for the lead-requires-admin-approval gate. It reuses SOL-WF-01's
`usage_count`/`tags` columns for the trending-sort composite index rather
than re-adding them.

## Changes to make

Create `backend-go/services/workflow-service/migrations/0008_template_visibility_sharing.up.sql`:

```sql
ALTER TABLE templates
  ADD COLUMN visibility TEXT NOT NULL DEFAULT 'private'
    CHECK (visibility IN ('private','team','company','public')),
  ADD COLUMN share_token TEXT UNIQUE,           -- NULL until visibility reaches 'public'
  ADD COLUMN rating_sum   INT NOT NULL DEFAULT 0,
  ADD COLUMN rating_count INT NOT NULL DEFAULT 0; -- average computed at read time, not stored

CREATE INDEX idx_templates_visibility ON templates(tenant_id, visibility);
CREATE UNIQUE INDEX idx_templates_share_token ON templates(share_token) WHERE share_token IS NOT NULL;
-- Trending sort needs both usage_count (0007) and rating; a composite
-- index keeps ListTemplates(sort=trending) an index-only scan.
CREATE INDEX idx_templates_trending ON templates(tenant_id, visibility, usage_count DESC, rating_sum DESC);
-- Full-text search backing TASK-WF-03-07's `query` filter.
CREATE INDEX idx_templates_fts ON templates USING GIN (to_tsvector('english', name || ' ' || coalesce(description,'')));

CREATE TABLE ratings (
  template_id UUID NOT NULL REFERENCES templates(id) ON DELETE CASCADE,
  user_id UUID NOT NULL,
  stars SMALLINT NOT NULL CHECK (stars BETWEEN 1 AND 5),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (template_id, user_id)
);

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

`approvals` mirrors `orchestration-service.md` §5's `decision_gates`
table shape deliberately (`id/tenant_id/.../status CHECK/resolved_at`) —
same "a row gating a state transition until a human resolves it" idiom.
Enable RLS on both `ratings` and `approvals`, `tenant_id`-scoped (via
`template_id`'s owning row for `ratings`, which has no direct
`tenant_id` column — confirm the RLS policy shape against how this
service's other child tables scope through a parent FK), matching
`workflow-service.md` §5's RLS paragraph.

Create `backend-go/services/workflow-service/migrations/0008_template_visibility_sharing.down.sql`:

```sql
DROP TABLE approvals;
DROP TABLE ratings;
DROP INDEX IF EXISTS idx_templates_fts;
DROP INDEX IF EXISTS idx_templates_trending;
DROP INDEX IF EXISTS idx_templates_share_token;
DROP INDEX IF EXISTS idx_templates_visibility;
ALTER TABLE templates
  DROP COLUMN rating_count, DROP COLUMN rating_sum, DROP COLUMN share_token, DROP COLUMN visibility;
```

## Verify

```bash
cd /opt/repos/orca/backend-go
migrate -path services/workflow-service/migrations -database "$WORKFLOW_SERVICE_TEST_DB_URL" up
migrate -path services/workflow-service/migrations -database "$WORKFLOW_SERVICE_TEST_DB_URL" down 1
migrate -path services/workflow-service/migrations -database "$WORKFLOW_SERVICE_TEST_DB_URL" up
```

Expected: `0007` (SOL-WF-01) must already be applied for this migration
to succeed. `up`/`down`/`up` round-trips cleanly; `idx_approvals_one_pending_per_template`
rejects a second concurrent pending approval row for the same template at
the constraint level.
