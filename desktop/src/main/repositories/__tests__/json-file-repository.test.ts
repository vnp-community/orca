import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mkdtempSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { JsonFileStateRepository } from '../json-file-repository'

function makeRepo(): { repo: JsonFileStateRepository; file: string; tmpDir: string } {
  const tmpDir = mkdtempSync(join(tmpdir(), 'orca-json-repo-test-'))
  const file = join(tmpDir, 'store.json')
  return { repo: new JsonFileStateRepository(file), file, tmpDir }
}

describe('JsonFileStateRepository', () => {
  let tmpDir: string
  let repo: JsonFileStateRepository
  let storeFile: string

  beforeEach(() => {
    const r = makeRepo()
    tmpDir = r.tmpDir
    repo = r.repo
    storeFile = r.file
  })

  afterEach(async () => {
    await repo.close()
    rmSync(tmpDir, { recursive: true, force: true })
  })

  describe('ping()', () => {
    it('returns true', async () => {
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
        createdAt: Date.now(), updatedAt: Date.now()
      } as any)
      expect(p.id).toBeTruthy()
      expect(p.id).toMatch(/^[0-9a-f-]{36}$/)
    })

    it('findById() finds created project', async () => {
      const created = await repo.projects.create({
        displayName: 'P1', badgeColor: '#000', sourceRepoIds: [],
        createdAt: Date.now(), updatedAt: Date.now()
      } as any)
      const found = await repo.projects.findById(created.id)
      expect(found?.displayName).toBe('P1')
    })

    it('findById() returns null for unknown id', async () => {
      expect(await repo.projects.findById('no-such-id')).toBeNull()
    })

    it('update() changes fields', async () => {
      const created = await repo.projects.create({
        displayName: 'Old', badgeColor: '#000', sourceRepoIds: [],
        createdAt: Date.now(), updatedAt: Date.now()
      } as any)
      const updated = await repo.projects.update(created.id, { displayName: 'New' } as any)
      expect(updated.displayName).toBe('New')
    })

    it('update() throws for unknown id', async () => {
      await expect(
        repo.projects.update('bad-id', { displayName: 'X' } as any)
      ).rejects.toThrow('Project not found: bad-id')
    })

    it('delete() removes project', async () => {
      const p = await repo.projects.create({
        displayName: 'Del', badgeColor: '#fff', sourceRepoIds: [],
        createdAt: Date.now(), updatedAt: Date.now()
      } as any)
      await repo.projects.delete(p.id)
      expect(await repo.projects.findById(p.id)).toBeNull()
    })

    it('findAll() returns all projects', async () => {
      await repo.projects.create({ displayName: 'P1', badgeColor: '#fff', sourceRepoIds: [], createdAt: 0, updatedAt: 0 } as any)
      await repo.projects.create({ displayName: 'P2', badgeColor: '#fff', sourceRepoIds: [], createdAt: 0, updatedAt: 0 } as any)
      expect(await repo.projects.findAll()).toHaveLength(2)
    })
  })

  describe('sshTargets', () => {
    it('create() and findById() roundtrip', async () => {
      const t = await repo.sshTargets.create({
        label: 'Dev', host: 'dev.example.com', port: 22, username: 'ubuntu'
      } as any)
      const found = await repo.sshTargets.findById(t.id)
      expect(found?.host).toBe('dev.example.com')
    })

    it('delete() removes target', async () => {
      const t = await repo.sshTargets.create({
        label: 'Prod', host: 'prod.example.com', port: 22, username: 'root'
      } as any)
      await repo.sshTargets.delete(t.id)
      expect(await repo.sshTargets.findById(t.id)).toBeNull()
    })

    it('update() changes fields', async () => {
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
    it('get() returns object', async () => {
      const s = await repo.settings.get()
      expect(s).toBeDefined()
      expect(typeof s).toBe('object')
    })

    it('update() persists setting', async () => {
      await repo.settings.update({ telemetry: true } as any)
      const s = await repo.settings.get()
      expect((s as any).telemetry).toBe(true)
    })

    it('update() merges with existing settings', async () => {
      await repo.settings.update({ telemetry: true } as any)
      await repo.settings.update({ debugMode: false } as any)
      const s = await repo.settings.get()
      expect((s as any).telemetry).toBe(true)
      expect((s as any).debugMode).toBe(false)
    })
  })

  describe('persistence', () => {
    it('data persists between instances after close()', async () => {
      const created = await repo.projects.create({
        displayName: 'Persist', badgeColor: '#fff', sourceRepoIds: [],
        createdAt: 0, updatedAt: 0
      } as any)
      await repo.close()

      const repo2 = new JsonFileStateRepository(storeFile)
      const found = await repo2.projects.findById(created.id)
      expect(found?.displayName).toBe('Persist')
      await repo2.close()
    })

    it('creates JSON file if it does not exist', async () => {
      const created = await repo.projects.create({
        displayName: 'New', badgeColor: '#000', sourceRepoIds: [],
        createdAt: 0, updatedAt: 0
      } as any)
      await repo.close()

      const { existsSync } = await import('node:fs')
      expect(existsSync(storeFile)).toBe(true)

      const repo2 = new JsonFileStateRepository(storeFile)
      expect(await repo2.projects.findById(created.id)).toBeTruthy()
      await repo2.close()
    })
  })
})
