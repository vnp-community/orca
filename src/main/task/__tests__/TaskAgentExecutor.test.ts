/**
 * Tests for TaskAgentExecutor (TDD-18) — T07
 *
 * Actual API:
 *   executeTask(params: ExecuteTaskParams): Promise<void>
 *   buildPrompt(task: OrcaTask): string
 *
 * NOTE: executeTask throws TASK_PERMISSION_DENIED (not TASK_ACCESS_DENIED).
 *       No buildPreamble — actual method is buildPrompt.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { SqliteSingleConnectionPool } from '../../db/sqlite/sqlite-pool'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations'
import { TaskService } from '../TaskService'
import { TaskDAGValidator } from '../TaskDAGValidator'
import { TaskAgentExecutor } from '../TaskAgentExecutor'

// ── Mock helpers ──────────────────────────────────────────────────────────────

function makeMockGrantService(permission: string | null = 'execute') {
  return {
    resolvePermission: vi.fn().mockResolvedValue(permission),
    grantPermission: vi.fn().mockResolvedValue('grant-id'),
    assertPermission: permission
      ? vi.fn().mockResolvedValue(undefined)
      : vi.fn().mockRejectedValue(new Error('TASK_PERMISSION_DENIED')),
  }
}

function makeMockSpawner(sessionId = 'session-abc') {
  return {
    spawn: vi.fn().mockResolvedValue({ sessionId }),
  }
}

async function makeTaskService() {
  const pool = new SqliteSingleConnectionPool(':memory:')
  await pool.withConnection(async (db) => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
  })
  const validator = new TaskDAGValidator(pool)
  const service = new TaskService(pool, validator)
  await pool.withConnection((db) =>
    db.query(
      'INSERT INTO orca_users (id, email, name, role, provider, created_at) VALUES (?, ?, ?, ?, ?, ?)',
      ['reporter-001', 'reporter@test.com', 'Reporter', 'developer', 'none', Date.now()]
    )
  )
  return { pool, service }
}

const EXEC_PARAMS = {
  projectId: 'proj-001',
  userId: 'user-001',
  worktreePath: '/repo',
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('TaskAgentExecutor', () => {
  let taskService: TaskService
  let pool: SqliteSingleConnectionPool

  beforeEach(async () => {
    const result = await makeTaskService()
    pool = result.pool
    taskService = result.service
  })

  // ── executeTask ───────────────────────────────────────────────────────────────
  describe('executeTask', () => {
    it('sets task status to in_progress before spawning', async () => {
      const task = await taskService.create({
        title: 'Implement X', type: 'task', status: 'backlog', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team',
      })

      const spawner = makeMockSpawner()
      const executor = new TaskAgentExecutor(taskService, spawner as any, makeMockGrantService() as any)

      const statuses: string[] = []
      const origUpdate = taskService.update.bind(taskService)
      vi.spyOn(taskService, 'update').mockImplementation(async (id, patch) => {
        if (patch.status) {statuses.push(patch.status)}
        return origUpdate(id, patch)
      })

      await executor.executeTask({ ...EXEC_PARAMS, taskId: task.id })
      expect(statuses).toContain('in_progress')
    })

    it('sets task status to review after successful spawn', async () => {
      const task = await taskService.create({
        title: 'Do Y', type: 'task', status: 'backlog', priority: 'medium',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const executor = new TaskAgentExecutor(taskService, makeMockSpawner() as any, makeMockGrantService() as any)
      await executor.executeTask({ ...EXEC_PARAMS, taskId: task.id })
      const updated = await taskService.get(task.id)
      expect(updated?.status).toBe('review')
    })

    it('sets task status to blocked when spawner throws', async () => {
      const task = await taskService.create({
        title: 'Fail task', type: 'task', status: 'backlog', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const badSpawner = { spawn: vi.fn().mockRejectedValue(new Error('Spawn failed')) }
      const executor = new TaskAgentExecutor(taskService, badSpawner as any, makeMockGrantService() as any)

      // executeTask re-throws, so catch it
      await expect(executor.executeTask({ ...EXEC_PARAMS, taskId: task.id })).rejects.toThrow()
      const updated = await taskService.get(task.id)
      expect(updated?.status).toBe('blocked')
    })

    it('throws TASK_NOT_FOUND for unknown taskId', async () => {
      const executor = new TaskAgentExecutor(taskService, makeMockSpawner() as any, makeMockGrantService() as any)
      await expect(executor.executeTask({ ...EXEC_PARAMS, taskId: 'unknown-id' })).rejects.toThrow('TASK_NOT_FOUND')
    })

    it('throws TASK_PERMISSION_DENIED when user lacks execute perm', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const noPermGrantService = makeMockGrantService(null)
      const executor = new TaskAgentExecutor(taskService, makeMockSpawner() as any, noPermGrantService as any)
      await expect(executor.executeTask({ ...EXEC_PARAMS, taskId: task.id })).rejects.toThrow('TASK_PERMISSION_DENIED')
    })

    it('spawner.spawn is called with correct projectId and userId', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const spawner = makeMockSpawner()
      const executor = new TaskAgentExecutor(taskService, spawner as any, makeMockGrantService('execute') as any)
      await executor.executeTask({ ...EXEC_PARAMS, taskId: task.id })
      expect(spawner.spawn).toHaveBeenCalledWith(expect.objectContaining({
        projectId: 'proj-001',
        userId: 'user-001',
      }))
    })
  })

  // ── buildPrompt ───────────────────────────────────────────────────────────────
  describe('buildPrompt', () => {
    it('includes task title in prompt', async () => {
      const task = await taskService.create({
        title: 'Implement Login Flow', type: 'task', status: 'backlog', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const executor = new TaskAgentExecutor(taskService, makeMockSpawner() as any, makeMockGrantService() as any)
      const prompt = executor.buildPrompt(task)
      expect(prompt).toContain('Implement Login Flow')
    })

    it('includes task description when present', async () => {
      const task = await taskService.create({
        title: 'T', description: 'Detailed description here',
        type: 'task', status: 'backlog', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const executor = new TaskAgentExecutor(taskService, makeMockSpawner() as any, makeMockGrantService() as any)
      const prompt = executor.buildPrompt(task)
      expect(prompt).toContain('Detailed description here')
    })

    it('uses promptTemplate when set', async () => {
      const task = await taskService.create({
        title: 'Templated Task', type: 'task', status: 'backlog', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team',
        promptTemplate: 'Execute: ${task.title} now',
      })
      const executor = new TaskAgentExecutor(taskService, makeMockSpawner() as any, makeMockGrantService() as any)
      const prompt = executor.buildPrompt(task)
      expect(prompt).toBe('Execute: Templated Task now')
    })

    it('formats task as markdown heading', async () => {
      const task = await taskService.create({
        title: 'Simple Task', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const executor = new TaskAgentExecutor(taskService, makeMockSpawner() as any, makeMockGrantService() as any)
      const prompt = executor.buildPrompt(task)
      expect(prompt).toContain('# Task:')
    })
  })

  // ── CR-TRACE-002: traceId propagation, no own span (TASK-BE-002.4) ────────
  describe('CR-TRACE-002 traceId propagation', () => {
    it('forwards traceId to agentSpawner.spawn() when provided', async () => {
      const task = await taskService.create({
        title: 'Traced task', type: 'task', status: 'backlog', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const spawner = makeMockSpawner()
      const executor = new TaskAgentExecutor(taskService, spawner as any, makeMockGrantService() as any)
      await executor.executeTask({ ...EXEC_PARAMS, taskId: task.id, traceId: 'resume-task-exec-1' })
      expect(spawner.spawn).toHaveBeenCalledWith(
        expect.objectContaining({ traceId: 'resume-task-exec-1' })
      )
    })

    it('spawn() is still called without a traceId field breaking when none is provided (backward compatible)', async () => {
      const task = await taskService.create({
        title: 'Untraced task', type: 'task', status: 'backlog', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const spawner = makeMockSpawner()
      const executor = new TaskAgentExecutor(taskService, spawner as any, makeMockGrantService() as any)
      await executor.executeTask({ ...EXEC_PARAMS, taskId: task.id })
      expect(spawner.spawn).toHaveBeenCalledWith(
        expect.objectContaining({ traceId: undefined })
      )
    })

    it('does not create its own tracer/span — no createTracer/Tracers.* reference in TaskAgentExecutor.ts source', () => {
      const source = readFileSync(join(__dirname, '../TaskAgentExecutor.ts'), 'utf-8')
      expect(source).not.toMatch(/createTracer\(/)
      expect(source).not.toMatch(/Tracers\./)
    })
  })
})
