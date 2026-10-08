-- CR-REQ-035 (MySQL has no RLS: repositories scope every query by tenant_id, checked by tenant_scope_test).
CREATE TABLE request_security_flags (
    tenant_id CHAR(36) NOT NULL,
    request_id CHAR(36) NOT NULL,
    contains_secret_suspected TINYINT(1) NOT NULL DEFAULT 0,
    erased_at TIMESTAMP(6) NULL,
    erased_by CHAR(36) NULL,
    PRIMARY KEY (tenant_id, request_id)
) ENGINE=InnoDB;

CREATE TABLE request_audit_outbox (
    id CHAR(36) PRIMARY KEY,
    tenant_id CHAR(36) NOT NULL,
    audit_id CHAR(36) NOT NULL,
    action VARCHAR(100) NOT NULL,
    actor_id VARCHAR(100) NOT NULL,
    actor_type VARCHAR(10) NOT NULL CHECK (actor_type IN ('user','agent','system')),
    target_type VARCHAR(50) NOT NULL,
    target_id VARCHAR(100) NOT NULL,
    outcome VARCHAR(10) NOT NULL CHECK (outcome IN ('allowed','denied')),
    ip_address VARCHAR(64) NOT NULL DEFAULT '',
    metadata_json TEXT NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    delivered_at TIMESTAMP(6) NULL,
    UNIQUE KEY request_audit_outbox_audit_id (audit_id),
    INDEX request_audit_outbox_pending (delivered_at, next_attempt_at),
    INDEX request_audit_outbox_tenant (tenant_id, created_at)
) ENGINE=InnoDB;

CREATE TABLE request_webhook_nonces (
    tenant_id CHAR(36) NOT NULL,
    source VARCHAR(100) NOT NULL,
    nonce_hash CHAR(64) NOT NULL,
    expires_at TIMESTAMP(6) NOT NULL,
    PRIMARY KEY (tenant_id, source, nonce_hash),
    INDEX request_webhook_nonces_expiry (expires_at)
) ENGINE=InnoDB;

-- 0 days keeps the data forever.
CREATE TABLE tenant_security_settings (
    tenant_id CHAR(36) PRIMARY KEY,
    request_retention_days INT NOT NULL DEFAULT 730 CHECK (request_retention_days >= 0),
    ai_trace_retention_days INT NOT NULL DEFAULT 30 CHECK (ai_trace_retention_days >= 0),
    ledger_retention_days INT NOT NULL DEFAULT 400 CHECK (ledger_retention_days >= 0),
    redact_pii_in_prompts TINYINT(1) NOT NULL DEFAULT 0,
    updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
) ENGINE=InnoDB;
