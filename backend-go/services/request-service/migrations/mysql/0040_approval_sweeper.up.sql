-- Expire/remind sweepers scan pending approvals across tenants; MySQL has no RLS, so only the index is needed.
CREATE INDEX approvals_sweep ON approvals (status, due_at);
