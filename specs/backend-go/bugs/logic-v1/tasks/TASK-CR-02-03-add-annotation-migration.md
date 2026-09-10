# TASK-CR-02-03: Add migration for `worktree_id`/`end_line`/`side`/`original_code`/`sent_to_agent`/`sent_at`

**From Solution:** SOL-CR-02
**Priority:** P0
**Service:** `annotation-service`
**File:** `backend-go/services/annotation-service/migrations/0003_annotation_side_range_sent.up.sql` (new), `0003_annotation_side_range_sent.down.sql` (new)
**Depends on:** none
**Status:** `[x]` DONE — 0003_annotation_side_range_sent.up/down.sql created matching spec exactly; syntax verified by inspection (no local Postgres in this environment to run migrate up/down/up), postgres integration test file (repository_test.go, //go:build integration) compiles clean against the new schema

---

## Context

The existing migrations are `0001_init` and `0002_annotation_request_id`
(not `0002_annotation_side_range_sent` as SOL-CR-02's sketch names it —
`0002` is already taken) — this is migration `0003`. All new columns are
`NOT NULL DEFAULT`-backed except the nullable `worktree_id`/`sent_at`, so
this is a pure additive `ALTER TABLE`, no backfill needed.

## Changes to make

`backend-go/services/annotation-service/migrations/0003_annotation_side_range_sent.up.sql`:

```sql
-- Adds diff-side, multi-line range, code snapshot, worktree scoping, and
-- sent-to-agent state to annotations — see BUG-CR-02 and SOL-CR-02.
-- All columns are additive with safe defaults; existing rows get
-- side=SIDE_UNSPECIFIED (0), end_line=0 (single-line), sent_to_agent=false
-- — "this data didn't exist for old rows", not a fabricated value.
ALTER TABLE annotation.annotations
    ADD COLUMN worktree_id   TEXT,
    ADD COLUMN end_line      INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN side          SMALLINT NOT NULL DEFAULT 0, -- 0=unspecified,1=old,2=new
    ADD COLUMN original_code TEXT NOT NULL DEFAULT '',
    ADD COLUMN sent_to_agent BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN sent_at       TIMESTAMPTZ;

CREATE INDEX idx_annotations_worktree ON annotation.annotations (tenant_id, worktree_id)
    WHERE worktree_id IS NOT NULL;
```

`backend-go/services/annotation-service/migrations/0003_annotation_side_range_sent.down.sql`:

```sql
DROP INDEX IF EXISTS annotation.idx_annotations_worktree;

ALTER TABLE annotation.annotations
    DROP COLUMN worktree_id,
    DROP COLUMN end_line,
    DROP COLUMN side,
    DROP COLUMN original_code,
    DROP COLUMN sent_to_agent,
    DROP COLUMN sent_at;
```

Note the index is scoped `(tenant_id, worktree_id)`, not
`(project_id_repo_id, worktree_id)` as SOL-CR-02's sketch has it — this
table has no `project_id_repo_id` column (it has `tenant_id`/`repo_id`
separately, per `0001_init.up.sql`); `tenant_id` is the correct leading
column for this service's tenant-isolation convention (see
`0001_init.up.sql`'s `idx_annotations_tenant_id`).

## Verify

```bash
cd /opt/repos/orca/backend-go/services/annotation-service
# Requires a local Postgres reachable per this service's migration tooling
# (see this service's README for the migrate invocation it uses).
migrate -path migrations -database "$ANNOTATION_SERVICE_DB_URL" up
migrate -path migrations -database "$ANNOTATION_SERVICE_DB_URL" down 1
migrate -path migrations -database "$ANNOTATION_SERVICE_DB_URL" up
```

Expected: both `up` and `down` apply cleanly with no errors, `up` is
idempotent-safe to re-run after a `down 1`.
