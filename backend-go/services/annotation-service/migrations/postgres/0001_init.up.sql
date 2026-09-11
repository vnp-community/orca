-- annotation-service owns this database exclusively — no other service
-- reads or writes this table. See
-- specs/backend-go/architecture/05-data-architecture.md.
CREATE SCHEMA IF NOT EXISTS annotation;

-- One table, mirroring annotation-service.md §5 — this is deliberately the
-- simplest service in the catalog: one table, no cross-service writes.
CREATE TABLE annotation.annotations (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL,
    author_id    UUID NOT NULL, -- no local FK: the authenticated actor id
                                 -- isn't guaranteed to be a clean users-table
                                 -- row for every transport, see §5's rationale.
    repo_id      TEXT NOT NULL,
    file_path    TEXT NOT NULL,
    line         INTEGER NOT NULL CHECK (line >= 0),
    ref          TEXT NOT NULL DEFAULT '', -- commit sha/ref the anchor resolved against
    content      TEXT NOT NULL,
    resolved     BOOLEAN NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_annotations_tenant_id ON annotation.annotations (tenant_id);
CREATE INDEX idx_annotations_file_lookup ON annotation.annotations (tenant_id, repo_id, file_path, line);

-- Row-Level Security as defense-in-depth per architecture/05 — the
-- application layer's explicit tenant_id filtering (see internal/adapter/postgres)
-- is the primary enforcement; this is the secondary backstop.
ALTER TABLE annotation.annotations ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON annotation.annotations
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
