-- annotation-service owns this database exclusively — no other service
-- reads or writes this table. MySQL/TiDB variant: no CREATE SCHEMA (a
-- MySQL database IS the schema-equivalent isolation unit — this migration
-- assumes DATABASE_DSN already points at a database named `annotation`,
-- mirroring the Postgres variant's `annotation` schema name; see
-- specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-002.md's
-- usage-service precedent for this naming convention).
--
-- id has no server-side default here (unlike Postgres's
-- `DEFAULT gen_random_uuid()`): internal/usecase/create_annotation.go
-- always generates the id in Go (uuid.NewString()) before calling
-- CreateAnnotation, so the Postgres default was already dead code from the
-- application's point of view — confirmed the same way BE-DB-SOL-001 §1
-- confirmed it for usage-service's sessions.id. No functional loss.
CREATE TABLE annotations (
    id           CHAR(36) PRIMARY KEY,
    tenant_id    CHAR(36) NOT NULL,
    author_id    CHAR(36) NOT NULL, -- no local FK: the authenticated actor id
                                     -- isn't guaranteed to be a clean users-table
                                     -- row for every transport, see postgres
                                     -- variant's comment.
    repo_id      TEXT NOT NULL,
    file_path    TEXT NOT NULL,
    line         INTEGER NOT NULL,
    ref          VARCHAR(255) NOT NULL DEFAULT '', -- commit sha/ref the anchor resolved against
    content      TEXT NOT NULL,
    resolved     BOOLEAN NOT NULL DEFAULT false,
    created_at   TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at   TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    CONSTRAINT annotations_line_check CHECK (line >= 0)
);

CREATE INDEX idx_annotations_tenant_id ON annotations (tenant_id);
-- MySQL rejects an index key over 3072 bytes (InnoDB, utf8mb4) — repo_id
-- and file_path are TEXT (unbounded), so this composite index needs
-- explicit prefix lengths on both, unlike the Postgres variant's
-- unbounded TEXT index. 255 chars covers virtually every real repo
-- identifier/file path while staying well under the limit alongside
-- tenant_id/line in the same key (144 + 1020 + 1020 + 4 bytes ≈ 2188,
-- under the 3072-byte InnoDB key-part limit).
CREATE INDEX idx_annotations_file_lookup ON annotations (tenant_id, repo_id(255), file_path(255), line);

-- No Row-Level Security equivalent in MySQL/TiDB — application-layer
-- tenant_id scoping in internal/adapter/mysql/repository.go is the ONLY
-- enforcement mechanism here, not a secondary backstop. See
-- TASK-BE-DB-003's finding (mirrored for this service, not re-audited
-- from scratch — same codebase-wide fact): no Go code anywhere calls
-- SET LOCAL app.tenant_id, so the Postgres RLS policy below never
-- actually activated either. This migration doesn't regress anything, it
-- just stops declaring a backstop that never ran:
--
--   ALTER TABLE annotation.annotations ENABLE ROW LEVEL SECURITY;
--   CREATE POLICY tenant_isolation ON annotation.annotations
--       USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
