-- infra.connections replaces the 0001-era simplification where
-- ResolveConnection equated connectionId with dev_server.id directly (see
-- postgres.Repository.ResolveConnection's prior doc comment). A connection
-- is now its own entity: which dev server owns it, plus the per-connection
-- metadata (repo_path, worktree_id) callers like git-gateway-service's
-- RelayExecutor need alongside the resolved DevServer. See
-- docs/execution-plan.md §2 Epic A's second pass and
-- specs/backend-go/services/infra-fleet-service.md §5.
CREATE TABLE infra.connections (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    dev_server_id       UUID NOT NULL REFERENCES infra.dev_servers(id),
    repo_path           TEXT NOT NULL DEFAULT '',
    worktree_id         TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_infra_connections_tenant ON infra.connections (tenant_id, created_at DESC);
CREATE INDEX idx_infra_connections_dev_server ON infra.connections (dev_server_id);
-- A worktree is bound to at most one live connection at a time — empty
-- worktree_id (connections not yet tied to a worktree) is excluded from the
-- uniqueness constraint rather than colliding on ''.
CREATE UNIQUE INDEX idx_infra_connections_tenant_worktree ON infra.connections (tenant_id, worktree_id)
    WHERE worktree_id <> '';

ALTER TABLE infra.connections ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON infra.connections
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- infra.port_forwards / infra.provider_registry_entries: schema only, per
-- the design doc's fuller sketch (specs/backend-go/services/infra-fleet-service.md
-- §5) — no usecase or RPC writes/reads these yet (same scoping the
-- connections table itself had before this pass). Tracked in
-- docs/execution-plan.md §2 Epic A as a follow-up once a real caller needs
-- port-forward or provider-registry-audit behavior.
CREATE TABLE infra.port_forwards (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    connection_id       UUID NOT NULL REFERENCES infra.connections(id),
    local_port          INTEGER NOT NULL CHECK (local_port BETWEEN 1 AND 65535),
    remote_port         INTEGER NOT NULL CHECK (remote_port BETWEEN 1 AND 65535),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_infra_port_forwards_tenant ON infra.port_forwards (tenant_id, created_at DESC);
CREATE INDEX idx_infra_port_forwards_connection ON infra.port_forwards (connection_id);

ALTER TABLE infra.port_forwards ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON infra.port_forwards
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

CREATE TABLE infra.provider_registry_entries (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    connection_id       UUID NOT NULL REFERENCES infra.connections(id),
    provider            TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_infra_provider_registry_entries_tenant ON infra.provider_registry_entries (tenant_id, created_at DESC);
CREATE INDEX idx_infra_provider_registry_entries_connection ON infra.provider_registry_entries (connection_id);

ALTER TABLE infra.provider_registry_entries ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON infra.provider_registry_entries
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
