DROP POLICY IF EXISTS relay_scan ON request.request_counters;
DROP TABLE IF EXISTS request.tenant_security_settings;
DROP TABLE IF EXISTS request.request_webhook_nonces;
DROP TABLE IF EXISTS request.request_audit_outbox;
DROP TABLE IF EXISTS request.request_security_flags;
