import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { SqliteAdapter } from '../../sqlite/sqlite-adapter'
import { MigrationRunner } from '../runner'
import { ALL_MIGRATIONS } from '../index'

describe('ALL_MIGRATIONS registry', () => {
  let db: SqliteAdapter

  beforeEach(() => {
    db = new SqliteAdapter(':memory:')
  })

  afterEach(() => db.close())

  it('contains 10 migrations', () => {
    expect(ALL_MIGRATIONS).toHaveLength(10)
  })

  it('migrations are ordered by version ascending', () => {
    const versions = ALL_MIGRATIONS.map((m) => m.version)
    expect(versions).toEqual([1, 2, 3, 4, 5, 6, 7, 8, 9, 10])
  })

  it('all migrations have unique versions', () => {
    const versions = ALL_MIGRATIONS.map((m) => m.version)
    expect(new Set(versions).size).toBe(versions.length)
  })

  it('all migrations have non-empty names', () => {
    expect(ALL_MIGRATIONS.every((m) => m.name.length > 0)).toBe(true)
  })

  it('running ALL_MIGRATIONS reaches version 10', async () => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
    expect(await runner.currentVersion()).toBe(10)
  })
})

describe('migration 0002 — add_automations', () => {
  let db: SqliteAdapter

  beforeEach(async () => {
    db = new SqliteAdapter(':memory:')
    // Run migration 0001 first (dependency)
    const runner = new MigrationRunner(db, ALL_MIGRATIONS.slice(0, 1))
    await runner.migrate()
  })

  afterEach(() => db.close())

  it('up() creates automations table', async () => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
    const rows = await db.query(
      "SELECT name FROM sqlite_master WHERE type='table' AND name='automations'"
    )
    expect(rows).toHaveLength(1)
  })

  it('can INSERT into automations', async () => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
    await db.query(
      "INSERT INTO projects (id, name, path, created_at, updated_at) VALUES ('p1', 'P', '/p', datetime('now'), datetime('now'))"
    )
    await db.query(
      "INSERT INTO automations (id, project_id, name, trigger, config) VALUES ('a1', 'p1', 'test', 'push', '{}')"
    )
    const rows = await db.query("SELECT name FROM automations WHERE id='a1'")
    expect(rows[0]?.['name']).toBe('test')
  })

  it('enabled defaults to 1', async () => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
    await db.query(
      "INSERT INTO projects (id, name, path, created_at, updated_at) VALUES ('p1', 'P', '/p', datetime('now'), datetime('now'))"
    )
    await db.query(
      "INSERT INTO automations (id, project_id, name, trigger, config) VALUES ('a1', 'p1', 'test', 'push', '{}')"
    )
    const rows = await db.query("SELECT enabled FROM automations WHERE id='a1'")
    expect(rows[0]?.['enabled']).toBe(1)
  })

  it('down() drops automations table', async () => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
    await runner.rollbackTo(1)
    const rows = await db.query(
      "SELECT name FROM sqlite_master WHERE type='table' AND name='automations'"
    )
    expect(rows).toHaveLength(0)
  })
})

describe('migration 0003 — add_workspace_sessions', () => {
  let db: SqliteAdapter

  beforeEach(async () => {
    db = new SqliteAdapter(':memory:')
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
  })

  afterEach(() => db.close())

  it('workspace_sessions table exists after migration', async () => {
    const rows = await db.query(
      "SELECT name FROM sqlite_master WHERE type='table' AND name='workspace_sessions'"
    )
    expect(rows).toHaveLength(1)
  })

  it('can INSERT a workspace session', async () => {
    await db.query(
      "INSERT INTO projects (id, name, path, created_at, updated_at) VALUES ('p1', 'P', '/p', datetime('now'), datetime('now'))"
    )
    await db.query(
      "INSERT INTO workspace_sessions (id, project_id, agent) VALUES ('s1', 'p1', 'claude')"
    )
    const rows = await db.query("SELECT agent FROM workspace_sessions WHERE id='s1'")
    expect(rows[0]?.['agent']).toBe('claude')
  })

  it('status defaults to active', async () => {
    await db.query(
      "INSERT INTO projects (id, name, path, created_at, updated_at) VALUES ('p1', 'P', '/p', datetime('now'), datetime('now'))"
    )
    await db.query(
      "INSERT INTO workspace_sessions (id, project_id, agent) VALUES ('s1', 'p1', 'claude')"
    )
    const rows = await db.query("SELECT status FROM workspace_sessions WHERE id='s1'")
    expect(rows[0]?.['status']).toBe('active')
  })

  it('down() drops workspace_sessions table', async () => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.rollbackTo(2)
    const rows = await db.query(
      "SELECT name FROM sqlite_master WHERE type='table' AND name='workspace_sessions'"
    )
    expect(rows).toHaveLength(0)
  })
})
