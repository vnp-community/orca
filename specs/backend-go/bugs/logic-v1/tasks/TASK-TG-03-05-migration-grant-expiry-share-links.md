# TASK-TG-03-05: Migration `0004` — `task_grants.expires_at`, `task_share_links` table

**From Solution:** SOL-TG-03
**Priority:** P1
**Service:** `task-service`
**File:** `backend-go/services/task-service/migrations/0004_grant_expiry_and_public_link.up.sql` (new)
**Depends on:** TASK-TG-01-02 (this is migration `0004`, sequential after TG-01's `0003`)
**Status:** `[ ]` TODO

---

## Context

`task.task_grants` already has an `id UUID PRIMARY KEY`
(`migrations/0001_init.up.sql:68`) but no `expires_at`. Public/anonymous
share-link access is a new table entirely — one row per active link,
revocation is a soft-delete (`revoked_at` set) so an audit trail survives,
matching the append-only posture `07-security-architecture.md`'s
audit section requires for security-relevant state changes generally.

## Changes to make

Create `backend-go/services/task-service/migrations/0004_grant_expiry_and_public_link.up.sql`:

```sql
ALTER TABLE task.task_grants
  ADD COLUMN expires_at TIMESTAMPTZ; -- NULL = never expires

CREATE INDEX idx_task_grants_expires ON task.task_grants (expires_at) WHERE expires_at IS NOT NULL;

-- Public/anonymous share-link flow. One row per active link; revocation is
-- a soft-delete (revoked_at set) so an audit trail survives.
CREATE TABLE task.task_share_links (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL,
    task_id     UUID NOT NULL REFERENCES task.tasks(id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL UNIQUE, -- SHA-256 of the random token, never the plaintext
    created_by  UUID NOT NULL,
    level       TEXT NOT NULL DEFAULT 'user' CHECK (level = 'user'), -- anonymous access is always read-only
    expires_at  TIMESTAMPTZ,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_task_share_links_task ON task.task_share_links (tenant_id, task_id) WHERE revoked_at IS NULL;

ALTER TABLE task.task_share_links ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON task.task_share_links
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
```

Create `backend-go/services/task-service/migrations/0004_grant_expiry_and_public_link.down.sql`:

```sql
DROP TABLE IF EXISTS task.task_share_links;

DROP INDEX IF EXISTS task.idx_task_grants_expires;
ALTER TABLE task.task_grants DROP COLUMN expires_at;
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/task-service
migrate -database "$DATABASE_DSN" -path migrations up
migrate -database "$DATABASE_DSN" -path migrations down 1
migrate -database "$DATABASE_DSN" -path migrations up
```

Expected: all three steps succeed; `\d task.task_grants` shows `expires_at`;
`\d task.task_share_links` shows the new table with RLS enabled after the
final `up`.
