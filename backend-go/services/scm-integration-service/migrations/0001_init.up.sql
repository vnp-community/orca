-- scm-integration-service owns this database exclusively — no other
-- service reads or writes these tables. See
-- specs/backend-go/architecture/05-data-architecture.md. Per this service's
-- own doc (§5), this schema holds operational bookkeeping ONLY — never a
-- copy, cache, or mirror of provider issue/PR/MR/comment data; every read
-- of that data hits the provider's live API on every call.
CREATE SCHEMA IF NOT EXISTS scm;

-- rate_limit_cache: last-known rate-limit snapshot per (tenant_id,
-- provider, bucket) — populated from a provider response's rate-limit
-- headers/body on every real adapter call (GitHub's X-RateLimit-* /
-- /rate_limit body, GitLab's RateLimit-* headers, etc.), read BEFORE
-- dispatching a burst of new calls to decide whether to back off (§8). Not
-- a source of truth for anything — a hot local read to avoid a round trip
-- purely to check quota. "bucket" exists because GitHub alone exposes
-- separate core/graphql/search buckets per token; every other provider
-- this service supports has exactly one bucket, always named "core" here.
CREATE TABLE scm.rate_limit_cache (
    tenant_id       UUID NOT NULL,
    provider        TEXT NOT NULL CHECK (provider IN ('github', 'gitlab', 'bitbucket', 'azure_devops', 'gitea')),
    bucket          TEXT NOT NULL DEFAULT 'core',
    remaining       INT NOT NULL,
    "limit"         INT NOT NULL,
    reset_at        TIMESTAMPTZ NOT NULL,
    last_checked_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (tenant_id, provider, bucket)
);

ALTER TABLE scm.rate_limit_cache ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON scm.rate_limit_cache
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- webhook_delivery_log: append-only record of inbound webhook deliveries
-- processed (event id, provider, delivery id, received_at, outcome) —
-- makes delivery idempotent against provider retries and gives operators a
-- debugging trail (§5). First writer: usecase.ReceiveWebhook /
-- internal/adapter/postgres.WebhookDeliveryRepository (TASK-PI-03-06,
-- BUG-PI-03) — tenant_id is written as a placeholder system UUID until
-- this service can resolve a real per-webhook tenant_id (see that
-- repository's doc comment).
CREATE TABLE scm.webhook_delivery_log (
    id           UUID PRIMARY KEY,
    tenant_id    UUID NOT NULL,
    provider     TEXT NOT NULL CHECK (provider IN ('github', 'gitlab', 'bitbucket', 'azure_devops', 'gitea')),
    delivery_id  TEXT NOT NULL, -- provider-assigned delivery id, e.g. GitHub's X-GitHub-Delivery
    event_id     TEXT NOT NULL DEFAULT '',
    received_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    outcome      TEXT NOT NULL CHECK (outcome IN ('processed', 'ignored', 'failed')),

    UNIQUE (provider, delivery_id) -- idempotency against provider retries, per this table's own doc above
);

CREATE INDEX idx_scm_webhook_delivery_log_tenant_received
    ON scm.webhook_delivery_log (tenant_id, received_at DESC);

ALTER TABLE scm.webhook_delivery_log ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON scm.webhook_delivery_log
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
