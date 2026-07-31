/**
 * Tests for TaskAIPlanner (TDD-18) — T06
 *
 * Strategy:
 * - decompose(): mock relay.call() + providerService (AIProviderService interface)
 * - applyDecomposition(): in-memory SQLite — verifies real DB persistence
 *
 * Actual API:
 *   constructor(taskService, providerService: AIProviderService, router: ProjectServerRouter)
 *   decompose(taskId, projectId, userId) → OrcaTask[] (proposals, NOT persisted)
 *   applyDecomposition(taskId, subtasks: Partial<OrcaTask>[]) → OrcaTask[] (persisted)
 *
 * Findings:
 *   - TaskService.create() ALWAYS stores status='backlog' (hardcoded in SQL)
 *   - projectId FK constraint: do NOT pass projectId unless a project record exists
 *   - TaskAIPlanner.decompose() calls this.router.getRelayForProject(projectId, userId)
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { SqliteSingleConnectionPool } from '../../db/sqlite/sqlite-pool'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations'
import { TaskService } from '../TaskService'
import { TaskDAGValidator } from '../TaskDAGValidator'
import { TaskAIPlanner } from '../TaskAIPlanner'

// ── Setup ─────────────────────────────────────────────────────────────────────

async function makeServices() {
  const pool = new SqliteSingleConnectionPool(':memory:')
  await pool.withConnection(async (db) => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
  })
  const validator = new TaskDAGValidator(pool)
  const taskService = new TaskService(pool, validator)
  return { pool, taskService }
}

async function insertUser(pool: SqliteSingleConnectionPool, userId: string): Promise<void> {
  await pool.withConnection((db) =>
    db.query(
      'INSERT INTO orca_users (id, email, name, role, provider, created_at) VALUES (?, ?, ?, ?, ?, ?)',
      [userId, `${userId}@test.com`, userId, 'developer', 'none', Date.now()]
    )
  )
}

// ── Mock helpers ──────────────────────────────────────────────────────────────

function makeMockProviderService() {
  return {
    getAccount: vi.fn().mockResolvedValue({ id: 'acct-001', provider: 'anthropic', model: 'claude-3-5-sonnet' }),
    listAccounts: vi.fn().mockResolvedValue([]),
    createAccount: vi.fn(),
    updateAccount: vi.fn(),
    deleteAccount: vi.fn(),
  }
}

function makeMockProjectRouter(relayCallResponse: unknown = { content: '[]' }) {
  const mockRelay = { call: vi.fn().mockResolvedValue(relayCallResponse) }
  const router = {
    getProject: vi.fn().mockResolvedValue({ id: 'proj-001', devServerId: 'srv-001', repoPath: '/repo' }),
    getRelayForProject: vi.fn().mockResolvedValue(mockRelay),
    _mockRelay: mockRelay,
  }
  return router
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('TaskAIPlanner', () => {
  let taskService: TaskService
  let pool: SqliteSingleConnectionPool

  beforeEach(async () => {
    ;({ pool, taskService } = await makeServices())
    await insertUser(pool, 'reporter-001')
    await insertUser(pool, 'user-001')
  })

  // ── decompose — prompt building ───────────────────────────────────────────────
  describe('decompose — prompt building', () => {
    it('relay.call is invoked during decompose', async () => {
      const mockRouter = makeMockProjectRouter({ content: '[]' })
      const planner = new TaskAIPlanner(taskService, makeMockProviderService() as any, mockRouter as any)

      const task = await taskService.create({
        title: 'Build authentication system',
        type: 'task', status: 'backlog', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team',
        // NO projectId — avoids FK constraint on orca_projects
      })

      await planner.decompose(task.id, 'proj-001', 'user-001')
      expect(mockRouter._mockRelay.call).toHaveBeenCalled()
    })

    it('relay is called with ai.complete method name', async () => {
      const mockRouter = makeMockProjectRouter({ content: '[]' })
      const planner = new TaskAIPlanner(taskService, makeMockProviderService() as any, mockRouter as any)

      const task = await taskService.create({
        title: 'Auth task', type: 'task', status: 'backlog', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team',
      })

      await planner.decompose(task.id, 'proj-001', 'user-001')
      const callMethod = mockRouter._mockRelay.call.mock.calls[0][0]
      expect(callMethod).toBe('ai.complete')
    })

    it('prompt param includes task title', async () => {
      const mockRouter = makeMockProjectRouter({ content: '[]' })
      const planner = new TaskAIPlanner(taskService, makeMockProviderService() as any, mockRouter as any)

      const task = await taskService.create({
        title: 'Build authentication system',
        type: 'task', status: 'backlog', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team',
      })

      await planner.decompose(task.id, 'proj-001', 'user-001')
      const prompt = mockRouter._mockRelay.call.mock.calls[0][1].prompt
      expect(prompt).toContain('Build authentication system')
    })

    it('prompt includes task description when present', async () => {
      const mockRouter = makeMockProjectRouter({ content: '[]' })
      const planner = new TaskAIPlanner(taskService, makeMockProviderService() as any, mockRouter as any)

      const task = await taskService.create({
        title: 'Feature X',
        description: 'This feature enables Y capability',
        type: 'task', status: 'backlog', priority: 'medium',
        reporterId: 'reporter-001', visibility: 'team',
      })

      await planner.decompose(task.id, 'proj-001', 'user-001')
      const prompt = mockRouter._mockRelay.call.mock.calls[0][1].prompt
      expect(prompt).toContain('This feature enables Y capability')
    })

    it('getRelayForProject is called with correct projectId', async () => {
      const mockRouter = makeMockProjectRouter({ content: '[]' })
      const planner = new TaskAIPlanner(taskService, makeMockProviderService() as any, mockRouter as any)

      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })

      await planner.decompose(task.id, 'proj-test', 'user-001')
      expect(mockRouter.getRelayForProject).toHaveBeenCalledWith('proj-test', 'user-001')
    })
  })

  // ── decompose — response parsing ─────────────────────────────────────────────
  describe('decompose — response parsing', () => {
    it('parses valid JSON array → returns OrcaTask proposals', async () => {
      const subtasks = JSON.stringify([
        { title: 'Setup DB', type: 'subtask', estimatedHours: 1, description: 'Init schema' },
        { title: 'API endpoint', type: 'subtask', estimatedHours: 3, description: 'REST layer' },
      ])
      const mockRouter = makeMockProjectRouter({ content: subtasks })
      const planner = new TaskAIPlanner(taskService, makeMockProviderService() as any, mockRouter as any)

      const task = await taskService.create({
        title: 'Epic Feature', type: 'task', status: 'backlog', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team',
      })

      const proposals = await planner.decompose(task.id, 'proj-001', 'user-001')
      expect(proposals).toHaveLength(2)
      expect(proposals[0].title).toBe('Setup DB')
      expect(proposals[0].type).toBe('subtask')
    })

    it('parses JSON from text field as fallback', async () => {
      const subtasks = JSON.stringify([
        { title: 'Only subtask', type: 'subtask', estimatedHours: 2 },
      ])
      const mockRouter = makeMockProjectRouter({ text: subtasks })
      const planner = new TaskAIPlanner(taskService, makeMockProviderService() as any, mockRouter as any)

      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })

      const proposals = await planner.decompose(task.id, 'proj-001', 'user-001')
      expect(proposals).toHaveLength(1)
    })

    it('returns [] when AI returns non-JSON text', async () => {
      const mockRouter = makeMockProjectRouter({ content: 'I cannot help with that' })
      const planner = new TaskAIPlanner(taskService, makeMockProviderService() as any, mockRouter as any)

      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })

      const proposals = await planner.decompose(task.id, 'proj-001', 'user-001')
      expect(proposals).toEqual([])
    })

    it('returns [] when JSON is valid but not an array', async () => {
      const mockRouter = makeMockProjectRouter({ content: '{"key": "value"}' })
      const planner = new TaskAIPlanner(taskService, makeMockProviderService() as any, mockRouter as any)

      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })

      const proposals = await planner.decompose(task.id, 'proj-001', 'user-001')
      expect(proposals).toEqual([])
    })

    it('returned proposals have parentId set to parent task id', async () => {
      const subtasks = JSON.stringify([
        { title: 'Child task', type: 'subtask', estimatedHours: 1 },
      ])
      const mockRouter = makeMockProjectRouter({ content: subtasks })
      const planner = new TaskAIPlanner(taskService, makeMockProviderService() as any, mockRouter as any)

      const task = await taskService.create({
        title: 'Parent', type: 'task', status: 'backlog', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team',
      })

      const proposals = await planner.decompose(task.id, 'proj-001', 'user-001')
      expect(proposals[0].parentId).toBe(task.id)
    })
  })

  // ── applyDecomposition ────────────────────────────────────────────────────────
  describe('applyDecomposition', () => {
    it('creates subtasks as children of parent task in DB', async () => {
      const planner = new TaskAIPlanner(taskService, makeMockProviderService() as any, makeMockProjectRouter() as any)

      const parent = await taskService.create({
        title: 'Parent', type: 'task', status: 'backlog', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team',
      })

      const subtasks = [
        { title: 'Sub A', type: 'subtask' as const, parentId: parent.id, visibility: 'team' as const },
        { title: 'Sub B', type: 'subtask' as const, parentId: parent.id, visibility: 'team' as const },
      ]

      const created = await planner.applyDecomposition(parent.id, subtasks)
      expect(created).toHaveLength(2)
      expect(created.every(t => t.parentId === parent.id)).toBe(true)
    })

    it('persisted tasks are retrievable via taskService.getChildren', async () => {
      const planner = new TaskAIPlanner(taskService, makeMockProviderService() as any, makeMockProjectRouter() as any)

      const parent = await taskService.create({
        title: 'Parent', type: 'task', status: 'backlog', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team',
      })

      await planner.applyDecomposition(parent.id, [
        { title: 'Persisted Sub', type: 'subtask' as const, parentId: parent.id, visibility: 'team' as const },
      ])

      const children = await taskService.getChildren(parent.id)
      expect(children.length).toBe(1)
      expect(children[0].title).toBe('Persisted Sub')
    })

    it('throws TASK_NOT_FOUND when parent does not exist', async () => {
      const planner = new TaskAIPlanner(taskService, makeMockProviderService() as any, makeMockProjectRouter() as any)
      await expect(planner.applyDecomposition('nonexistent', [])).rejects.toThrow('TASK_NOT_FOUND')
    })
  })

  // ── error cases ────────────────────────────────────────────────────────────────
  describe('error cases', () => {
    it('throws TASK_NOT_FOUND when decomposing unknown task', async () => {
      const planner = new TaskAIPlanner(taskService, makeMockProviderService() as any, makeMockProjectRouter() as any)
      await expect(planner.decompose('bad-id', 'proj-001', 'user-001')).rejects.toThrow('TASK_NOT_FOUND')
    })
  })
})
