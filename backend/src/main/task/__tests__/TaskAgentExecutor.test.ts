/**
 * Tests for TaskAgentExecutor (TDD-18 + Source→Plan→Execute pipeline, migration 0016)
 *
 * Covers:
 *   - (a) simple path: no subtasks, no dependency edges → single-agent spawn (existing behavior)
 *   - agentSessionId persisted onto orca_tasks.agent_session_id after a successful spawn
 *   - (b) complex path: subtasks and/or dependency edges → dispatchToOrchestration()
 *   - dispatchToOrchestration() guard when no TaskOrchestrationBridge is wired
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { SqliteSingleConnectionPool } from '../../db/sqlite/sqlite-pool'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations'
import { TaskService } from '../TaskService'
import { TaskDAGValidator } from '../TaskDAGValidator'
import { TaskAgentExecutor } from '../TaskAgentExecutor'
import type { TaskOrchestrationBridge } from '../TaskOrchestrationBridge'
import type { ProfileAwareAgentSpawner } from '../../project/ProfileAwareAgentSpawner'
import type { TaskGrantService } from '../TaskGrantService'
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
  }
}

function makeMockSpawner(sessionId = 'session-abc') {
  return {
    spawn: vi.fn().mockResolvedValue({ sessionId }),
  }
}

function makeMockOrchestrationBridge() {
  return {
    dispatch: vi.fn().mockResolvedValue({ taskRowId: 'row-root' }),
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

  beforeEach(async () => {
    const result = await makeTaskService()
    taskService = result.service
  })

  // ── (a) simple path — existing behavior ──────────────────────────────────────
  describe('executeTask — simple path (no subtasks, no dependency edges)', () => {
    it('sets task status to in_progress before spawning', async () => {
      const task = await taskService.create({
        title: 'Implement X', type: 'task', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team',
      })

      const spawner = makeMockSpawner()
      const executor = new TaskAgentExecutor(taskService, spawner as unknown as ProfileAwareAgentSpawner, makeMockGrantService() as unknown as TaskGrantService)

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
        title: 'Do Y', type: 'task', priority: 'medium',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const executor = new TaskAgentExecutor(taskService, makeMockSpawner() as unknown as ProfileAwareAgentSpawner, makeMockGrantService() as unknown as TaskGrantService)
      await executor.executeTask({ ...EXEC_PARAMS, taskId: task.id })
      const updated = await taskService.get(task.id)
      expect(updated?.status).toBe('review')
    })

    it('persists agentSessionId onto orca_tasks.agent_session_id after a successful spawn', async () => {
      const task = await taskService.create({
        title: 'Do Z', type: 'task', priority: 'medium',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const executor = new TaskAgentExecutor(
        taskService, makeMockSpawner('session-xyz') as unknown as ProfileAwareAgentSpawner, makeMockGrantService() as unknown as TaskGrantService
      )
      await executor.executeTask({ ...EXEC_PARAMS, taskId: task.id })
      const updated = await taskService.get(task.id)
      expect(updated?.agentSessionId).toBe('session-xyz')
    })

    it('sets task status to blocked when spawner throws (agentSessionId left unset)', async () => {
      const task = await taskService.create({
        title: 'Fail task', type: 'task', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const badSpawner = { spawn: vi.fn().mockRejectedValue(new Error('Spawn failed')) }
      const executor = new TaskAgentExecutor(taskService, badSpawner as unknown as ProfileAwareAgentSpawner, makeMockGrantService() as unknown as TaskGrantService)

      await expect(executor.executeTask({ ...EXEC_PARAMS, taskId: task.id })).rejects.toThrow()
      const updated = await taskService.get(task.id)
      expect(updated?.status).toBe('blocked')
      expect(updated?.agentSessionId).toBeNull()
    })

    it('throws TASK_NOT_FOUND for unknown taskId', async () => {
      const executor = new TaskAgentExecutor(taskService, makeMockSpawner() as unknown as ProfileAwareAgentSpawner, makeMockGrantService() as unknown as TaskGrantService)
      await expect(executor.executeTask({ ...EXEC_PARAMS, taskId: 'unknown-id' })).rejects.toThrow('TASK_NOT_FOUND')
    })

    it('throws TASK_PERMISSION_DENIED when user lacks execute perm', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const noPermGrantService = makeMockGrantService(null)
      const executor = new TaskAgentExecutor(taskService, makeMockSpawner() as unknown as ProfileAwareAgentSpawner, noPermGrantService as unknown as TaskGrantService)
      await expect(executor.executeTask({ ...EXEC_PARAMS, taskId: task.id })).rejects.toThrow('TASK_PERMISSION_DENIED')
    })

    it('does not call dispatchToOrchestration for a leaf task', async () => {
      const task = await taskService.create({
        title: 'Leaf', type: 'task', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const bridge = makeMockOrchestrationBridge()
      const executor = new TaskAgentExecutor(
        taskService, makeMockSpawner() as unknown as ProfileAwareAgentSpawner, makeMockGrantService() as unknown as TaskGrantService, bridge as unknown as TaskOrchestrationBridge
      )
      await executor.executeTask({ ...EXEC_PARAMS, taskId: task.id })
      expect(bridge.dispatch).not.toHaveBeenCalled()
    })
  })

  // ── (b) complex path — subtasks or dependency edges → orchestration ─────────
  describe('executeTask — complex path (has subtasks or dependency edges)', () => {
    it('routes a task with a child subtask to dispatchToOrchestration, and never spawns a single agent', async () => {
      const parent = await taskService.create({
        title: 'Epic', type: 'epic', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team',
      })
      await taskService.create({
        title: 'Subtask', type: 'subtask', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team', parentId: parent.id,
      })

      const spawner = makeMockSpawner()
      const bridge = makeMockOrchestrationBridge()
      const executor = new TaskAgentExecutor(
        taskService, spawner as unknown as ProfileAwareAgentSpawner, makeMockGrantService() as unknown as TaskGrantService, bridge as unknown as TaskOrchestrationBridge
      )

      await executor.executeTask({ ...EXEC_PARAMS, taskId: parent.id })

      expect(bridge.dispatch).toHaveBeenCalledWith(parent.id, { worktree: EXEC_PARAMS.worktreePath })
      expect(spawner.spawn).not.toHaveBeenCalled()
    })

    it('routes a task with a dependency edge to dispatchToOrchestration', async () => {
      const taskA = await taskService.create({
        title: 'A', type: 'task', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const taskB = await taskService.create({
        title: 'B', type: 'task', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team',
      })
      await taskService.addEdge(taskA.id, taskB.id, 'depends_on')

      const bridge = makeMockOrchestrationBridge()
      const executor = new TaskAgentExecutor(
        taskService, makeMockSpawner() as unknown as ProfileAwareAgentSpawner, makeMockGrantService() as unknown as TaskGrantService, bridge as unknown as TaskOrchestrationBridge
      )

      await executor.executeTask({ ...EXEC_PARAMS, taskId: taskA.id })
      expect(bridge.dispatch).toHaveBeenCalledWith(taskA.id, { worktree: EXEC_PARAMS.worktreePath })
    })

    it('throws TASK_ORCHESTRATION_UNAVAILABLE when no TaskOrchestrationBridge is wired', async () => {
      const parent = await taskService.create({
        title: 'Epic', type: 'epic', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team',
      })
      await taskService.create({
        title: 'Subtask', type: 'subtask', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team', parentId: parent.id,
      })

      // No 4th constructor arg — matches server-bootstrap.ts until Wave 3 wires it.
      const executor = new TaskAgentExecutor(taskService, makeMockSpawner() as unknown as ProfileAwareAgentSpawner, makeMockGrantService() as unknown as TaskGrantService)
      await expect(executor.executeTask({ ...EXEC_PARAMS, taskId: parent.id }))
        .rejects.toThrow('TASK_ORCHESTRATION_UNAVAILABLE')
    })
  })

  // ── buildPrompt ───────────────────────────────────────────────────────────────
  describe('buildPrompt', () => {
    it('includes task title in prompt', async () => {
      const task = await taskService.create({
        title: 'Implement Login Flow', type: 'task', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const executor = new TaskAgentExecutor(taskService, makeMockSpawner() as unknown as ProfileAwareAgentSpawner, makeMockGrantService() as unknown as TaskGrantService)
      const prompt = executor.buildPrompt(task)
      expect(prompt).toContain('Implement Login Flow')
    })

    it('uses promptTemplate when set', async () => {
      const task = await taskService.create({
        title: 'Templated Task', type: 'task', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team',
        promptTemplate: 'Execute: ${task.title} now',
      })
      const executor = new TaskAgentExecutor(taskService, makeMockSpawner() as unknown as ProfileAwareAgentSpawner, makeMockGrantService() as unknown as TaskGrantService)
      const prompt = executor.buildPrompt(task)
      expect(prompt).toBe('Execute: Templated Task now')
    })
  })

  // ── executeTask tracing (TASK-BE-018.5) ──────────────────────────────────────
  describe('executeTask tracing', () => {
    it('step order for simple path stays permission-check → agent-spawn (no extra step)', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      const executor = new TaskAgentExecutor(taskService, makeMockSpawner() as unknown as ProfileAwareAgentSpawner, makeMockGrantService() as unknown as TaskGrantService)

      const { events, stop } = captureTraceEvents()
      await executor.executeTask({ ...EXEC_PARAMS, taskId: task.id })
      stop()

      const spanEvents = events.filter((e) => e.flow === 'taskGraph:execute' && e.level === 'step')
      expect(spanEvents.map((e) => e.label)).toEqual(['permission-check', 'agent-spawn'])
    })

    it('complex path records an orchestration-dispatch step and span.ok(mode: orchestration)', async () => {
      const parent = await taskService.create({
        title: 'Epic', type: 'epic', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team',
      })
      await taskService.create({
        title: 'Subtask', type: 'subtask', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team', parentId: parent.id,
      })
      const bridge = makeMockOrchestrationBridge()
      const executor = new TaskAgentExecutor(
        taskService, makeMockSpawner() as unknown as ProfileAwareAgentSpawner, makeMockGrantService() as unknown as TaskGrantService, bridge as unknown as TaskOrchestrationBridge
      )

      const { events, stop } = captureTraceEvents()
      await executor.executeTask({ ...EXEC_PARAMS, taskId: parent.id })
      stop()

      const spanEvents = events.filter((e) => e.flow === 'taskGraph:execute')
      expect(spanEvents.some((e) => e.level === 'step' && e.label === 'orchestration-dispatch')).toBe(true)
      const okEvent = spanEvents.find((e) => e.level === 'ok')
      expect(okEvent?.fields.mode).toBe('orchestration')
    })
  })
})
