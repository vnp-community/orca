CREATE SCHEMA IF NOT EXISTS request;

CREATE TABLE request.outbox_events (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    subject TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    version INT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    published_at TIMESTAMPTZ,
    seq BIGINT GENERATED ALWAYS AS IDENTITY
);

CREATE UNIQUE INDEX idx_request_outbox_unpublished 
    ON request.outbox_events (created_at, seq) 
    WHERE published_at IS NULL;

CREATE TABLE request.processed_events (
    tenant_id UUID NOT NULL,
    event_id UUID NOT NULL,
    subject TEXT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, event_id)
);

CREATE INDEX idx_request_processed_events_at ON request.processed_events (processed_at);

DO $$ 
DECLARE
    t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY['outbox_events','processed_events'] LOOP
        EXECUTE 'ALTER TABLE request.' || t || ' ENABLE ROW LEVEL SECURITY';
        EXECUTE 'ALTER TABLE request.' || t || ' FORCE ROW LEVEL SECURITY';
        
        EXECUTE 'CREATE POLICY tenant_isolation ON request.' || t || '
            AS PERMISSIVE FOR ALL
            TO PUBLIC
            USING (tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')' || '::uuid' || ')
            WITH CHECK (tenant_id = NULLIF(current_setting(''app.tenant_id'', true), '''')' || '::uuid' || ')';
    END LOOP;
END $$;

CREATE POLICY relay_read ON request.outbox_events
    AS PERMISSIVE FOR SELECT
    TO PUBLIC
    USING (current_setting('app.relay', true) = 'on');

CREATE POLICY relay_mark_published ON request.outbox_events
    AS PERMISSIVE FOR UPDATE
    TO PUBLIC
    USING (current_setting('app.relay', true) = 'on')
    WITH CHECK (current_setting('app.relay', true) = 'on');
