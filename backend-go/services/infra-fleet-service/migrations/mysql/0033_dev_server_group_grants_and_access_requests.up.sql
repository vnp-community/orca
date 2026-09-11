-- CR-DS-007 (docs/crs/v2/dev-server/CR-DS-007-department-based-access-control.md)
-- and CR-DS-008 (docs/crs/v2/dev-server/CR-DS-008-first-login-department-gate-and-access-request.md).

CREATE TABLE dev_server_group_grants (
    id                    CHAR(36) PRIMARY KEY,
    tenant_id             CHAR(36) NOT NULL,
    dev_server_group_id   CHAR(36) NOT NULL REFERENCES dev_server_groups(id),
    -- VARCHAR(16), not Postgres's unbounded TEXT — a short fixed enum
    -- ('department'|'team') and part of the UNIQUE constraint below.
    grantee_kind          VARCHAR(16) NOT NULL CHECK (grantee_kind IN ('department', 'team')),
    -- grantee_id is a logical FK into tenant-service's departments/teams —
    -- a different service's database, so no physical FK (see
    -- CR-DS-007 §2's "resolve at the edge" note). VARCHAR(255), not
    -- unbounded TEXT, for the same UNIQUE-constraint reason as
    -- grantee_kind — see migrations/mysql/0022's pane_key comment for why
    -- a prefix-length index isn't used here instead (would only guarantee
    -- uniqueness of the prefix, not the full value).
    grantee_id            VARCHAR(255) NOT NULL,
    created_at            TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    UNIQUE (dev_server_group_id, grantee_kind, grantee_id)
);

CREATE INDEX idx_infra_dev_server_group_grants_tenant ON dev_server_group_grants (tenant_id);
CREATE INDEX idx_infra_dev_server_group_grants_group ON dev_server_group_grants (dev_server_group_id);

-- No RLS equivalent — see migrations/mysql/0001_init.up.sql's comment.

CREATE TABLE dev_server_access_requests (
    id                    CHAR(36) PRIMARY KEY,
    tenant_id             CHAR(36) NOT NULL,
    user_id               CHAR(36) NOT NULL,
    dev_server_group_id   CHAR(36) NOT NULL REFERENCES dev_server_groups(id),
    -- VARCHAR, not TEXT, for this CHECK-constrained enum column — see
    -- migrations/mysql/0013_ephemeral_vm_runtimes.up.sql's comment.
    status                VARCHAR(16) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'rejected')),
    -- MySQL 8.0.13+ requires TEXT column defaults to be a parenthesized
    -- expression — see migrations/mysql/0002_connections.up.sql's comment.
    message               TEXT NOT NULL DEFAULT (''),
    grantee_kind          VARCHAR(16) NOT NULL CHECK (grantee_kind IN ('department', 'team')),
    grantee_id            VARCHAR(255) NOT NULL,
    created_at            TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    resolved_at           TIMESTAMP(6),
    resolved_by           CHAR(36)
);

CREATE INDEX idx_infra_dev_server_access_requests_tenant ON dev_server_access_requests (tenant_id, status);

-- No RLS equivalent — see migrations/mysql/0001_init.up.sql's comment.
