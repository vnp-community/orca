-- infra-fleet-service owns this database exclusively — no other service
-- reads or writes these tables. See
-- specs/backend-go/architecture/05-data-architecture.md.
--
-- This is the proto-scoped subset of the fuller schema sketched in
-- specs/backend-go/services/infra-fleet-service.md §5: no separate
-- `connections`, `port_forwards`, `provider_registry_entries`, or
-- `terminal_sessions` tables yet — ResolveConnection resolves directly
-- against dev_servers (see internal/adapter/postgres/repository.go's doc
-- comment), and fleet_health stores only the latest sample per dev server
-- rather than a sampled-over-time history. See this service's README
-- "Known gaps" for the follow-up.
CREATE SCHEMA IF NOT EXISTS infra;

CREATE TABLE infra.dev_servers (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    host                TEXT NOT NULL,
    connection_mode     TEXT NOT NULL CHECK (connection_mode IN
                           ('relay-ssh', 'relay-websocket', 'direct-websocket')),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_infra_dev_servers_tenant ON infra.dev_servers (tenant_id, created_at DESC);

-- Row-Level Security as defense-in-depth per architecture/05 — the
-- application layer's explicit tenant_id filtering (see internal/adapter/postgres)
-- is the primary enforcement; this is the secondary backstop.
ALTER TABLE infra.dev_servers ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON infra.dev_servers
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- ADR-021's ssh_targets table (see infra-fleet-service.md §5). auth material
-- is never stored here — vault_ssh_role is a pointer into Vault's SSH
-- secrets engine role used to issue a short-lived certificate per
-- connection, see domain.SshTarget's invariant.
CREATE TABLE infra.ssh_targets (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    host                TEXT NOT NULL,
    user_name           TEXT NOT NULL,
    vault_ssh_role      TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_infra_ssh_targets_tenant ON infra.ssh_targets (tenant_id, created_at DESC);

ALTER TABLE infra.ssh_targets ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON infra.ssh_targets
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- Latest fleet-health sample per dev server. No tenant_id column of its own
-- — tenant scope is inherited transitively through the dev_server_id FK and
-- enforced with an explicit join-through-tenant at the repository layer
-- (GetFleetHealth), per infra-fleet-service.md §5's documented pattern for
-- tables that don't carry their own tenant_id. A periodic 30s-cadence
-- poller (see infra-fleet-service.md §8) is the intended writer of this
-- table; it is not implemented in this scaffold.
CREATE TABLE infra.fleet_health (
    dev_server_id       UUID PRIMARY KEY REFERENCES infra.dev_servers(id),
    reachable           BOOLEAN NOT NULL,
    cpu_percent         DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (cpu_percent BETWEEN 0 AND 100),
    ram_percent         DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (ram_percent BETWEEN 0 AND 100),
    disk_percent        DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (disk_percent BETWEEN 0 AND 100),
    latency_ms          BIGINT NOT NULL DEFAULT 0 CHECK (latency_ms >= 0),
    checked_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_infra_fleet_health_checked_at ON infra.fleet_health (checked_at DESC);
