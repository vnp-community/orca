/**
 * Migration Registry
 *
 * ALL_MIGRATIONS is the canonical ordered list of all migrations.
 * Import this in server-bootstrap to auto-migrate.
 *
 * @module db/migrations/index
 */

import { migration0001InitialSchema } from './0001_initial_schema'
import { migration0002AddAutomations } from './0002_add_automations'
import { migration0003AddWorkspaceSessions } from './0003_add_workspace_sessions'
import { migration0004OrcaAppTables } from './0004_orca_app_tables'
import { migration0005AddAuthSchema } from './0005_add_auth_schema'
import type { Migration } from './types'

/** All migrations in version order. */
export const ALL_MIGRATIONS: readonly Migration[] = [
  migration0001InitialSchema,
  migration0002AddAutomations,
  migration0003AddWorkspaceSessions,
  migration0004OrcaAppTables,
  migration0005AddAuthSchema
]

export { MigrationRunner } from './runner'
export type { Migration, AppliedMigration, MigrationResult } from './types'
