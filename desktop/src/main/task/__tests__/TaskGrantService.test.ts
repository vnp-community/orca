/**
 * Tests for TaskGrantService (TDD-18) — T05
 *
 * Uses in-memory SQLite + ALL_MIGRATIONS (real DB, no mocks for pool).
 * Tests BFS ancestor grant resolution, cascade, scope matching.
 *
 * NOTE: Actual API uses grantPermission() (not grantAccess()),
 * and resolvePermission() returns TaskPermission | null (not assertPermission).
 */

import { describe, it, expect, beforeEach } from 'vitest'
import { SqliteSingleConnectionPool } from '../../db/sqlite/sqlite-pool'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations'
import { TaskService } from '../TaskService'
import { TaskDAGValidator } from '../TaskDAGValidator'
import { TaskGrantService } from '../TaskGrantService'
import { registerTraceSink, type TraceEvent } from '../../../shared/trace'

function captureTraceEvents(): { events: TraceEvent[]; stop: () => void } {
  const events: TraceEvent[] = []
  const unregister = registerTraceSink((e) => events.push(e))
  return { events, stop: unregister }
}

// ── Helpers ──────────────────────────────────────────────────────────────────

async function makeServices() {
  const pool = new SqliteSingleConnectionPool(':memory:')
  await pool.withConnection(async (db) => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
  })
  const validator = new TaskDAGValidator(pool)
  const taskService = new TaskService(pool, validator)
  const grantService = new TaskGrantService(pool, taskService)
  return { pool, taskService, grantService }
}

async function insertUser(pool: SqliteSingleConnectionPool, userId: string): Promise<void> {
  await pool.withConnection((db) =>
    db.query(
      'INSERT INTO orca_users (id, email, name, role, provider, created_at) VALUES (?, ?, ?, ?, ?, ?)',
      [userId, `${userId}@test.com`, userId, 'developer', 'none', Date.now()]
    )
  )
}

