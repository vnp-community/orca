-- MySQL translation of migrations/postgres/0004_grant_expiry_and_public_link.up.sql.
ALTER TABLE task_grants
  ADD COLUMN expires_at TIMESTAMP(6) NULL; -- NULL = never expires

-- Postgres's idx_task_grants_expires is a PARTIAL index (WHERE expires_at
-- IS NOT NULL) — full index instead, same reasoning as migrations/mysql/0002.
CREATE INDEX idx_task_grants_expires ON task_grants (expires_at);

-- Public/anonymous share-link flow. token_hash is SHA-256 hex — always
-- exactly 64 characters — so VARCHAR(64) is an exact-fit bounded type, not
-- a lossy prefix-index workaround: InnoDB needs a bounded length for a
-- UNIQUE key, and 64 is the real, fixed length of every value this column
-- ever holds (internal/usecase.CreatePublicLink hashes with sha256, per
-- that usecase's doc comment).
CREATE TABLE task_share_links (
    id          CHAR(36) NOT NULL PRIMARY KEY DEFAULT (UUID()),
    tenant_id   CHAR(36) NOT NULL,
    task_id     CHAR(36) NOT NULL,
    token_hash  VARCHAR(64) NOT NULL,
    created_by  CHAR(36) NOT NULL,
    level       VARCHAR(10) NOT NULL DEFAULT 'user',
    expires_at  TIMESTAMP(6) NULL,
    revoked_at  TIMESTAMP(6) NULL,
    created_at  TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT task_share_links_level_check CHECK (level = 'user'),
    CONSTRAINT task_share_links_token_hash_unique UNIQUE (token_hash),
    CONSTRAINT fk_task_share_links_task FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
) ENGINE=InnoDB;

-- Postgres's idx_task_share_links_task is a PARTIAL index (WHERE
-- revoked_at IS NULL) — full index instead, same reasoning as
-- migrations/mysql/0002.
CREATE INDEX idx_task_share_links_task ON task_share_links (tenant_id, task_id);
