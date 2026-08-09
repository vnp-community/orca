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
import { SqliteSingleConnectionPool } from '../../db/sqlite/sqlite-pool'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations'
import { TaskService } from '../TaskService'
import { TaskDAGValidator } from '../TaskDAGValidator'
import { TaskAgentExecutor } from '../TaskAgentExecutor'
import { registerTraceSink, type TraceEvent } from '../../../shared/trace'

function captureTraceEvents(): { events: TraceEvent[]; stop: () => void } {
  const events: TraceEvent[] = []
  const unregister = registerTraceSink((e) => events.push(e))
  return { events, stop: unregister }
}

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

  // ── executeTask tracing (TASK-BE-018.5) ──────────────────────────────────────
  describe('executeTask tracing', () => {
    it('permission denied → span.fail(TASK_PERMISSION_DENIED), NO step(agent-spawn)', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const noPermGrantService = makeMockGrantService(null)
      const executor = new TaskAgentExecutor(taskService, makeMockSpawner() as any, noPermGrantService as any)

      const { events, stop } = captureTraceEvents()
      await expect(executor.executeTask({ ...EXEC_PARAMS, taskId: task.id })).rejects.toThrow('TASK_PERMISSION_DENIED')
      stop()

      const spanEvents = events.filter((e) => e.flow === 'taskGraph:execute')
      expect(spanEvents.some((e) => e.level === 'step' && e.label === 'agent-spawn')).toBe(false)
      const failEvent = spanEvents.find((e) => e.level === 'fail')
      expect(failEvent?.fields.err).toContain('TASK_PERMISSION_DENIED')
      // Only ever fails once for this call — the outer catch re-throws without double-failing.
      expect(spanEvents.filter((e) => e.level === 'fail')).toHaveLength(1)
    })

    it('spawn succeeds → span.ok({status: "review"})', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const executor = new TaskAgentExecutor(taskService, makeMockSpawner() as any, makeMockGrantService() as any)

      const { events, stop } = captureTraceEvents()
      await executor.executeTask({ ...EXEC_PARAMS, taskId: task.id })
      stop()

      const spanEvents = events.filter((e) => e.flow === 'taskGraph:execute')
      const okEvent = spanEvents.find((e) => e.level === 'ok')
      expect(okEvent?.fields.status).toBe('review')
      expect(spanEvents.some((e) => e.level === 'step' && e.label === 'agent-spawn')).toBe(true)
    })

    it('spawn throws → span.fail(err, {status: "blocked"})', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const badSpawner = { spawn: vi.fn().mockRejectedValue(new Error('Spawn failed')) }
      const executor = new TaskAgentExecutor(taskService, badSpawner as any, makeMockGrantService() as any)

      const { events, stop } = captureTraceEvents()
      await expect(executor.executeTask({ ...EXEC_PARAMS, taskId: task.id })).rejects.toThrow('Spawn failed')
      stop()

      const spanEvents = events.filter((e) => e.flow === 'taskGraph:execute')
      const failEvent = spanEvents.find((e) => e.level === 'fail')
      expect(failEvent?.fields.status).toBe('blocked')
      expect(failEvent?.fields.err).toContain('Spawn failed')
    })

    it('forwards traceId: span.id into agentSpawner.spawn() options', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const spawner = makeMockSpawner()
      const executor = new TaskAgentExecutor(taskService, spawner as any, makeMockGrantService() as any)

      const { events, stop } = captureTraceEvents()
      await executor.executeTask({ ...EXEC_PARAMS, taskId: task.id })
      stop()

      const startEvent = events.find((e) => e.flow === 'taskGraph:execute' && e.level === 'start')
      const spawnOptions = spawner.spawn.mock.calls[0][0]
      expect(spawnOptions.traceId).toBe(startEvent?.id)
    })

    it('step order: permission-check happens before agent-spawn', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', status: 'backlog', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const executor = new TaskAgentExecutor(taskService, makeMockSpawner() as any, makeMockGrantService() as any)

      const { events, stop } = captureTraceEvents()
      await executor.executeTask({ ...EXEC_PARAMS, taskId: task.id })
      stop()

      const spanEvents = events.filter((e) => e.flow === 'taskGraph:execute' && e.level === 'step')
      expect(spanEvents.map((e) => e.label)).toEqual(['permission-check', 'agent-spawn'])
    })

    it('task not found → span.fail(TASK_NOT_FOUND), NO step(agent-spawn)', async () => {
      const executor = new TaskAgentExecutor(taskService, makeMockSpawner() as any, makeMockGrantService() as any)

      const { events, stop } = captureTraceEvents()
      await expect(executor.executeTask({ ...EXEC_PARAMS, taskId: 'unknown-id' })).rejects.toThrow('TASK_NOT_FOUND')
      stop()

      const spanEvents = events.filter((e) => e.flow === 'taskGraph:execute')
      expect(spanEvents.some((e) => e.level === 'step' && e.label === 'agent-spawn')).toBe(false)
      const failEvent = spanEvents.find((e) => e.level === 'fail')
      expect(failEvent?.fields.err).toContain('TASK_NOT_FOUND')
    })
  })
})
