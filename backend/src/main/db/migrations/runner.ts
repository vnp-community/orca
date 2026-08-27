/**
 * Migration Runner
 *
 * Manages schema versioning with atomic up/down migrations.
 * Stores applied migration state in a `schema_migrations` table.
 *
 * @module db/migrations/runner
 */

import type { IDatabase } from '../types'
import type { Migration, AppliedMigration, MigrationResult } from './types'

const MIGRATIONS_TABLE = 'schema_migrations'

/** Returns the CREATE TABLE SQL for the migration tracking table */
function createMigrationTableSql(): string {
  return `
    CREATE TABLE IF NOT EXISTS ${MIGRATIONS_TABLE} (
      version    INTEGER PRIMARY KEY,
      name       TEXT    NOT NULL,
      applied_at TEXT    NOT NULL
    )
  `
}

export class MigrationRunner {
  constructor(
    private readonly db: IDatabase,
    private readonly migrations: readonly Migration[]
  ) {}

  /** Ensure the migrations tracking table exists */
  private async ensureTable(): Promise<void> {
    await this.db.exec(createMigrationTableSql())
  }

  /** List all applied migrations from the DB */
  async getApplied(): Promise<AppliedMigration[]> {
    await this.ensureTable()
    const rows = await this.db.query(
      `SELECT version, name, applied_at as "appliedAt" FROM ${MIGRATIONS_TABLE} ORDER BY version ASC`
    )
    return rows.map((r) => ({
      version: r['version'] as number,
      name: r['name'] as string,
      appliedAt: r['appliedAt'] as string
    }))
  }

  /** Return pending migrations (not yet applied) */
  async getPending(): Promise<Migration[]> {
    const applied = await this.getApplied()
    const appliedVersions = new Set(applied.map((a) => a.version))
    return [...this.migrations]
      .filter((m) => !appliedVersions.has(m.version))
      .sort((a, b) => a.version - b.version)
  }

  /**
   * Run all pending migrations in order.
   * Each migration runs in its own transaction (atomic).
   * @returns Array of applied MigrationResult
   */
  async migrate(): Promise<MigrationResult[]> {
    await this.ensureTable()
    const pending = await this.getPending()
    const results: MigrationResult[] = []

    for (const migration of pending) {
      const startMs = Date.now()
      await this.db.transaction(async () => {
        await migration.up(this.db)
        await this.db.query(
          `INSERT INTO ${MIGRATIONS_TABLE} (version, name, applied_at) VALUES (?, ?, ?)`,
          [migration.version, migration.name, new Date().toISOString()]
        )
      })
      results.push({
        version: migration.version,
        name: migration.name,
        direction: 'up',
        durationMs: Date.now() - startMs
      })
    }

    return results
  }

  /**
   * Roll back to a specific target version (exclusive).
   * Runs down() for all applied migrations with version > targetVersion.
   * @returns Array of rolled-back MigrationResult
   */
  async rollbackTo(targetVersion: number): Promise<MigrationResult[]> {
    await this.ensureTable()
    const applied = await this.getApplied()
    const toRollback = applied
      .filter((a) => a.version > targetVersion)
      .sort((a, b) => b.version - a.version) // descending

    const migrationMap = new Map(this.migrations.map((m) => [m.version, m]))
    const results: MigrationResult[] = []

    for (const applied of toRollback) {
      const migration = migrationMap.get(applied.version)
      if (!migration) {
        throw new Error(
          `Cannot rollback version ${applied.version}: migration definition not found`
        )
      }
      const startMs = Date.now()
      await this.db.transaction(async () => {
        await migration.down(this.db)
        await this.db.query(
          `DELETE FROM ${MIGRATIONS_TABLE} WHERE version = ?`,
          [migration.version]
        )
      })
      results.push({
        version: migration.version,
        name: migration.name,
        direction: 'down',
        durationMs: Date.now() - startMs
      })
    }

    return results
  }

  /**
   * Return the highest applied migration version, or 0 if none applied.
   */
  async currentVersion(): Promise<number> {
    await this.ensureTable()
    const rows = await this.db.query(
      `SELECT MAX(version) as v FROM ${MIGRATIONS_TABLE}`
    )
    const v = rows[0]?.['v']
    return v != null ? (v as number) : 0
  }
}
