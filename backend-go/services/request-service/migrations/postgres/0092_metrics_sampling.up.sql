-- The gauge sampler counts stuck Requests across tenants (outbox-style relay access, read only), like the approval sweeper does.
CREATE POLICY relay_metrics_scan ON request.requests
    AS PERMISSIVE FOR SELECT
    USING (current_setting('app.relay', true) = 'on');

CREATE INDEX idx_requests_stuck ON request.requests (status, updated_at)
    WHERE status IN ('classifying', 'analyzing', 'planning', 'executing');
