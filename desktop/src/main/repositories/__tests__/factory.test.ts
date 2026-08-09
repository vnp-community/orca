import { describe, it, expect, vi } from 'vitest'
import { createStateRepository } from '../factory'
import { mkdtempSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'

describe('createStateRepository', () => {
  it('throws when neither pool nor dataFile provided', () => {
    expect(() => createStateRepository({})).toThrow(
      'must provide either pool (SQL backend) or dataFile (JSON backend)'
    )
  })

  it('returns JsonFileStateRepository when dataFile is provided', async () => {
    const tmpDir = mkdtempSync(join(tmpdir(), 'orca-factory-test-'))
    const file = join(tmpDir, 'store.json')
    try {
      const repo = createStateRepository({ dataFile: file })
      expect(repo).toBeDefined()
      expect(await repo.ping()).toBe(true)
      await repo.close()
    } finally {
      rmSync(tmpDir, { recursive: true, force: true })
    }
  })

  it('returns SqlStateRepository when pool is provided', async () => {
    const { SqliteSingleConnectionPool } = await import('../../db/sqlite/sqlite-pool')
    const { MigrationRunner } = await import('../../db/migrations/runner')
    const { ALL_MIGRATIONS } = await import('../../db/migrations')

    const pool = new SqliteSingleConnectionPool(':memory:')
    await pool.withConnection(async (db) => {
      const runner = new MigrationRunner(db, ALL_MIGRATIONS)
      await runner.migrate()
    })

    const repo = createStateRepository({ pool })
    expect(repo).toBeDefined()
    expect(await repo.ping()).toBe(true)
    await repo.close()
  })

  it('pool takes priority over dataFile', async () => {
    const { SqliteSingleConnectionPool } = await import('../../db/sqlite/sqlite-pool')
    const { MigrationRunner } = await import('../../db/migrations/runner')
    const { ALL_MIGRATIONS } = await import('../../db/migrations')

    const pool = new SqliteSingleConnectionPool(':memory:')
    await pool.withConnection(async (db) => {
      const runner = new MigrationRunner(db, ALL_MIGRATIONS)
      await runner.migrate()
    })

    // Provide both — pool should win (SqlStateRepository, not JsonFileStateRepository)
    const repo = createStateRepository({ pool, dataFile: '/tmp/should-not-be-created.json' })
    expect(repo).toBeDefined()
    expect(repo.constructor.name).toBe('SqlStateRepository')
    await repo.close()
  })
})
