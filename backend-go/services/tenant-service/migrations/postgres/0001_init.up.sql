-- tenant-service owns this database exclusively — no other service reads or
-- writes these tables. See specs/backend-go/architecture/05-data-architecture.md.
-- tenant-service is the ONE service whose primary table has no tenant_id
-- column: tenant.companies.id IS the tenant_id every other service's schema
-- logically references (tenant-service.md §1/§5).
CREATE SCHEMA IF NOT EXISTS tenant;

CREATE TABLE tenant.companies (
    id             UUID PRIMARY KEY,
    name           TEXT NOT NULL,
    settings_json  JSONB NOT NULL DEFAULT '{}',
    admin_user_id  UUID,               -- logical FK -> auth.users, validated via auth-service's API at write time, never joined in SQL
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by     UUID
);
-- No RLS on companies: a request either has a validated company_id and
-- looks up exactly that row, or it doesn't (tenant-service.md §5).

CREATE TABLE tenant.departments (
    id             UUID PRIMARY KEY,
    company_id     UUID NOT NULL REFERENCES tenant.companies(id),
    name           TEXT NOT NULL,
    settings_json  JSONB NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by     UUID
);
CREATE INDEX idx_departments_company ON tenant.departments (company_id);

-- Row-Level Security as defense-in-depth per architecture/05 — the
-- application layer's explicit company_id filtering (see internal/adapter/postgres)
-- is the primary enforcement; this is the secondary backstop.
ALTER TABLE tenant.departments ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON tenant.departments
    USING (company_id = current_setting('app.tenant_id', true)::uuid);

CREATE TABLE tenant.user_profiles (
    user_id        UUID PRIMARY KEY,   -- logical FK -> auth.users (different DB)
    company_id     UUID NOT NULL REFERENCES tenant.companies(id),
    department_id  UUID REFERENCES tenant.departments(id) ON DELETE SET NULL, -- unset means company-only inheritance
    settings_json  JSONB NOT NULL DEFAULT '{}',
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_user_profiles_company ON tenant.user_profiles (company_id);
CREATE INDEX idx_user_profiles_department ON tenant.user_profiles (department_id);

ALTER TABLE tenant.user_profiles ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON tenant.user_profiles
    USING (company_id = current_setting('app.tenant_id', true)::uuid);

CREATE TABLE tenant.teams (
    id             UUID PRIMARY KEY,
    company_id     UUID NOT NULL REFERENCES tenant.companies(id),
    name           TEXT NOT NULL,
    settings_json  JSONB NOT NULL DEFAULT '{}', -- team-layer profile override; see tenant-service.md §4's 4-layer merge
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
    -- deliberately no department_id: teams are not scoped to one
    -- department by design (tenant-service.md §4).
);
CREATE INDEX idx_teams_company ON tenant.teams (company_id);

ALTER TABLE tenant.teams ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON tenant.teams
    USING (company_id = current_setting('app.tenant_id', true)::uuid);

CREATE TABLE tenant.team_members (
    team_id     UUID NOT NULL REFERENCES tenant.teams(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL,          -- logical FK -> auth.users
    role        TEXT NOT NULL DEFAULT 'member',
    priority    INT NOT NULL DEFAULT 0, -- tiebreaker used by profile resolution, see tenant-service.md §4
    added_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (team_id, user_id)
);
CREATE INDEX idx_team_members_user ON tenant.team_members (user_id); -- used for cascade team-layer resolution

-- team_members has no company_id column of its own; this policy checks the
-- owning team's company_id, same defense-in-depth intent as the other
-- tables (tenant-service.md §5).
ALTER TABLE tenant.team_members ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON tenant.team_members
    USING (team_id IN (SELECT id FROM tenant.teams WHERE company_id = current_setting('app.tenant_id', true)::uuid));
