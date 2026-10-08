-- request_flow_enabled defaults to false: a tenant must be switched on explicitly (CR-REQ-025).
CREATE TABLE tenant_settings (
    tenant_id CHAR(36) PRIMARY KEY,
    request_flow_enabled TINYINT(1) NOT NULL DEFAULT 0,
    updated_by VARCHAR(255) NOT NULL DEFAULT '',
    updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
) ENGINE=InnoDB;
