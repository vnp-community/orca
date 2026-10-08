-- Append-only measurements the type policies judge on completion; the newest row per (request, kind) is effective.
-- metrics has no DEFAULT: MySQL JSON defaults need 8.0.13 expressions, so the application always sends '{}'.
CREATE TABLE request_checks (
    id CHAR(36) PRIMARY KEY,
    tenant_id CHAR(36) NOT NULL,
    request_id CHAR(36) NOT NULL,
    kind VARCHAR(30) NOT NULL CHECK (kind IN ('perf_baseline','perf_after','tests_before','tests_after','security_recheck','ops_result')),
    status VARCHAR(10) NOT NULL CHECK (status IN ('passed','failed')),
    metrics JSON NOT NULL,
    summary TEXT NOT NULL,
    -- orca_verified is reserved for checks Orca measured itself; no RPC may set it.
    source VARCHAR(14) NOT NULL CHECK (source IN ('agent','manual','orca_verified')),
    task_id CHAR(36) NULL,
    recorded_by CHAR(36) NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    INDEX idx_request_checks_latest (tenant_id, request_id, kind, created_at DESC)
) ENGINE=InnoDB;
