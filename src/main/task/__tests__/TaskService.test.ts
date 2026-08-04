/**
 * Tests for TaskService (TDD-18) — T04
 *
 * Uses in-memory SQLite + ALL_MIGRATIONS (real DB, no mocks).
 * Pattern: same as ProjectService.test.ts
 */

import { describe, it, expect, beforeEach } from 'vitest'
import { SqliteSingleConnectionPool } from '../../db/sqlite/sqlite-pool'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations'
import { TaskService } from '../TaskService'
import { TaskDAGValidator } from '../TaskDAGValidator'
import { registerTraceSink, type TraceEvent } from '../../../shared/trace'

function captureTraceEvents(): { events: TraceEvent[]; stop: () => void } {
  const events: TraceEvent[] = []
  const unregister = registerTraceSink((e) => events.push(e))
  return { events, stop: unregister }
}

// ── Helpers ──────────────────────────────────────────────────────────────────

async function makeService(): Promise<{ pool: SqliteSingleConnectionPool; service: TaskService }> {
  const pool = new SqliteSingleConnectionPool(':memory:')
  await pool.withConnection(async (db) => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
  })
  const validator = new TaskDAGValidator(pool)
  const service = new TaskService(pool, validator)
  return { pool, service }
}

async function insertUser(pool: SqliteSingleConnectionPool, userId: string): Promise<void> {
  await pool.withConnection((db) =>
    db.query(
      'INSERT INTO orca_users (id, email, name, role, provider, created_at) VALUES (?, ?, ?, ?, ?, ?)',
      [userId, `${userId}@test.com`, userId, 'developer', 'none', Date.now()]
    )
  )
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('TaskService', () => {
  let pool: SqliteSingleConnectionPool
  let service: TaskService

  beforeEach(async () => {
    ;({ pool, service } = await makeService())
    await insertUser(pool, 'user-001')
    await insertUser(pool, 'user-002')
  })

  // ── create ──────────────────────────────────────────────────────────────────
  describe('create', () => {
    it('stores task with correct title and type', async () => {
      const task = await service.create({
        title: 'My Task',
        type: 'task',
        status: 'backlog',
        priority: 'medium',
        reporterId: 'user-001',
        visibility: 'team',
      })
      expect(task.id).toBeDefined()
      expect(task.title).toBe('My Task')
      expect(task.type).toBe('task')
    })

    it('defaults progressPercent to 0', async () => {
      const task = await service.create({
        title: 'Zero progress',
        type: 'task',
        status: 'backlog',
        priority: 'low',
        reporterId: 'user-001',
        visibility: 'team',
      })
      expect(task.progressPercent).toBe(0)
    })

    it('stores parentId when provided', async () => {
      const parent = await service.create({
        title: 'Epic',
        type: 'epic',
        status: 'backlog',
        priority: 'high',
        reporterId: 'user-001',
        visibility: 'team',
      })
      const child = await service.create({
        title: 'Story',
        type: 'story',
        status: 'backlog',
        priority: 'high',
        reporterId: 'user-001',
        visibility: 'team',
        parentId: parent.id,
      })
      expect(child.parentId).toBe(parent.id)
    })

    it('assigns a unique UUID id', async () => {
      const a = await service.create({ title: 'A', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      const b = await service.create({ title: 'B', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      expect(a.id).not.toBe(b.id)
    })
  })

  // ── get / update / delete ────────────────────────────────────────────────────
  describe('get + update + delete', () => {
    it('get returns task by id', async () => {
      const task = await service.create({ title: 'T', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      const fetched = await service.get(task.id)
      expect(fetched?.id).toBe(task.id)
    })

    it('get returns null for unknown id', async () => {
      expect(await service.get('nonexistent')).toBeNull()
    })

    it('update changes status', async () => {
      const task = await service.create({ title: 'T', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      await service.update(task.id, { status: 'in_progress' })
      const updated = await service.get(task.id)
      expect(updated?.status).toBe('in_progress')
    })

    it('update changes title', async () => {
      const task = await service.create({ title: 'Old', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      await service.update(task.id, { title: 'New Title' })
      const updated = await service.get(task.id)
      expect(updated?.title).toBe('New Title')
    })

    it('delete removes task', async () => {
      const task = await service.create({ title: 'Del', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      await service.delete(task.id)
      expect(await service.get(task.id)).toBeNull()
    })
  })

  // ── tree operations ──────────────────────────────────────────────────────────
  describe('getChildren', () => {
    it('returns direct children only', async () => {
      const parent = await service.create({ title: 'P', type: 'epic', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      const child1 = await service.create({ title: 'C1', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team', parentId: parent.id })
      const child2 = await service.create({ title: 'C2', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team', parentId: parent.id })
      const children = await service.getChildren(parent.id)
      expect(children.map(c => c.id).sort()).toEqual([child1.id, child2.id].sort())
    })

    it('returns empty array when no children', async () => {
      const leaf = await service.create({ title: 'Leaf', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      const children = await service.getChildren(leaf.id)
      expect(children).toEqual([])
    })
  })

  describe('getAncestors', () => {
    it('returns ancestor chain (root first, via unshift)', async () => {
      const grandparent = await service.create({ title: 'GP', type: 'epic', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      const parent = await service.create({ title: 'P', type: 'story', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team', parentId: grandparent.id })
      const child = await service.create({ title: 'C', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team', parentId: parent.id })
      const ancestors = await service.getAncestors(child.id)
      expect(ancestors.length).toBe(2)
      // Implementation uses unshift → root is first, parent is last
      expect(ancestors.map(a => a.id)).toContain(grandparent.id)
      expect(ancestors.map(a => a.id)).toContain(parent.id)
    })

    it('returns empty array for root task', async () => {
      const root = await service.create({ title: 'Root', type: 'epic', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      expect(await service.getAncestors(root.id)).toEqual([])
    })
  })

  // ── dependency edges ─────────────────────────────────────────────────────────
  describe('addEdge', () => {
    it('inserts edge when no cycle', async () => {
      const a = await service.create({ title: 'A', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      const b = await service.create({ title: 'B', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      await expect(service.addEdge(a.id, b.id, 'depends_on')).resolves.toBeUndefined()
    })

    it('throws TASK_DAG_CYCLE when adding creates cycle', async () => {
      const a = await service.create({ title: 'A', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      const b = await service.create({ title: 'B', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      await service.addEdge(a.id, b.id, 'depends_on')
      await expect(service.addEdge(b.id, a.id, 'depends_on')).rejects.toThrow('TASK_DAG_CYCLE')
    })

    it('getDependencies returns upstream tasks', async () => {
      const a = await service.create({ title: 'A', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      const b = await service.create({ title: 'B', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      await service.addEdge(b.id, a.id, 'depends_on') // b depends on a
      const deps = await service.getDependencies(b.id)
      expect(deps.some(d => d.task.id === a.id)).toBe(true)
    })
  })

  // ── addEdge tracing (TASK-BE-018.1) ──────────────────────────────────────────
  describe('addEdge tracing', () => {
    it('valid edge → step(cycle-check, {wouldCycle:false}) then ok()', async () => {
      const { events, stop } = captureTraceEvents()
      const a = await service.create({ title: 'A', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      const b = await service.create({ title: 'B', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })

      await service.addEdge(a.id, b.id, 'depends_on')
      stop()

      const spanEvents = events.filter((e) => e.flow === 'taskGraph:addEdge')
      const stepEvent = spanEvents.find((e) => e.level === 'step' && e.label === 'cycle-check')
      expect(stepEvent?.fields.wouldCycle).toBe(false)
      expect(spanEvents.some((e) => e.level === 'ok')).toBe(true)
      expect(spanEvents.some((e) => e.level === 'fail')).toBe(false)
    })

    it('cycle-creating edge → span.fail(TASK_DEPENDENCY_CYCLE), no INSERT runs', async () => {
      const a = await service.create({ title: 'A', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      const b = await service.create({ title: 'B', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      await service.addEdge(a.id, b.id, 'depends_on')

      const { events, stop } = captureTraceEvents()
      await expect(service.addEdge(b.id, a.id, 'depends_on')).rejects.toThrow('TASK_DAG_CYCLE')
      stop()

      const spanEvents = events.filter((e) => e.flow === 'taskGraph:addEdge')
      const failEvent = spanEvents.find((e) => e.level === 'fail')
      expect(failEvent?.fields.err).toContain('TASK_DEPENDENCY_CYCLE')
      expect(spanEvents.some((e) => e.level === 'ok')).toBe(false)

      // No new edge was inserted by the rejected addEdge() call — b has zero outgoing edges
      // (only the original a→b exists, which is a's outgoing edge, not b's).
      const deps = await service.getDependencies(b.id)
      expect(deps).toHaveLength(0)
    })
  })

  // ── progress calculation ─────────────────────────────────────────────────────
  describe('recalculateProgress', () => {
    // NOTE: TaskService.create() always stores status='backlog' (hardcoded in SQL).
    // Must use service.update() to set desired status before calling recalculateProgress.

    it('leaf task with status done → 100', async () => {
      const task = await service.create({ title: 'Done', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      await service.update(task.id, { status: 'done' })
      const progress = await service.recalculateProgress(task.id)
      expect(progress).toBe(100)
    })

    it('leaf task with status in_progress → 40', async () => {
      const task = await service.create({ title: 'WIP', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      await service.update(task.id, { status: 'in_progress' })
      expect(await service.recalculateProgress(task.id)).toBe(40)
    })

    it('leaf task with status backlog → 0', async () => {
      const task = await service.create({ title: 'Todo', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      expect(await service.recalculateProgress(task.id)).toBe(0)
    })

    it('parent averages children progress', async () => {
      const parent = await service.create({ title: 'P', type: 'epic', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      const c1 = await service.create({ title: 'C1', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team', parentId: parent.id })
      await service.create({ title: 'C2', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team', parentId: parent.id })
      await service.update(c1.id, { status: 'done' })
      // done=100, backlog=0 → avg=50
      expect(await service.recalculateProgress(parent.id)).toBe(50)
    })
  })

  // ── list with filters ────────────────────────────────────────────────────────
  describe('list', () => {
    it('filters by assigneeId', async () => {
      await service.create({ title: 'A1', type: 'task', status: 'todo', priority: 'low', reporterId: 'user-001', visibility: 'team', assigneeId: 'user-001' })
      await service.create({ title: 'A2', type: 'task', status: 'todo', priority: 'low', reporterId: 'user-001', visibility: 'team', assigneeId: 'user-002' })
      const results = await service.list({ assigneeId: 'user-001' })
      expect(results.every(t => t.assigneeId === 'user-001')).toBe(true)
    })

    it('filters by single status', async () => {
      await service.create({ title: 'T1', type: 'task', status: 'todo', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      await service.create({ title: 'T2', type: 'task', status: 'done', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      const todo = await service.list({ status: 'todo' })
      expect(todo.every(t => t.status === 'todo')).toBe(true)
    })

    it('filters by type', async () => {
      await service.create({ title: 'Epic1', type: 'epic', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      await service.create({ title: 'Task1', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      const epics = await service.list({ type: 'epic' })
      expect(epics.every(t => t.type === 'epic')).toBe(true)
    })

    it('returns all tasks when no filter provided', async () => {
      await service.create({ title: 'X', type: 'task', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      await service.create({ title: 'Y', type: 'epic', status: 'backlog', priority: 'low', reporterId: 'user-001', visibility: 'team' })
      const all = await service.list()
      expect(all.length).toBeGreaterThanOrEqual(2)
    })
  })
})
