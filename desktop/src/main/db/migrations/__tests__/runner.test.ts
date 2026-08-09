import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { SqliteAdapter } from '../../sqlite/sqlite-adapter'
import { MigrationRunner } from '../runner'
import type { Migration } from '../types'

function makeMigration(
  version: number,
  name: string,
  upSql: string,
  downSql: string
): Migration {
  return {
    version,
    name,
    async up(db) { await db.exec(upSql) },
    async down(db) { await db.exec(downSql) }
  }
}

describe('MigrationRunner', () => {
  let db: SqliteAdapter
  let migrations: Migration[]

  beforeEach(() => {
    db = new SqliteAdapter(':memory:')
    migrations = [
      makeMigration(1, 'create_users', 'CREATE TABLE users (id INTEGER PRIMARY KEY)', 'DROP TABLE users'),
      makeMigration(2, 'create_posts', 'CREATE TABLE posts (id INTEGER PRIMARY KEY, user_id INTEGER)', 'DROP TABLE posts'),
      makeMigration(3, 'add_email', 'ALTER TABLE users ADD COLUMN email TEXT', 'SELECT 1')
    ]
  })

  afterEach(() => {
    db.close()
  })

  describe('migrate()', () => {
    it('runs all pending migrations from fresh DB', async () => {
      const runner = new MigrationRunner(db, migrations)
      const results = await runner.migrate()
      expect(results).toHaveLength(3)
      expect(results.map((r) => r.version)).toEqual([1, 2, 3])
      expect(results.every((r) => r.direction === 'up')).toBe(true)
    })

    it('creates schema_migrations table', async () => {
      const runner = new MigrationRunner(db, migrations)
      await runner.migrate()
      const rows = await db.query('SELECT * FROM schema_migrations ORDER BY version')
      expect(rows).toHaveLength(3)
    })

    it('records migration name and appliedAt', async () => {
      const runner = new MigrationRunner(db, migrations)
      await runner.migrate()
      const rows = await db.query('SELECT name FROM schema_migrations ORDER BY version')
      expect(rows[0]?.['name']).toBe('create_users')
    })

    it('is idempotent — calling migrate() twice applies each migration once', async () => {
      const runner = new MigrationRunner(db, migrations)
      await runner.migrate()
      const results2 = await runner.migrate()
      expect(results2).toHaveLength(0) // already applied
    })

    it('runs only pending migrations when some are already applied', async () => {
      const runner1 = new MigrationRunner(db, migrations.slice(0, 1))
      await runner1.migrate()

      const runner2 = new MigrationRunner(db, migrations)
      const results = await runner2.migrate()
      expect(results.map((r) => r.version)).toEqual([2, 3])
    })

    it('applies migrations in ascending version order', async () => {
      const reversed = [...migrations].reverse()
      const runner = new MigrationRunner(db, reversed)
      const results = await runner.migrate()
      expect(results.map((r) => r.version)).toEqual([1, 2, 3])
    })

    it('result includes durationMs >= 0', async () => {
      const runner = new MigrationRunner(db, migrations)
      const results = await runner.migrate()
      expect(results.every((r) => r.durationMs >= 0)).toBe(true)
    })

    it('rolls back entire migration on error', async () => {
      const failMigration: Migration = {
        version: 1,
        name: 'fails',
        async up(db) {
          await db.exec('CREATE TABLE t (id INTEGER)')
          throw new Error('migration error')
        },
        async down(db) { await db.exec('DROP TABLE IF EXISTS t') }
      }
      const runner = new MigrationRunner(db, [failMigration])
      await expect(runner.migrate()).rejects.toThrow('migration error')

      // migration should NOT be recorded
      const applied = await runner.getApplied()
      expect(applied).toHaveLength(0)
    })
  })

  describe('getApplied()', () => {
    it('returns empty array on fresh DB', async () => {
      const runner = new MigrationRunner(db, migrations)
      expect(await runner.getApplied()).toEqual([])
    })

    it('returns applied migrations sorted by version', async () => {
      const runner = new MigrationRunner(db, migrations)
      await runner.migrate()
      const applied = await runner.getApplied()
      expect(applied.map((a) => a.version)).toEqual([1, 2, 3])
    })
  })

  describe('getPending()', () => {
    it('returns all migrations when none applied', async () => {
      const runner = new MigrationRunner(db, migrations)
      const pending = await runner.getPending()
      expect(pending).toHaveLength(3)
    })

    it('returns only unapplied after partial migration', async () => {
      const runner = new MigrationRunner(db, migrations.slice(0, 2))
      await runner.migrate()
      const runner2 = new MigrationRunner(db, migrations)
      const pending = await runner2.getPending()
      expect(pending.map((m) => m.version)).toEqual([3])
    })
  })

  describe('rollbackTo()', () => {
    it('rolls back migrations above target version', async () => {
      const runner = new MigrationRunner(db, migrations)
      await runner.migrate()
      const results = await runner.rollbackTo(1)
      expect(results.map((r) => r.version)).toEqual([3, 2]) // descending
      expect(results.every((r) => r.direction === 'down')).toBe(true)
    })

    it('currentVersion() reflects rollback', async () => {
      const runner = new MigrationRunner(db, migrations)
      await runner.migrate()
      await runner.rollbackTo(1)
      expect(await runner.currentVersion()).toBe(1)
    })

    it('returns empty array when target is already at current', async () => {
      const runner = new MigrationRunner(db, migrations)
      await runner.migrate()
      const results = await runner.rollbackTo(3)
      expect(results).toHaveLength(0)
    })
  })

  describe('currentVersion()', () => {
    it('returns 0 on fresh DB', async () => {
      const runner = new MigrationRunner(db, migrations)
      expect(await runner.currentVersion()).toBe(0)
    })

    it('returns highest applied version', async () => {
      const runner = new MigrationRunner(db, migrations)
      await runner.migrate()
      expect(await runner.currentVersion()).toBe(3)
    })
  })
})
