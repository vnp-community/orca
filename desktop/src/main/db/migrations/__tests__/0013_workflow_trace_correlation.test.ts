/**
 * Tests for migration 0013 — workflow_trace_correlation (TASK-BE-017.1)
 *
 * Verifies the `root_trace_id` column added to `orca_workflow_executions`,
 * used by WorkflowOrchestrator (TASK-BE-017.2) to persist the `workflow:execute`
 * root span id so resumeRunningExecutions() can resume it after a restart.
 *
 * @module main/db/migrations/__tests__/0013_workflow_trace_correlation.test
 */

import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { SqliteAdapter } from '../../sqlite/sqlite-adapter'
import { MigrationRunner } from '../runner'
import { ALL_MIGRATIONS } from '../index'

describe('migration 0013 — workflow_trace_correlation', () => {
  let db: SqliteAdapter

  beforeEach(() => {
    db = new SqliteAdapter(':memory:')
  })

  afterEach(() => db.close())

  it('is registered in ALL_MIGRATIONS as version 13, after migration 0012', () => {
    const versions = ALL_MIGRATIONS.map((m) => m.version)
    expect(versions).toContain(13)
    const idx13 = versions.indexOf(13)
    expect(versions[idx13 - 1]).toBe(12)
  })

  it('reaches version 13 after running ALL_MIGRATIONS', async () => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
    expect(await runner.currentVersion()).toBe(13)
  })

  it('up() adds a nullable root_trace_id column to orca_workflow_executions', async () => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
    const cols = await db.query('PRAGMA table_info(orca_workflow_executions)')
    const rootTraceIdCol = cols.find((c) => c['name'] === 'root_trace_id')
    expect(rootTraceIdCol).toBeDefined()
    expect(rootTraceIdCol?.['notnull']).toBe(0)
  })

  it('does not error when root_trace_id is omitted on INSERT for an existing (legacy) execution row', async () => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
    await expect(
      db.query(
        `INSERT INTO orca_workflow_executions
           (id, definition_snapshot, status, inputs_json, current_wave, triggered_by, created_at)
         VALUES (?, ?, 'pending', ?, 0, ?, ?)`,
        ['exec-legacy-1', '{"steps":[]}', '{}', 'user-1', Date.now()]
      )
    ).resolves.toBeDefined()

    const rows = await db.query(
      'SELECT root_trace_id as rootTraceId FROM orca_workflow_executions WHERE id = ?',
      ['exec-legacy-1']
    )
    expect(rows[0]?.['rootTraceId']).toBeNull()
  })

  it('root_trace_id round-trips a persisted span id', async () => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
    await db.query(
      `INSERT INTO orca_workflow_executions
         (id, definition_snapshot, status, inputs_json, current_wave, triggered_by, created_at, root_trace_id)
       VALUES (?, ?, 'pending', ?, 0, ?, ?, ?)`,
      ['exec-with-trace', '{"steps":[]}', '{}', 'user-1', Date.now(), 'span-abc123']
    )
    const rows = await db.query(
      'SELECT root_trace_id as rootTraceId FROM orca_workflow_executions WHERE id = ?',
      ['exec-with-trace']
    )
    expect(rows[0]?.['rootTraceId']).toBe('span-abc123')
  })

  it('down() is a safe no-op — table still exists after rollback', async () => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
    await runner.rollbackTo(12)
    const rows = await db.query(
      "SELECT name FROM sqlite_master WHERE type='table' AND name='orca_workflow_executions'"
    )
    expect(rows).toHaveLength(1)
  })
})
