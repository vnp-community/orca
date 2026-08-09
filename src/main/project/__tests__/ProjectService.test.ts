/**
 * Tests for ProjectService (TDD-15) — TASK-017
 *
 * Uses in-memory SQLite + ALL_MIGRATIONS, following sql-repository.test.ts pattern.
 * ≥ 15 tests covering all CRUD + access control.
 *
 * @module main/project/__tests__/ProjectService.test
 */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { SqliteSingleConnectionPool } from '../../db/sqlite/sqlite-pool'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations'
import { ProjectService } from '../ProjectService'
import type { DevServerManager } from '../../dev-server/dev-server-manager'
import { registerTraceSink, type TraceEvent } from '../../../shared/trace'

// ── helpers ────────────────────────────────────────────────────────────────

const FAKE_DEV_SERVER_ID = 'dev-server-001'

/** Mock DevServerManager that accepts FAKE_DEV_SERVER_ID */
function makeMockDSM(hasServer = true): DevServerManager {
  return {
    get: vi.fn().mockReturnValue(
      hasServer ? { id: FAKE_DEV_SERVER_ID, name: 'Test Server', connectionType: 'direct-websocket', status: 'connected' } : null
    ),
  } as unknown as DevServerManager
}

async function makeService(hasServer = true): Promise<{
  pool: SqliteSingleConnectionPool
  service: ProjectService
}> {
  const pool = new SqliteSingleConnectionPool(':memory:')
  await pool.withConnection(async (db) => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
  })
  const service = new ProjectService(pool, makeMockDSM(hasServer))
  return { pool, service }
}

/** Insert a minimal user row to satisfy orca_users FK */
async function insertUser(pool: SqliteSingleConnectionPool, userId: string): Promise<void> {
  await pool.withConnection((db) =>
    db.query(
      'INSERT INTO orca_users (id, email, name, role, provider, created_at) VALUES (?, ?, ?, ?, ?, ?)',
      [userId, `${userId}@test.com`, userId, 'developer', 'none', Date.now()]
    )
  )
}

// ── tests ──────────────────────────────────────────────────────────────────

