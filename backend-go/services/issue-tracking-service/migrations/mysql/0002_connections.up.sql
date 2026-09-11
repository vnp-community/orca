-- MySQL/TiDB variant of postgres/0002_connections.up.sql.
--
-- `id`'s Postgres default (`gen_random_uuid()`) is never read back by Go
-- code — internal/adapter/postgres/connections.go's INSERT column list
-- omits `id` entirely and no query in that file ever selects it (confirmed
-- by reading the whole file, not assumed) — so the MySQL equivalent only
-- needs SOME unique PK value generated server-side, not a Go-generated one.
-- `DEFAULT (UUID())` (MySQL 8.0.13+; the mysql:8 test/prod image satisfies
-- this) keeps the column UUID-shaped like the Postgres variant instead of
-- switching to AUTO_INCREMENT, which would be a bigger type change than
-- this rollout needs.
--
-- tenant_id/user_id/provider/external_workspace_id are bounded VARCHAR, not
-- TEXT: they're part of a composite UNIQUE KEY below, and InnoDB rejects
-- BLOB/TEXT columns in a key without an explicit prefix length. Bounds
-- chosen so the composite key (64+64+16+255 chars * 4 bytes/char utf8mb4 =
-- 1596 bytes) stays under InnoDB's 3072-byte max index key length.
CREATE TABLE IF NOT EXISTS connections (
    id                    CHAR(36) PRIMARY KEY DEFAULT (UUID()),
    tenant_id             VARCHAR(64) NOT NULL,
    user_id               VARCHAR(64) NOT NULL,
    provider              VARCHAR(16) NOT NULL,
    external_workspace_id VARCHAR(255) NOT NULL,
    workspace_name        VARCHAR(255) NOT NULL DEFAULT '',
    workspace_url         VARCHAR(2048) NOT NULL DEFAULT '',
    viewer_id             VARCHAR(255) NOT NULL DEFAULT '',
    viewer_display_name   VARCHAR(255) NOT NULL DEFAULT '',
    viewer_email          VARCHAR(255) NOT NULL DEFAULT '',
    credential_id         CHAR(36) NOT NULL,
    is_selected           BOOLEAN NOT NULL DEFAULT TRUE,
    -- No `ON UPDATE CURRENT_TIMESTAMP(6)` on updated_at: the adapter sets
    -- it explicitly (NOW(6) in every write), same explicit-control posture
    -- the Postgres adapter already has (`updated_at = now()` in its own
    -- UPDATE SETs) — avoids MySQL's implicit-first-TIMESTAMP-auto-update
    -- surprise silently doing the same job a second, divergent way.
    created_at            TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at            TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT issuetracking_connections_site_key
        UNIQUE (tenant_id, user_id, provider, external_workspace_id)
);

CREATE INDEX idx_connections_lookup ON connections (tenant_id, user_id, provider);

-- No RLS equivalent — see 0001_outbox.up.sql's comment; same posture here.
