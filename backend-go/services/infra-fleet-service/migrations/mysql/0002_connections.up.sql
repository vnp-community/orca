-- connections replaces the 0001-era simplification where
-- ResolveConnection equated connectionId with dev_server.id directly (see
-- postgres.Repository.ResolveConnection's prior doc comment). A connection
-- is now its own entity: which dev server owns it, plus the per-connection
-- metadata (repo_path, worktree_id) callers like git-gateway-service's
-- RelayExecutor need alongside the resolved DevServer. See
-- docs/execution-plan.md §2 Epic A's second pass and
-- specs/backend-go/services/infra-fleet-service.md §5.
CREATE TABLE connections (
    id                  CHAR(36) PRIMARY KEY,
    tenant_id           CHAR(36) NOT NULL,
    dev_server_id       CHAR(36) NOT NULL REFERENCES dev_servers(id),
    -- MySQL 8.0.13+ requires BLOB/TEXT/JSON column defaults to be a
    -- parenthesized expression, not a plain literal like Postgres accepts —
    -- confirmed the hard way (real mysql:8, "Error 1101") before writing
    -- this comment. Same fix applies everywhere else in this rollout a TEXT
    -- column carries a DEFAULT.
    repo_path           TEXT NOT NULL DEFAULT (''),
    worktree_id         TEXT NOT NULL DEFAULT (''),
    created_at          TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
);

CREATE INDEX idx_infra_connections_tenant ON connections (tenant_id, created_at DESC);
CREATE INDEX idx_infra_connections_dev_server ON connections (dev_server_id);
-- A worktree is bound to at most one live connection at a time — empty
-- worktree_id (connections not yet tied to a worktree) is excluded from the
-- uniqueness constraint rather than colliding on ''. MySQL/TiDB has no
-- partial index (unique or not) — the Postgres original's
-- `WHERE worktree_id <> ''` is translated with the standard MySQL
-- workaround already established at ai-provider-service/migrations/mysql/
-- 0003_account_registration_fields.up.sql: a generated column that
-- evaluates to the uniqueness key only when the partial condition holds,
-- NULL otherwise, then a plain UNIQUE index on that column — MySQL treats
-- multiple NULLs in a UNIQUE index as distinct, same as Postgres.
ALTER TABLE connections ADD COLUMN worktree_uniq_key VARCHAR(600) GENERATED ALWAYS AS (
    CASE WHEN worktree_id <> '' THEN CONCAT(tenant_id, '|', worktree_id) ELSE NULL END
) STORED;
CREATE UNIQUE INDEX idx_infra_connections_tenant_worktree ON connections (worktree_uniq_key);

-- No RLS equivalent — see migrations/mysql/0001_init.up.sql's comment,
-- same rationale (no backend-go code ever activates RLS on Postgres either).

-- port_forwards / provider_registry_entries: schema only, per
-- the design doc's fuller sketch (specs/backend-go/services/infra-fleet-service.md
-- §5) — no usecase or RPC writes/reads these yet (same scoping the
-- connections table itself had before this pass). Tracked in
-- docs/execution-plan.md §2 Epic A as a follow-up once a real caller needs
-- port-forward or provider-registry-audit behavior.
CREATE TABLE port_forwards (
    id                  CHAR(36) PRIMARY KEY,
    tenant_id           CHAR(36) NOT NULL,
    connection_id       CHAR(36) NOT NULL REFERENCES connections(id),
    local_port          INTEGER NOT NULL CHECK (local_port BETWEEN 1 AND 65535),
    remote_port         INTEGER NOT NULL CHECK (remote_port BETWEEN 1 AND 65535),
    created_at          TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
);

CREATE INDEX idx_infra_port_forwards_tenant ON port_forwards (tenant_id, created_at DESC);
CREATE INDEX idx_infra_port_forwards_connection ON port_forwards (connection_id);

-- No RLS equivalent — see 0001_init.up.sql's comment.

CREATE TABLE provider_registry_entries (
    id                  CHAR(36) PRIMARY KEY,
    tenant_id           CHAR(36) NOT NULL,
    connection_id       CHAR(36) NOT NULL REFERENCES connections(id),
    provider            TEXT NOT NULL,
    created_at          TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
);

CREATE INDEX idx_infra_provider_registry_entries_tenant ON provider_registry_entries (tenant_id, created_at DESC);
CREATE INDEX idx_infra_provider_registry_entries_connection ON provider_registry_entries (connection_id);

-- No RLS equivalent — see 0001_init.up.sql's comment.
