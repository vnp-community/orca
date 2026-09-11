-- id has no server-side default here (unlike Postgres's
-- `DEFAULT gen_random_uuid()`): MySQL disallows non-deterministic
-- functions such as UUID() in a column DEFAULT expression (they're
-- rejected as "unsafe" — only deterministic expressions/literals are
-- allowed as of MySQL 8.0.13's expression-default support), so
-- internal/adapter/mysql.IssueListCacheRepository.Put generates the id in
-- Go (uuid.NewString()) before every INSERT ... ON DUPLICATE KEY UPDATE.
-- id is never read back by Get (only tenant_id/provider/repo/filter_hash
-- are), so this is a pure plumbing detail with no functional loss — same
-- reasoning BE-DB-SOL-001 §1 already established for usage-service's
-- sessions.id and annotation-service's annotations.id.
--
-- repo/filter_hash are TEXT (unbounded) in the Postgres variant; the
-- UNIQUE(tenant_id, provider, repo, filter_hash) constraint below needs
-- exact (not prefix-indexed) equality, so both become bounded VARCHAR/CHAR
-- instead of TEXT+prefix-index — same correctness reasoning as
-- migrations/mysql/0001_init.up.sql's webhook_delivery_log comment.
-- filter_hash is always a sha256 hex digest (usecase.filterHash /
-- internal/adapter/{postgres,mysql}'s own filterHash helper) — fixed 64
-- hex chars, so CHAR(64) exactly, never truncated. repo is a provider repo
-- slug ("owner/name") — VARCHAR(255) comfortably covers every real
-- provider's max login+repo-name length (e.g. GitHub: 39 + 100 chars).
CREATE TABLE issue_list_cache (
    id              CHAR(36) PRIMARY KEY,
    tenant_id       CHAR(36) NOT NULL,
    provider        VARCHAR(32) NOT NULL,
    repo            VARCHAR(255) NOT NULL,
    filter_hash     CHAR(64) NOT NULL,
    issues_json     JSON NOT NULL,
    cached_at       TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    expires_at      TIMESTAMP(6) NOT NULL,

    UNIQUE KEY uq_issue_list_cache (tenant_id, provider, repo, filter_hash)
);
CREATE INDEX idx_issue_list_cache_expires ON issue_list_cache (expires_at);

-- No Row-Level Security equivalent in MySQL/TiDB — same posture as
-- 0001_init.up.sql's tables:
--
--   ALTER TABLE scm.issue_list_cache ENABLE ROW LEVEL SECURITY;
--   CREATE POLICY tenant_isolation ON scm.issue_list_cache
--       USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
