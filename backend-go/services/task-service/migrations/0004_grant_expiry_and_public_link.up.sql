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