describe('ProjectService', () => {
  let pool: SqliteSingleConnectionPool
  let service: ProjectService

  beforeEach(async () => {
    const setup = await makeService()
    pool = setup.pool
    service = setup.service
  })

  afterEach(async () => {
    await pool.destroy().catch(() => {})
  })

  // ── 1. create: returns OrcaProject with correct fields ────────────────────

  it('create: returns OrcaProject with correct fields', async () => {
    await insertUser(pool, 'u-1')
    const project = await service.create({
      name: 'My Project',
      devServerId: FAKE_DEV_SERVER_ID,
      repoPath: '/home/user/repo',
      createdBy: 'u-1',
    })
    expect(project.id).toMatch(/^[0-9a-f-]{36}$/)
    expect(project.name).toBe('My Project')
    expect(project.devServerId).toBe(FAKE_DEV_SERVER_ID)
    expect(project.repoPath).toBe('/home/user/repo')
    expect(project.defaultBranch).toBe('main')
    expect(project.visibility).toBe('team')
    expect(project.createdBy).toBe('u-1')
    expect(project.createdAt).toBeInstanceOf(Date)
    expect(project.updatedAt).toBeInstanceOf(Date)
  })

  // ── 2. create: auto-adds creator as 'owner' member ────────────────────────

  it('create: auto-adds creator as owner member', async () => {
    await insertUser(pool, 'u-2')
    const project = await service.create({
      name: 'P2', devServerId: FAKE_DEV_SERVER_ID, repoPath: '/repo', createdBy: 'u-2'
    })
    const member = await service.getMember(project.id, 'u-2')
    expect(member).not.toBeNull()
    expect(member?.role).toBe('owner')
  })

  // ── 3. create: throws DEV_SERVER_NOT_FOUND when invalid devServerId ────────

  it('create: throws DEV_SERVER_NOT_FOUND when devServerId unknown', async () => {
    const { service: svcNoServer } = await makeService(false)
    await expect(
      svcNoServer.create({
        name: 'X', devServerId: 'invalid-id', repoPath: '/repo', createdBy: 'u-x'
      })
    ).rejects.toThrow('DEV_SERVER_NOT_FOUND')
  })

  // ── 4. get: returns null for non-existent project ─────────────────────────

  it('get: returns null for non-existent project', async () => {
    const result = await service.get('no-such-id')
    expect(result).toBeNull()
  })

  // ── 5. list: returns only projects where userId is member ─────────────────

  it('list: returns only projects where userId is member', async () => {
    await insertUser(pool, 'u-a')
    await insertUser(pool, 'u-b')
    const p1 = await service.create({ name: 'P-A', devServerId: FAKE_DEV_SERVER_ID, repoPath: '/ra', createdBy: 'u-a' })
    await service.create({ name: 'P-B', devServerId: FAKE_DEV_SERVER_ID, repoPath: '/rb', createdBy: 'u-b' })

    const listForA = await service.list('u-a')
    expect(listForA.length).toBe(1)
    expect(listForA[0].id).toBe(p1.id)
  })

  // ── 6. list: returns empty for user with no projects ──────────────────────

  it('list: returns empty for user with no projects', async () => {
    await insertUser(pool, 'u-empty')
    const result = await service.list('u-empty')
    expect(result).toEqual([])
  })

  // ── 7. update: updates name correctly ────────────────────────────────────

  it('update: updates name correctly', async () => {
    await insertUser(pool, 'u-upd')
    const p = await service.create({ name: 'Old Name', devServerId: FAKE_DEV_SERVER_ID, repoPath: '/r', createdBy: 'u-upd' })
    await service.update(p.id, { name: 'New Name' }, 'u-upd')
    const updated = await service.get(p.id)
    expect(updated?.name).toBe('New Name')
  })

  // ── 8. update: updates visibility correctly ───────────────────────────────

  it('update: updates visibility correctly', async () => {
    await insertUser(pool, 'u-vis')
    const p = await service.create({ name: 'V', devServerId: FAKE_DEV_SERVER_ID, repoPath: '/r', createdBy: 'u-vis' })
    await service.update(p.id, { visibility: 'private' }, 'u-vis')
    const updated = await service.get(p.id)
    expect(updated?.visibility).toBe('private')
  })

  // ── 9. delete: removes project ────────────────────────────────────────────

  it('delete: removes project', async () => {
    await insertUser(pool, 'u-del')
    const p = await service.create({ name: 'ToDelete', devServerId: FAKE_DEV_SERVER_ID, repoPath: '/r', createdBy: 'u-del' })
    await service.delete(p.id, 'u-del')
    const result = await service.get(p.id)
    expect(result).toBeNull()
  })

  // ── 10. addMember: adds with role 'member' ────────────────────────────────

  it('addMember: adds member with given role', async () => {
    await insertUser(pool, 'u-owner')
    await insertUser(pool, 'u-mem')
    const p = await service.create({ name: 'Team', devServerId: FAKE_DEV_SERVER_ID, repoPath: '/r', createdBy: 'u-owner' })
    await service.addMember(p.id, 'u-mem', 'member')
    const m = await service.getMember(p.id, 'u-mem')
    expect(m?.role).toBe('member')
  })

  // ── 11. addMember: upserts when called twice (last role wins) ─────────────

  it('addMember: upserts — second call updates role', async () => {
    await insertUser(pool, 'u-own2')
    await insertUser(pool, 'u-m2')
    const p = await service.create({ name: 'T2', devServerId: FAKE_DEV_SERVER_ID, repoPath: '/r', createdBy: 'u-own2' })
    await service.addMember(p.id, 'u-m2', 'viewer')
    await service.addMember(p.id, 'u-m2', 'member') // upsert
    const m = await service.getMember(p.id, 'u-m2')
    expect(m?.role).toBe('member')
  })

  // ── 12. removeMember: removes correctly ───────────────────────────────────

  it('removeMember: removes member', async () => {
    await insertUser(pool, 'u-own3')
    await insertUser(pool, 'u-m3')
    const p = await service.create({ name: 'T3', devServerId: FAKE_DEV_SERVER_ID, repoPath: '/r', createdBy: 'u-own3' })
    await service.addMember(p.id, 'u-m3', 'member')
    await service.removeMember(p.id, 'u-m3')
    const m = await service.getMember(p.id, 'u-m3')
    expect(m).toBeNull()
  })

  // ── 13. updateMemberRole: changes role ────────────────────────────────────

  it('updateMemberRole: changes role from member to viewer', async () => {
    await insertUser(pool, 'u-own4')
    await insertUser(pool, 'u-m4')
    const p = await service.create({ name: 'T4', devServerId: FAKE_DEV_SERVER_ID, repoPath: '/r', createdBy: 'u-own4' })
    await service.addMember(p.id, 'u-m4', 'member')
    await service.updateMemberRole(p.id, 'u-m4', 'viewer')
    const m = await service.getMember(p.id, 'u-m4')
    expect(m?.role).toBe('viewer')
  })

  // ── 14. assertAccess: returns member for valid access ─────────────────────

  it('assertAccess: returns member for valid member', async () => {
    await insertUser(pool, 'u-own5')
    const p = await service.create({ name: 'T5', devServerId: FAKE_DEV_SERVER_ID, repoPath: '/r', createdBy: 'u-own5' })
    const member = await service.assertAccess(p.id, 'u-own5')
    expect(member.role).toBe('owner')
    expect(member.userId).toBe('u-own5')
  })

  // ── 15. assertAccess: throws PROJECT_ACCESS_DENIED for non-member ──────────

  it('assertAccess: throws PROJECT_ACCESS_DENIED for non-member', async () => {
    await insertUser(pool, 'u-own6')
    const p = await service.create({ name: 'T6', devServerId: FAKE_DEV_SERVER_ID, repoPath: '/r', createdBy: 'u-own6' })
    await expect(service.assertAccess(p.id, 'stranger')).rejects.toThrow('PROJECT_ACCESS_DENIED')
  })

  // ── 16. getMembers: returns all project members ───────────────────────────

  it('getMembers: returns all members', async () => {
    await insertUser(pool, 'u-own7')
    await insertUser(pool, 'u-m7')
    const p = await service.create({ name: 'T7', devServerId: FAKE_DEV_SERVER_ID, repoPath: '/r', createdBy: 'u-own7' })
    await service.addMember(p.id, 'u-m7', 'member')
    const members = await service.getMembers(p.id)
    expect(members.length).toBe(2)
    expect(members.map(m => m.userId).sort()).toEqual(['u-m7', 'u-own7'].sort())
  })

  // ── 17. delete: cascades to members ──────────────────────────────────────

  it('delete: cascades to members (no orphaned members)', async () => {
    await insertUser(pool, 'u-own8')
    const p = await service.create({ name: 'T8', devServerId: FAKE_DEV_SERVER_ID, repoPath: '/r', createdBy: 'u-own8' })
    await service.delete(p.id, 'u-own8')
    // project is gone, members should be gone (cascade)
    const members = await service.getMembers(p.id)
    expect(members).toEqual([])
  })
})

