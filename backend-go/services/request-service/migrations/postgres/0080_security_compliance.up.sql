-- CR-REQ-035: secret flag and erase marker (side table: the requests table belongs to other waves),
-- durable audit outbox, webhook replay nonces, retention settings.
CREATE TABLE request.request_security_flags (
    tenant_id UUID NOT NULL,
    request_id UUID NOT NULL,
    contains_secret_suspected BOOLEAN NOT NULL DEFAULT FALSE,
    erased_at TIMESTAMPTZ,
    erased_by UUID,
    PRIMARY KEY (tenant_id, request_id)
);

CREATE TABLE request.request_audit_outbox (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    audit_id UUID NOT NULL UNIQUE,
    action TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    actor_type TEXT NOT NULL CHECK (actor_type IN ('user','agent','system')),
    target_type TEXT NOT NULL,
    target_id TEXT NOT NULL,
    outcome TEXT NOT NULL CHECK (outcome IN ('allowed','denied')),
    ip_address TEXT NOT NULL DEFAULT '',
    metadata_json TEXT NOT NULL DEFAULT '{}' CHECK (octet_length(metadata_json) <= 4096),
    attempts INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at TIMESTAMPTZ
);

CREATE INDEX request_audit_outbox_pending ON request.request_audit_outbox (next_attempt_at) WHERE delivered_at IS NULL;
CREATE INDEX request_audit_outbox_tenant ON request.request_audit_outbox (tenant_id, created_at);

CREATE TABLE request.request_webhook_nonces (
    tenant_id UUID NOT NULL,
    source TEXT NOT NULL,
    nonce_hash CHAR(64) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (tenant_id, source, nonce_hash)
);

CREATE INDEX request_webhook_nonces_expiry ON request.request_webhook_nonces (expires_at);

-- 0 days keeps the data forever.
CREATE TABLE request.tenant_security_settings (
    tenant_id UUID PRIMARY KEY,
    request_retention_days INT NOT NULL DEFAULT 730 CHECK (request_retention_days >= 0),
    ai_trace_retention_days INT NOT NULL DEFAULT 30 CHECK (ai_trace_retention_days >= 0),
    ledger_retention_days INT NOT NULL DEFAULT 400 CHECK (ledger_retention_days >= 0),
    redact_pii_in_prompts BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

DO $$
DECLARE
    t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['request_security_flags', 'request_audit_outbox', 'request_webhook_nonces', 'tenant_security_settings'] LOOP
        EXECUTE format('ALTER TABLE request.%I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE request.%I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('CREATE POLICY tenant_isolation ON request.%I FOR ALL USING (tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')::uuid)', t);
    END LOOP;
END
$$;

-- Cross-tenant workers: the audit deliverer, the nonce pruner and the retention job's tenant listing.
CREATE POLICY audit_relay_read ON request.request_audit_outbox FOR SELECT
    USING (current_setting('app.relay', true) = 'on');
CREATE POLICY audit_relay_update ON request.request_audit_outbox FOR UPDATE
    USING (current_setting('app.relay', true) = 'on')
    WITH CHECK (current_setting('app.relay', true) = 'on');
CREATE POLICY audit_relay_prune ON request.request_audit_outbox FOR DELETE
    USING (current_setting('app.relay', true) = 'on' AND delivered_at IS NOT NULL);
CREATE POLICY nonce_relay_scan ON request.request_webhook_nonces FOR SELECT
    USING (current_setting('app.relay', true) = 'on' AND expires_at < now());
CREATE POLICY nonce_relay_prune ON request.request_webhook_nonces FOR DELETE
    USING (current_setting('app.relay', true) = 'on' AND expires_at < now());
CREATE POLICY relay_scan ON request.request_counters FOR SELECT
    USING (current_setting('app.relay', true) = 'on');
