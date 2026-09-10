# TASK-WF-01-01: Add template authoring/sharing schema migration `0007`

**From Solution:** SOL-WF-01
**Priority:** P0 — everything else in this solution reads/writes these columns
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/migrations/0007_template_authoring_fields.up.sql`
**Depends on:** none
**Status:** `[x]` DONE — migration 0007 added (schema-qualified `workflow.templates`, spec's bare table name fixed); verified via `go test -tags=integration -run TestRepository_CreateAndGetTemplate ./internal/adapter/postgres/...` (real Postgres testcontainer, up applies cleanly with backfill)

---

## Context

`templates` is missing every column BUG-WF-01 needs: `description`,
`tags`, `owner_id`, `usage_count`, and the Inherit-mode merge columns
(`overrides`, `inject_steps`, `remove_steps`), plus a Clone-mode
provenance pointer. This is the additive migration that closes that gap.
`owner_id` is added nullable (not `NOT NULL`) per `05-data-architecture.md`'s
expand/contract rule — a follow-up `0008_template_owner_not_null.up.sql`
tightens it once every tenant's backfill is verified. Note: SOL-WF-03's
`0008_template_visibility_sharing` migration (TASK-WF-03-01) builds directly
on the columns this migration adds, so this one must land first.

## Changes to make

Create `backend-go/services/workflow-service/migrations/0007_template_authoring_fields.up.sql`:

```sql
ALTER TABLE templates
  ADD COLUMN description TEXT,
  ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}',
  ADD COLUMN owner_id UUID,                      -- backfilled below, then NOT NULL in a follow-up migration
  ADD COLUMN usage_count INT NOT NULL DEFAULT 0,
  ADD COLUMN overrides JSONB NOT NULL DEFAULT '{}',
  ADD COLUMN inject_steps JSONB NOT NULL DEFAULT '[]',
  ADD COLUMN remove_steps JSONB NOT NULL DEFAULT '[]',
  ADD COLUMN cloned_from_template_id UUID REFERENCES templates(id) ON DELETE SET NULL;

-- Backfill: no owner history exists for templates created before this
-- migration — the only safe backfill value is each row's own tenant,
-- treated as an implicit "system"-owned template. Real ownership for
-- pre-existing rows must be reconciled by a data-fix script per tenant
-- (out of scope here) — this migration only expands the shape.
UPDATE templates SET owner_id = tenant_id WHERE owner_id IS NULL;

CREATE INDEX idx_templates_tags ON templates USING GIN (tags);
CREATE INDEX idx_templates_owner ON templates(tenant_id, owner_id);
```

Create `backend-go/services/workflow-service/migrations/0007_template_authoring_fields.down.sql`:

```sql
DROP INDEX IF EXISTS idx_templates_owner;
DROP INDEX IF EXISTS idx_templates_tags;
ALTER TABLE templates
  DROP COLUMN cloned_from_template_id,
  DROP COLUMN remove_steps,
  DROP COLUMN inject_steps,
  DROP COLUMN overrides,
  DROP COLUMN usage_count,
  DROP COLUMN owner_id,
  DROP COLUMN tags,
  DROP COLUMN description;
```

## Verify

```bash
cd /opt/repos/orca/backend-go
# Run against whatever local/test Postgres this service's migration test
# harness uses (check existing 000N migration tests for the setup pattern).
migrate -path services/workflow-service/migrations -database "$WORKFLOW_SERVICE_TEST_DB_URL" up
migrate -path services/workflow-service/migrations -database "$WORKFLOW_SERVICE_TEST_DB_URL" down 1
migrate -path services/workflow-service/migrations -database "$WORKFLOW_SERVICE_TEST_DB_URL" up
```

Expected: `up` applies cleanly against a database with existing `templates`
rows (the `owner_id` backfill `UPDATE` must not error), `down` fully
reverses it, and a second `up` succeeds again (round-trip clean).