// ── CR-TRACE-015: ProjectService.create() tracing (TASK-BE-015.5) ──────────

describe('ProjectService.create tracing (CR-TRACE-015)', () => {
  let pool: SqliteSingleConnectionPool
  let service: ProjectService

  beforeEach(async () => {
    const setup = await makeService()
    pool = setup.pool
    service = setup.service
  })

  afterEach(async () => {
    await pool.destroy().catch(() => {})
  })

  function captureTraceEvents(): { events: TraceEvent[]; stop: () => void } {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    return { events, stop: unregister }
  }

  it('create() success → profile:projectRoute ok({ op: "create", projectId })', async () => {
    await insertUser(pool, 'u-trace-1')
    const { events, stop } = captureTraceEvents()

    const project = await service.create({
      name: 'Traced Project', devServerId: FAKE_DEV_SERVER_ID, repoPath: '/repo', createdBy: 'u-trace-1'
    })
    stop()

    const okEvent = events.find((e) => e.flow === 'profile:projectRoute' && e.level === 'ok')
    expect(okEvent?.fields).toMatchObject({ op: 'create', projectId: project.id })
  })

  it('create() with a non-existent devServerId → span.fail("DEV_SERVER_NOT_FOUND", { op: "create" }), no project created', async () => {
    const dsm = makeMockDSM(false)
    const noServerService = new ProjectService(pool, dsm)
    await insertUser(pool, 'u-trace-2')
    const { events, stop } = captureTraceEvents()

    await expect(
      noServerService.create({ name: 'X', devServerId: 'missing-dev-server', repoPath: '/repo', createdBy: 'u-trace-2' })
    ).rejects.toThrow('DEV_SERVER_NOT_FOUND')
    stop()

    const failEvent = events.find((e) => e.flow === 'profile:projectRoute' && e.level === 'fail')
    expect(failEvent?.fields).toMatchObject({ op: 'create' })
    expect(failEvent?.fields.err).toContain('DEV_SERVER_NOT_FOUND')
  })

  it('create() emits a validateDevServer step before ok()', async () => {
    await insertUser(pool, 'u-trace-3')
    const { events, stop } = captureTraceEvents()

    await service.create({ name: 'Y', devServerId: FAKE_DEV_SERVER_ID, repoPath: '/repo', createdBy: 'u-trace-3' })
    stop()

    const spanEvents = events.filter((e) => e.flow === 'profile:projectRoute')
    expect(spanEvents.map((e) => (e.level === 'step' ? e.label : e.level))).toEqual([
      'start',
      'validateDevServer',
      'ok'
    ])
  })
})
