import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { SqliteSingleConnectionPool } from '../../db/sqlite/sqlite-pool'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations'
import { SqlStateRepository } from '../sql-repository'

async function makeRepo(): Promise<SqlStateRepository> {
  const pool = new SqliteSingleConnectionPool(':memory:')
  await pool.withConnection(async (db) => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
  })
  return new SqlStateRepository(pool)
}

describe('SqlStateRepository', () => {
  let repo: SqlStateRepository

  beforeEach(async () => {
    repo = await makeRepo()
  })

  afterEach(async () => {
    await repo.close().catch(() => {})
  })

  describe('ping()', () => {
    it('returns true for working pool', async () => {
      expect(await repo.ping()).toBe(true)
    })
  })

  describe('projects', () => {
    it('findAll() returns empty initially', async () => {
      expect(await repo.projects.findAll()).toEqual([])
    })

    it('create() assigns UUID id', async () => {
      const p = await repo.projects.create({
        displayName: 'Test', badgeColor: '#fff', sourceRepoIds: [],
        createdAt: 0, updatedAt: 0
      } as any)
      expect(p.id).toBeTruthy()
      expect(p.id).toMatch(/^[0-9a-f-]{36}$/)
    })

    it('findById() roundtrip', async () => {
      const created = await repo.projects.create({
        displayName: 'P1', badgeColor: '#000', sourceRepoIds: [], createdAt: 0, updatedAt: 0
      } as any)
      const found = await repo.projects.findById(created.id)
      expect(found).toBeTruthy()
      expect(found?.displayName).toBe('P1')
    })

    it('findById() returns null for unknown', async () => {
      expect(await repo.projects.findById('no-such')).toBeNull()
    })

    it('update() changes project fields', async () => {
      const p = await repo.projects.create({
        displayName: 'Old', badgeColor: '#fff', sourceRepoIds: [], createdAt: 0, updatedAt: 0
      } as any)
      const updated = await repo.projects.update(p.id, { displayName: 'New' } as any)
      expect(updated.displayName).toBe('New')
    })

    it('update() throws for unknown id', async () => {
      await expect(
        repo.projects.update('bad', { displayName: 'X' } as any)
      ).rejects.toThrow('Project not found: bad')
    })

    it('delete() removes project', async () => {
      const p = await repo.projects.create({
        displayName: 'Del', badgeColor: '#fff', sourceRepoIds: [], createdAt: 0, updatedAt: 0
      } as any)
      await repo.projects.delete(p.id)
      expect(await repo.projects.findById(p.id)).toBeNull()
    })

    it('findAll() returns multiple', async () => {
      await repo.projects.create({ displayName: 'A', badgeColor: '#fff', sourceRepoIds: [], createdAt: 0, updatedAt: 0 } as any)
      await repo.projects.create({ displayName: 'B', badgeColor: '#fff', sourceRepoIds: [], createdAt: 0, updatedAt: 0 } as any)
      expect(await repo.projects.findAll()).toHaveLength(2)
    })
  })

  describe('sshTargets', () => {
    it('create() and findById() roundtrip', async () => {
      const t = await repo.sshTargets.create({
        label: 'Dev', host: 'dev.example.com', port: 22, username: 'ubuntu'
      } as any)
      const found = await repo.sshTargets.findById(t.id)
      expect(found).toBeTruthy()
    })

    it('delete() removes target', async () => {
      const t = await repo.sshTargets.create({
        label: 'Prod', host: 'prod.example.com', port: 22, username: 'root'
      } as any)
      await repo.sshTargets.delete(t.id)
      expect(await repo.sshTargets.findById(t.id)).toBeNull()
    })

    it('update() changes target', async () => {
      const t = await repo.sshTargets.create({
        label: 'Old', host: 'host.example.com', port: 22, username: 'user'
      } as any)
      const updated = await repo.sshTargets.update(t.id, { label: 'New' } as any)
      expect(updated.label).toBe('New')
    })

    it('update() throws for unknown id', async () => {
      await expect(
        repo.sshTargets.update('bad-id', { label: 'X' } as any)
      ).rejects.toThrow('SshTarget not found: bad-id')
    })
  })

  describe('settings', () => {
    it('get() returns empty object initially', async () => {
      const s = await repo.settings.get()
      expect(s).toEqual({})
    })

    it('update() persists settings', async () => {
      await repo.settings.update({ telemetry: true } as any)
      const s = await repo.settings.get()
      expect((s as any).telemetry).toBe(true)
    })

    it('update() merges with existing', async () => {
      await repo.settings.update({ telemetry: true } as any)
      await repo.settings.update({ debugMode: false } as any)
      const s = await repo.settings.get()
      expect((s as any).telemetry).toBe(true)
      expect((s as any).debugMode).toBe(false)
    })
  })
})
