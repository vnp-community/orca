-- Deviation from the Postgres source (documented, not silent): `path` is
-- TEXT there (unbounded) but participates in a UNIQUE constraint here.
-- MySQL/InnoDB cannot uniquely index an unbounded TEXT column without a
-- prefix length, and a prefix would only guarantee uniqueness of the first
-- N bytes — too weak for a path-collision guard.
--
-- VARCHAR(512), not VARCHAR(1024): confirmed LIVE via a real "Error 1071:
-- Specified key was too long; max key length is 3072 bytes" running this
-- migration against MySQL 8 (utf8mb4, 4 bytes/char) — VARCHAR(1024) alone
-- is already 4096 bytes, over the limit before even adding
-- tenant_id/dev_server_id. Budget: CHAR(36) x2 = 288 bytes, leaving
-- 3072-288=2784 bytes = 696 utf8mb4 chars for `path`; VARCHAR(512) (2048
-- bytes) keeps a comfortable margin below that while still covering any
-- realistic filesystem path (Linux PATH_MAX is 4096 BYTES, not chars, and
-- typical repo-relative paths are far shorter in practice) — see
-- BE-DB-SOL-016 §5/§8 for the full writeup of this tradeoff, including the
-- test failure that caught the original VARCHAR(1024) sizing mistake.
CREATE TABLE folder_workspaces (
    id            CHAR(36) PRIMARY KEY,
    tenant_id     CHAR(36) NOT NULL,
    dev_server_id CHAR(36) NOT NULL,
    path          VARCHAR(512) NOT NULL,
    name          TEXT NOT NULL,
    added_by      CHAR(36) NOT NULL,
    created_at    DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    UNIQUE (tenant_id, dev_server_id, path)
);
CREATE INDEX idx_folder_workspaces_tenant ON folder_workspaces (tenant_id);

-- RLS dropped — see 0001_init.up.sql's comment.
