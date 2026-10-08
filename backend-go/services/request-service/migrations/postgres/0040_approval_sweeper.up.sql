-- Expire/remind sweepers scan pending approvals across tenants (outbox-style relay access, read only).
CREATE INDEX approvals_sweep ON request.approvals (status, due_at) WHERE status = 'pending';

CREATE POLICY relay_scan ON request.approvals
    AS PERMISSIVE FOR SELECT
    USING (current_setting('app.relay', true) = 'on');
