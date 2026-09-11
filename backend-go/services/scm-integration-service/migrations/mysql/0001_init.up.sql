-- scm-integration-service owns this database exclusively — no other
-- service reads or writes these tables. MySQL/TiDB variant: no CREATE
-- SCHEMA (a MySQL database IS the schema-equivalent isolation unit — this
-- migration assumes DATABASE_DSN already points at a database named `scm`,
-- mirroring the Postgres variant's `scm` schema name; see
-- specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-002.md's
-- usage-service precedent for this naming convention). Per this service's
-- own doc (§5), this schema holds operational bookkeeping ONLY — never a
-- copy, cache, or mirror of provider issue/PR/MR/comment data.

-- rate_limit_cache: last-known rate-limit snapshot per (tenant_id,
-- provider, bucket) — see the Postgres variant's comment for the full
-- rationale (unchanged by dialect). `limit` is a MySQL reserved word,
-- backtick-quoted throughout (mirrors the Postgres variant's own
-- double-quoting for the same reason).
CREATE TABLE rate_limit_cache (
    tenant_id       CHAR(36) NOT NULL,
    provider        VARCHAR(32) NOT NULL,
    bucket          VARCHAR(32) NOT NULL DEFAULT 'core',
    remaining       INT NOT NULL,
    `limit`         INT NOT NULL,
    reset_at        TIMESTAMP(6) NOT NULL,
    last_checked_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    CONSTRAINT rate_limit_cache_provider_check CHECK (provider IN ('github', 'gitlab', 'bitbucket', 'azure_devops', 'gitea')),
    PRIMARY KEY (tenant_id, provider, bucket)
);

-- No Row-Level Security equivalent in MySQL/TiDB — application-layer
-- tenant_id scoping in internal/adapter/mysql is the ONLY tenant-isolation
-- enforcement for this adapter (see TASK-BE-DB-003's finding, true here for
-- the same reason it was true for usage-service/annotation-service: no Go
-- code anywhere calls SET LOCAL app.tenant_id, so the Postgres adapter's
-- RLS policy below never actually activated either — this migration
-- doesn't regress anything, it just stops declaring a backstop that never
-- ran):
--
--   ALTER TABLE scm.rate_limit_cache ENABLE ROW LEVEL SECURITY;
--   CREATE POLICY tenant_isolation ON scm.rate_limit_cache
--       USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- webhook_delivery_log: append-only record of inbound webhook deliveries
-- processed — see the Postgres variant's comment for the full rationale.
-- delivery_id/event_id are TEXT (unbounded) in the Postgres variant;
-- here they're VARCHAR(255) instead of TEXT+prefix-index (unlike
-- annotation-service's repo_id/file_path precedent, which only needed a
-- prefix index for a plain WHERE-scan index): this table's idempotency
-- guarantee lives in the UNIQUE(provider, delivery_id) constraint below, and
-- a MySQL/InnoDB prefix index (e.g. delivery_id(255)) only de-duplicates on
-- the first 255 bytes, not the full value — silently wrong for
-- correctness, not just slower, if two distinct delivery_id values ever
-- shared a 255-byte prefix. VARCHAR(255) keeps the constraint exact
-- (provider-assigned delivery ids are short, e.g. GitHub's X-GitHub-Delivery
-- is a UUID) while still being directly indexable.
CREATE TABLE webhook_delivery_log (
    id           CHAR(36) PRIMARY KEY,
    tenant_id    CHAR(36) NOT NULL,
    provider     VARCHAR(32) NOT NULL,
    delivery_id  VARCHAR(255) NOT NULL,
    event_id     VARCHAR(255) NOT NULL DEFAULT '',
    received_at  TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    outcome      VARCHAR(16) NOT NULL,

    CONSTRAINT webhook_delivery_log_provider_check CHECK (provider IN ('github', 'gitlab', 'bitbucket', 'azure_devops', 'gitea')),
    CONSTRAINT webhook_delivery_log_outcome_check CHECK (outcome IN ('processed', 'ignored', 'failed')),
    UNIQUE KEY uq_webhook_delivery_log_provider_delivery (provider, delivery_id)
);

CREATE INDEX idx_scm_webhook_delivery_log_tenant_received
    ON webhook_delivery_log (tenant_id, received_at DESC);

-- No Row-Level Security equivalent in MySQL/TiDB — same posture as
-- rate_limit_cache above:
--
--   ALTER TABLE scm.webhook_delivery_log ENABLE ROW LEVEL SECURITY;
--   CREATE POLICY tenant_isolation ON scm.webhook_delivery_log
--       USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
