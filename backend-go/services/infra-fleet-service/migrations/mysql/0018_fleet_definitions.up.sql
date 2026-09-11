CREATE TABLE fleet_definitions (
    id CHAR(36) PRIMARY KEY,
    tenant_id CHAR(36) NOT NULL,
    -- VARCHAR(255), not Postgres's unbounded TEXT — this column sits inside
    -- a plain UNIQUE table constraint below, and MySQL/InnoDB needs a fixed
    -- indexable width for that (no unbounded-TEXT index support); 255 is a
    -- generous bound for a human-assigned definition name.
    name VARCHAR(255) NOT NULL,
    version INT NOT NULL DEFAULT 1,
    servers JSON NOT NULL,
    provision JSON,
    created_by CHAR(36) NOT NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    UNIQUE (tenant_id, name)
);
