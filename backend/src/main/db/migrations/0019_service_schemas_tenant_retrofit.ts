/**
 * Migration 0019 — Service Schemas + Tenant Retrofit (Phase 0 of ADR-021)
 *
 * Phase 0 of "hợp nhất server-mode data plane vào Postgres" (ADR-021,
 * specs/backend/models/08-postgres-microservices-target-architecture.md):
 *   1. Creates 1 Postgres SCHEMA per target microservice boundary — namespacing
 *      only in this migration, no tables moved yet (that happens per-service in
 *      later phases, see ADR-021 §Trạng thái Implementation).
 *   2. Retrofits a nullable `tenant_id` column onto every ACTIVE table that is
 *      scoped to one organization (see specs/backend/models/02-sql-schema-catalog.md
 *      for which tables are "active" vs "dormant" — only active tables are touched).
 *
 * Deliberately additive/non-destructive: no data moved, no column made NOT NULL
 * yet (see ADR-021 Phase 1 — backfill must run and be verified before that).
 * Safe to run against the live b15.openledger.vn Postgres without any service
 * code change — nothing reads these new columns/schemas yet.
 *
 * Why `tenant_id TEXT` (not a native SQL UUID type): every id column in this
 * schema family is `TEXT` holding an app-generated `randomUUID()` string (see
 * 0001_initial_schema.ts through 0018_annotations.ts) — tenant_id follows the
 * same convention for portability (SQLite/MySQL/TiDB have no native UUID type;
 * Postgres's UUID type would be the one dialect-specific outlier here).
 *
 * Why CREATE SCHEMA is Postgres-only: SQLite has no CREATE SCHEMA at all: it
 * would fail migrate() on the common local-dev default dialect. MySQL/TiDB's
 * CREATE SCHEMA is a synonym for CREATE DATABASE — a different unit of
 * isolation than a Postgres schema, and ADR-021 defers the TiDB translation
 * (literal database-per-service, one ORCA_DB_URL per service) to whenever TiDB
 * migration actually happens, not to this migration. No-op on non-Postgres.
 *
 * @module db/migrations/0019_service_schemas_tenant_retrofit
 */

import type { Migration } from './types'

/** Target microservice boundaries — see specs/backend/models/08-postgres-microservices-target-architecture.md §2 */
const SERVICE_SCHEMAS = [
  'auth',
  'tenant',
  'project',
  'infra',
  'ai_provider',
  'workflow',
  'task',
  'orchestration',
  'automation',
  'annotation',
  'notification',
  'usage',
  'credential'
] as const

/**
 * Active tables that are scoped to exactly one organization (tenant) — see
 * specs/backend/models/02-sql-schema-catalog.md for the active/dormant survey
 * this list is derived from. Excludes:
 *   - child rows always reached via a parent join (orca_task_edges/_grants/_comments,
 *     orca_workflow_step_executions) — tenant_id would be redundant, not wrong.
 *   - orca_departments (already has company_id — that IS its tenant boundary).
 *   - orca_companies (it IS the tenant, not scoped to one).
 *   - orca_global_settings (single server-instance-wide config row, not
 *     tenant-scoped by design — see target-architecture doc §2 footnote).
 *   - orca_projects/orca_repos (0004, JSON-blob legacy store) — ownership
 *     undecided, deferred to Phase 1 (target-architecture doc §2 footnote).
 */
const TENANT_SCOPED_TABLES = [
  'orca_users',
  'orca_sessions',
  'orca_audit_log',
  'orca_access_policies',
  'orca_teams',
  'orca_team_members',
  'orca_user_profiles',
  'orca_v5_projects',
  'orca_v5_project_members',
  'orca_project_source_projects',
  'orca_ssh_targets',
  'orca_ai_provider_accounts',
  'orca_provider_usage',
  'orca_workflow_templates',
  'orca_workflow_executions',
  'orca_tasks',
  'orca_annotations'
] as const

export const migration0019ServiceSchemasTenantRetrofit: Migration = {
  version: 19,
  name: 'service_schemas_tenant_retrofit',

  async up(db) {
    // ── 1. Service schemas (Postgres only — see module doc comment) ──────────
    if (db.capabilities.dialect === 'postgresql') {
      for (const schema of SERVICE_SCHEMAS) {
        await db.exec(`CREATE SCHEMA IF NOT EXISTS ${schema}`)
      }
    }

    // ── 2. tenant_id retrofit — nullable for now (Phase 1 backfills + locks NOT NULL) ──
    for (const table of TENANT_SCOPED_TABLES) {
      try {
        await db.exec(`ALTER TABLE ${table} ADD COLUMN tenant_id TEXT`)
      } catch {
        // Idempotent — column may already exist (re-run, or partial prior apply).
        // Same pattern as 0006_company_dept.ts's department_id ALTER.
      }
      await db.exec(
        `CREATE INDEX IF NOT EXISTS idx_${table}_tenant ON ${table}(tenant_id)`
      )
    }
  },

  async down(db) {
    // tenant_id columns: SQLite doesn't support DROP COLUMN before 3.35 — no-op,
    // same pattern as every ALTER-TABLE-ADD-COLUMN migration since 0013.
    for (const table of [...TENANT_SCOPED_TABLES].reverse()) {
      await db.exec(`DROP INDEX IF EXISTS idx_${table}_tenant`)
    }
    if (db.capabilities.dialect === 'postgresql') {
      // Schemas intentionally NOT dropped on down() — they're empty namespaces
      // at Phase 0 (no tables moved in), but another concurrent migration/
      // service could have started using them; dropping is not safe to automate.
    }
  }
}
