-- LookupRequestBySource may omit the site (PR events carry none), so the lookup cannot use idx_requests_source.
CREATE INDEX idx_requests_source_ref ON request.requests (tenant_id, source_provider, source_ref);
