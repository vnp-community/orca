-- External MCP server registry (BE-MCP-SOL-014). No column anywhere holds a
-- secret value: only reference NAMES and the credential-broker owner pointer.
CREATE TABLE mcp.external_servers (
    id                 UUID PRIMARY KEY,
    tenant_id          UUID NOT NULL,
    scope              TEXT NOT NULL CHECK (scope IN ('tenant', 'team', 'user')),
    scope_id           TEXT NOT NULL,   -- tenant: tenant id; team/user: logical id (no cross-DB FK)
    name               TEXT NOT NULL CHECK (name ~ '^[a-z0-9][a-z0-9_-]{0,62}$'),
    transport          TEXT NOT NULL CHECK (transport IN ('http', 'stdio')),
    url                TEXT,
    command            TEXT,
    args               JSONB NOT NULL DEFAULT '[]',
    status             TEXT NOT NULL DEFAULT 'pending_review' CHECK (status IN ('pending_review', 'approved', 'disabled')),
    spec_digest        TEXT NOT NULL,   -- sha256(transport,url|command,args,env/header NAMES); a change forces re-review
    last_probe_digest  TEXT,
    last_probe_at      TIMESTAMPTZ,
    last_probe_tools   JSONB,           -- snapshot copied to approved_tools on approve
    approved_digest    TEXT,
    approved_tools     JSONB,           -- {name,description}[] for the UI diff: untrusted text
    health_ok          BOOLEAN,
    health_checked_at  TIMESTAMPTZ,
    health_error       TEXT,
    health_claimed_at  TIMESTAMPTZ,     -- multi-replica worker claim stamp
    created_by         TEXT NOT NULL,
    reviewed_by        TEXT,
    reviewed_at        TIMESTAMPTZ,
    version            INT NOT NULL DEFAULT 1,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT external_http_has_url  CHECK (transport <> 'http'  OR url IS NOT NULL),
    CONSTRAINT external_stdio_has_cmd CHECK (transport <> 'stdio' OR command IS NOT NULL),
    CONSTRAINT external_servers_unique_name UNIQUE (tenant_id, scope, scope_id, name)
);
CREATE INDEX idx_external_servers_tenant_name ON mcp.external_servers (tenant_id, name);
CREATE INDEX idx_external_servers_health ON mcp.external_servers (health_claimed_at NULLS FIRST)
    WHERE status = 'approved' AND transport = 'http';

-- Pointers only. broker_owner_id NULL = declared but no value set yet.
CREATE TABLE mcp.external_server_secret_refs (
    server_id       UUID NOT NULL REFERENCES mcp.external_servers (id) ON DELETE CASCADE,
    tenant_id       UUID NOT NULL,
    kind            TEXT NOT NULL CHECK (kind IN ('env', 'header')),
    name            TEXT NOT NULL,
    broker_owner_id TEXT,
    set_by          TEXT,
    set_at          TIMESTAMPTZ,
    PRIMARY KEY (server_id, kind, name)
);

-- Digest history (rug-pull forensics).
CREATE TABLE mcp.external_server_tools_history (
    id          UUID PRIMARY KEY,
    tenant_id   UUID NOT NULL,
    server_id   UUID NOT NULL REFERENCES mcp.external_servers (id) ON DELETE CASCADE,
    digest      TEXT NOT NULL,
    tools       JSONB NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL,
    source      TEXT NOT NULL CHECK (source IN ('probe', 'health')),
    decision    TEXT CHECK (decision IN ('approved', 'rejected')),
    decided_by  TEXT
);
CREATE INDEX idx_external_tools_history ON mcp.external_server_tools_history (server_id, observed_at DESC);

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['external_servers', 'external_server_secret_refs', 'external_server_tools_history'] LOOP
    EXECUTE format('ALTER TABLE mcp.%I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE mcp.%I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format($p$CREATE POLICY tenant_isolation ON mcp.%I
        USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
        WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)$p$, t);
  END LOOP;
END $$;

-- The health worker discovers and stamps due servers across tenants, then
-- works inside ordinary tenant-scoped transactions.
CREATE POLICY worker_claim_select ON mcp.external_servers FOR SELECT
    USING (current_setting('app.relay', true) = 'on' AND status = 'approved' AND transport = 'http');
CREATE POLICY worker_claim_update ON mcp.external_servers FOR UPDATE
    USING (current_setting('app.relay', true) = 'on' AND status = 'approved' AND transport = 'http')
    WITH CHECK (current_setting('app.relay', true) = 'on' AND status = 'approved' AND transport = 'http');