async function insertTeamMember(pool: SqliteSingleConnectionPool, userId: string, teamId: string): Promise<void> {
  await pool.withConnection((db) =>
    db.query(
      'INSERT INTO orca_team_members (user_id, team_id, added_at) VALUES (?, ?, ?)',
      [userId, teamId, Date.now()]
    )
  )
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('TaskGrantService', () => {
  let pool: SqliteSingleConnectionPool
  let taskService: TaskService
  let grantService: TaskGrantService

  beforeEach(async () => {
    ;({ pool, taskService, grantService } = await makeServices())
    await insertUser(pool, 'reporter-001')
    await insertUser(pool, 'assignee-001')
    await insertUser(pool, 'user-001')
    await insertUser(pool, 'user-002')
  })

  // ── Implicit behavior: reporter gets manage via explicit grant only ────────────
  // NOTE: TaskGrantService has NO implicit reporter/assignee grants in resolvePermission().
  // resolvePermission() only resolves DB grants. Test what the implementation actually does.
  describe('no implicit grant', () => {
    it('reporter returns null without explicit grant', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      // No explicit grant → null
      expect(await grantService.resolvePermission('reporter-001', task.id)).toBeNull()
    })

    it('assignee returns null without explicit grant', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', assigneeId: 'assignee-001', visibility: 'team',
      })
      // No explicit grant → null
      expect(await grantService.resolvePermission('assignee-001', task.id)).toBeNull()
    })
  })

  // ── Direct user grants ────────────────────────────────────────────────────────
  describe('direct user grant', () => {
    it('returns granted permission for direct user', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      await grantService.grantPermission({
        taskId: task.id, scope: 'user', scopeId: 'user-001',
        permission: 'view', grantedBy: 'reporter-001',
      })
      expect(await grantService.resolvePermission('user-001', task.id)).toBe('view')
    })

    it('higher permission wins over lower (execute > view)', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      await grantService.grantPermission({
        taskId: task.id, scope: 'user', scopeId: 'user-001',
        permission: 'view', grantedBy: 'reporter-001',
      })
      await grantService.grantPermission({
        taskId: task.id, scope: 'everyone',
        permission: 'execute', grantedBy: 'reporter-001',
      })
      // execute (4) > view (1) — everyone scope matches user-001 too
      expect(await grantService.resolvePermission('user-001', task.id)).toBe('execute')
    })

    it('grantPermission returns a string grant ID', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const grantId = await grantService.grantPermission({
        taskId: task.id, scope: 'user', scopeId: 'user-001',
        permission: 'comment', grantedBy: 'reporter-001',
      })
      expect(typeof grantId).toBe('string')
      expect(grantId.length).toBeGreaterThan(0)
    })
  })

  // ── Cascade grants (applyTree) ────────────────────────────────────────────────
  describe('ancestor cascade', () => {
    it('applyTree=true propagates grant to subtask', async () => {
      const parent = await taskService.create({
        title: 'Epic', type: 'epic', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const child = await taskService.create({
        title: 'Task', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team', parentId: parent.id,
      })
      await grantService.grantPermission({
        taskId: parent.id, scope: 'user', scopeId: 'user-001',
        permission: 'edit', applyTree: true, grantedBy: 'reporter-001',
      })
      expect(await grantService.resolvePermission('user-001', child.id)).toBe('edit')
    })

    it('applyTree=false does NOT propagate to subtask', async () => {
      const parent = await taskService.create({
        title: 'Epic', type: 'epic', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const child = await taskService.create({
        title: 'Task', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team', parentId: parent.id,
      })
      await grantService.grantPermission({
        taskId: parent.id, scope: 'user', scopeId: 'user-001',
        permission: 'edit', applyTree: false, grantedBy: 'reporter-001',
      })
      expect(await grantService.resolvePermission('user-001', child.id)).toBeNull()
    })
  })

  // ── Scope matching ────────────────────────────────────────────────────────────
  describe('team scope', () => {
    it('team grant matches user in team', async () => {
      await insertTeamMember(pool, 'user-001', 'team-alpha')
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      await grantService.grantPermission({
        taskId: task.id, scope: 'team', scopeId: 'team-alpha',
        permission: 'comment', grantedBy: 'reporter-001',
      })
      expect(await grantService.resolvePermission('user-001', task.id)).toBe('comment')
    })

    it('team grant does NOT match user NOT in team', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      await grantService.grantPermission({
        taskId: task.id, scope: 'team', scopeId: 'team-alpha',
        permission: 'comment', grantedBy: 'reporter-001',
      })
      expect(await grantService.resolvePermission('user-002', task.id)).toBeNull()
    })
  })

  describe('everyone scope (formerly company)', () => {
    it('everyone scope matches any user', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'company',
      })
      await grantService.grantPermission({
        taskId: task.id, scope: 'everyone',
        permission: 'view', grantedBy: 'reporter-001',
      })
      expect(await grantService.resolvePermission('user-001', task.id)).toBe('view')
      expect(await grantService.resolvePermission('user-002', task.id)).toBe('view')
    })
  })

  // ── No grant ──────────────────────────────────────────────────────────────────
  describe('no grant', () => {
    it('returns null when user has no grant at all', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      expect(await grantService.resolvePermission('user-001', task.id)).toBeNull()
    })
  })

  // ── revokeGrant ───────────────────────────────────────────────────────────────
  describe('revokeGrant', () => {
    it('revoked grant no longer resolves permission', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const grantId = await grantService.grantPermission({
        taskId: task.id, scope: 'user', scopeId: 'user-001',
        permission: 'edit', grantedBy: 'reporter-001',
      })
      expect(await grantService.resolvePermission('user-001', task.id)).toBe('edit')
      await grantService.revokeGrant(grantId)
      expect(await grantService.resolvePermission('user-001', task.id)).toBeNull()
    })
  })

  // ── listGrants ────────────────────────────────────────────────────────────────
  describe('listGrants', () => {
    it('returns all grants for a task', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      await grantService.grantPermission({ taskId: task.id, scope: 'user', scopeId: 'user-001', permission: 'view', grantedBy: 'reporter-001' })
      await grantService.grantPermission({ taskId: task.id, scope: 'user', scopeId: 'user-002', permission: 'comment', grantedBy: 'reporter-001' })
      const grants = await grantService.listGrants(task.id)
      expect(grants.length).toBe(2)
    })
  })

  // ── resolvePermission tracing (TASK-BE-018.3) ────────────────────────────────
  describe('resolvePermission tracing', () => {
    it('direct grant match → step(grant-match, {direct:true})', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      await grantService.grantPermission({
        taskId: task.id, scope: 'user', scopeId: 'user-001',
        permission: 'edit', grantedBy: 'reporter-001',
      })

      const { events, stop } = captureTraceEvents()
      await grantService.resolvePermission('user-001', task.id)
      stop()

      const spanEvents = events.filter((e) => e.flow === 'taskGraph:grantResolve')
      const matchStep = spanEvents.find((e) => e.level === 'step' && e.label === 'grant-match')
      expect(matchStep?.fields.direct).toBe(true)
      expect(spanEvents.some((e) => e.level === 'ok')).toBe(true)
    })

    it('ancestor-only match (applyTree) → step(grant-match, {direct:false})', async () => {
      const parent = await taskService.create({
        title: 'Epic', type: 'epic', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const child = await taskService.create({
        title: 'Task', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team', parentId: parent.id,
      })
      await grantService.grantPermission({
        taskId: parent.id, scope: 'user', scopeId: 'user-001',
        permission: 'edit', applyTree: true, grantedBy: 'reporter-001',
      })

      const { events, stop } = captureTraceEvents()
      await grantService.resolvePermission('user-001', child.id)
      stop()

      const spanEvents = events.filter((e) => e.flow === 'taskGraph:grantResolve')
      const matchStep = spanEvents.find((e) => e.level === 'step' && e.label === 'grant-match')
      expect(matchStep?.fields.direct).toBe(false)
    })

    it('no match → span.fail(NO_GRANT_FOUND), returns null', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })

      const { events, stop } = captureTraceEvents()
      const result = await grantService.resolvePermission('user-001', task.id)
      stop()

      expect(result).toBeNull()
      const spanEvents = events.filter((e) => e.flow === 'taskGraph:grantResolve')
      const failEvent = spanEvents.find((e) => e.level === 'fail')
      expect(failEvent?.fields.err).toContain('NO_GRANT_FOUND')
      expect(spanEvents.some((e) => e.level === 'step')).toBe(false)
    })

    it('emits exactly 1 step() per call — no per-candidate/per-grant noise', async () => {
      const parent = await taskService.create({
        title: 'Epic', type: 'epic', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const child = await taskService.create({
        title: 'Task', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team', parentId: parent.id,
      })
      // Multiple grants across multiple candidates (task itself + ancestor) —
      // the nested candidates × grants loop must NOT emit a step per iteration.
      await grantService.grantPermission({
        taskId: child.id, scope: 'user', scopeId: 'user-001',
        permission: 'view', grantedBy: 'reporter-001',
      })
      await grantService.grantPermission({
        taskId: child.id, scope: 'everyone',
        permission: 'comment', grantedBy: 'reporter-001',
      })
      await grantService.grantPermission({
        taskId: parent.id, scope: 'user', scopeId: 'user-001',
        permission: 'edit', applyTree: true, grantedBy: 'reporter-001',
      })

      const { events, stop } = captureTraceEvents()
      await grantService.resolvePermission('user-001', child.id)
      stop()

      const stepEvents = events.filter((e) => e.flow === 'taskGraph:grantResolve' && e.level === 'step')
      expect(stepEvents).toHaveLength(1)
    })
  })
})
