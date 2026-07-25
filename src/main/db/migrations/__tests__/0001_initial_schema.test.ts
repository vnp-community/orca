import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { SqliteAdapter } from '../../sqlite/sqlite-adapter'
import { MigrationRunner } from '../runner'
import { migration0001InitialSchema } from '../0001_initial_schema'

describe('migration 0001 — initial_schema', () => {
  let db: SqliteAdapter
  let runner: MigrationRunner

  beforeEach(() => {
    db = new SqliteAdapter(':memory:')
    runner = new MigrationRunner(db, [migration0001InitialSchema])
  })

  afterEach(() => db.close())

  it('up() creates settings table', async () => {
    await runner.migrate()
    const rows = await db.query("SELECT name FROM sqlite_master WHERE type='table' AND name='settings'")
    expect(rows).toHaveLength(1)
  })

  it('up() creates projects table', async () => {
    await runner.migrate()
    const rows = await db.query("SELECT name FROM sqlite_master WHERE type='table' AND name='projects'")
    expect(rows).toHaveLength(1)
  })

  it('up() creates repos table', async () => {
    await runner.migrate()
    const rows = await db.query("SELECT name FROM sqlite_master WHERE type='table' AND name='repos'")
    expect(rows).toHaveLength(1)
  })

  it('up() creates ssh_targets table', async () => {
    await runner.migrate()
    const rows = await db.query("SELECT name FROM sqlite_master WHERE type='table' AND name='ssh_targets'")
    expect(rows).toHaveLength(1)
  })

  it('up() creates idx_repos_project_id index', async () => {
    await runner.migrate()
    const rows = await db.query("SELECT name FROM sqlite_master WHERE type='index' AND name='idx_repos_project_id'")
    expect(rows).toHaveLength(1)
  })

  it('can INSERT and SELECT from settings', async () => {
    await runner.migrate()
    await db.query("INSERT INTO settings (key, value, updated_at) VALUES ('theme', 'dark', datetime('now'))")
    const rows = await db.query("SELECT value FROM settings WHERE key='theme'")
    expect(rows[0]?.['value']).toBe('dark')
  })

  it('can INSERT and SELECT from projects', async () => {
    await runner.migrate()
    await db.query(
      "INSERT INTO projects (id, name, path, created_at, updated_at) VALUES ('p1', 'MyProject', '/workspace', datetime('now'), datetime('now'))"
    )
    const rows = await db.query("SELECT name FROM projects WHERE id='p1'")
    expect(rows[0]?.['name']).toBe('MyProject')
  })

  it('repos FK references projects', async () => {
    await runner.migrate()
    await db.exec('PRAGMA foreign_keys = ON')
    await db.query(
      "INSERT INTO projects (id, name, path, created_at, updated_at) VALUES ('p1', 'P', '/p', datetime('now'), datetime('now'))"
    )
    await expect(
      db.query(
        "INSERT INTO repos (id, project_id, name, created_at, updated_at) VALUES ('r1', 'nonexistent', 'R', datetime('now'), datetime('now'))"
      )
    ).rejects.toThrow()
  })

  it('down() drops all tables', async () => {
    await runner.migrate()
    await runner.rollbackTo(0)
    const tables = await db.query(
      "SELECT name FROM sqlite_master WHERE type='table' AND name IN ('settings','projects','repos','ssh_targets')"
    )
    expect(tables).toHaveLength(0)
  })

  it('is idempotent — migrate() twice is safe', async () => {
    await runner.migrate()
    const result2 = await runner.migrate()
    expect(result2).toHaveLength(0)
  })
})
