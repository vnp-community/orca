-- tenant-service owns this database exclusively — no other service reads or
-- writes these tables. MySQL/TiDB variant: no CREATE SCHEMA (a MySQL
-- database IS the schema-equivalent isolation unit — this migration assumes
-- DATABASE_DSN already points at a database named `tenant`, mirroring the
-- Postgres variant's `tenant` schema name; see
-- specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-002.md's
-- naming convention). Table names carry no `tenant_`/`tenant.` prefix —
-- the database itself is the isolation unit.
--
-- Dialect translation notes (BE-DB-SOL-011):
--  - UUID -> CHAR(36) (ids are generated in Go, never gen_random_uuid()).
--  - JSONB -> JSON, with DEFAULT (JSON_OBJECT()) replacing DEFAULT '{}' —
--    MySQL 8.0.13+ expression-default syntax (verified against mysql:8, the
--    same image common/testutil.StartMySQL uses).
--  - TIMESTAMPTZ -> TIMESTAMP(6) (application always writes UTC; MySQL's
--    TIMESTAMP is UTC-normalized internally, matching Postgres's `now()`
--    columns closely enough for this service's opaque timestamp usage).
--  - Row-Level Security has no MySQL equivalent. tenant-service is the
--    tenant ROOT (companies has no tenant_id column of its own — see
--    tenant-service.md §5), so the RLS policies below only ever applied to
--    departments/user_profiles/teams/team_members — dropped here per
--    dbcapability.Capabilities.SupportsRLS=false, application-layer
--    company_id filtering (internal/adapter/mysql) is the ONLY enforcement
--    on this dialect, same posture usage-service's BE-DB-SOL-001 §4
--    established: no code anywhere in backend-go ever calls
--    `SET LOCAL app.tenant_id`, so these policies never actually activated
--    on Postgres either — dropping them for MySQL does not lower real
--    protection, see TASK-BE-DB-016's tenant-isolation-without-RLS tests
--    for the proof this still holds.
CREATE TABLE companies (
    id             CHAR(36) PRIMARY KEY,
    name           TEXT NOT NULL,
    settings_json  JSON NOT NULL DEFAULT (JSON_OBJECT()),
    admin_user_id  CHAR(36),           -- logical FK -> auth.users, validated via auth-service's API at write time, never joined in SQL
    created_at     TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at     TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_by     CHAR(36)
);
-- No RLS on companies in the Postgres variant either — see that migration's
-- own comment (this table IS the tenant root, tenant-service.md §5).

CREATE TABLE departments (
    id             CHAR(36) PRIMARY KEY,
    company_id     CHAR(36) NOT NULL REFERENCES companies(id),
    name           TEXT NOT NULL,
    settings_json  JSON NOT NULL DEFAULT (JSON_OBJECT()),
    created_at     TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at     TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_by     CHAR(36)
);
CREATE INDEX idx_departments_company ON departments (company_id);
-- RLS dropped for this dialect — see file-level comment above.

CREATE TABLE user_profiles (
    user_id        CHAR(36) PRIMARY KEY,   -- logical FK -> auth.users (different DB)
    company_id     CHAR(36) NOT NULL REFERENCES companies(id),
    department_id  CHAR(36) REFERENCES departments(id) ON DELETE SET NULL, -- unset means company-only inheritance
    settings_json  JSON NOT NULL DEFAULT (JSON_OBJECT()),
    updated_at     TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
);
CREATE INDEX idx_user_profiles_company ON user_profiles (company_id);
CREATE INDEX idx_user_profiles_department ON user_profiles (department_id);
-- RLS dropped for this dialect — see file-level comment above.

CREATE TABLE teams (
    id             CHAR(36) PRIMARY KEY,
    company_id     CHAR(36) NOT NULL REFERENCES companies(id),
    name           TEXT NOT NULL,
    settings_json  JSON NOT NULL DEFAULT (JSON_OBJECT()), -- team-layer profile override; see tenant-service.md §4's 4-layer merge
    created_at     TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at     TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
    -- deliberately no department_id: teams are not scoped to one
    -- department by design (tenant-service.md §4).
);
CREATE INDEX idx_teams_company ON teams (company_id);
-- RLS dropped for this dialect — see file-level comment above.

CREATE TABLE team_members (
    team_id     CHAR(36) NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id     CHAR(36) NOT NULL,          -- logical FK -> auth.users
    -- role: TEXT -> VARCHAR(32) — never read/written by any Go code today
    -- (grep-confirmed: team_repository.go's AddMember never sets it,
    -- ListMembers never selects it, domain.TeamMember has no Role field),
    -- narrowed for a plain literal DEFAULT rather than relying on MySQL
    -- 8.0.13+'s TEXT expression-default syntax for an inert column.
    role        VARCHAR(32) NOT NULL DEFAULT 'member',
    priority    INT NOT NULL DEFAULT 0, -- tiebreaker used by profile resolution, see tenant-service.md §4
    added_at    TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    PRIMARY KEY (team_id, user_id)
);
CREATE INDEX idx_team_members_user ON team_members (user_id); -- used for cascade team-layer resolution
-- team_members has no company_id column of its own; the Postgres variant's
-- policy checked the owning team's company_id via a subquery — same
-- dropped-for-this-dialect posture as every other table above.
