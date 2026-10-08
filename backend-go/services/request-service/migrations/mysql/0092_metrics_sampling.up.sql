-- The gauge sampler counts stuck Requests across tenants; MySQL has no RLS, so only the index is needed.
CREATE INDEX idx_requests_stuck ON requests (status, updated_at);
