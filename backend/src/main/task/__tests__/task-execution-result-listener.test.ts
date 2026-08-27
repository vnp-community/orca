/**
 * Tests for task-execution-result-listener — the two write-back functions
 * TaskAgentExecutor (path a) and TaskOrchestrationBridge (path b) call once
 * they observe their respective execution outcome.
 */

import { describe, it, expect, beforeEach } from 'vitest'
import { SqliteSingleConnectionPool } from '../../db/sqlite/sqlite-pool'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations'
import { TaskService } from '../TaskService'
import { TaskDAGValidator } from '../TaskDAGValidator'
import {
  recordAgentSessionCompletion,
  recordOrchestrationRunCompletion,
} from '../task-execution-result-listener'

async function makeTaskService() {
  const pool = new SqliteSingleConnectionPool(':memory:')
  await pool.withConnection(async (db) => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
  })
  const service = new TaskService(pool, new TaskDAGValidator(pool))
  await pool.withConnection((db) =>
    db.query(
      'INSERT INTO orca_users (id, email, name, role, provider, created_at) VALUES (?, ?, ?, ?, ?, ?)',
      ['reporter-001', 'reporter@test.com', 'Reporter', 'developer', 'none', Date.now()]
    )
  )
  return service
}

describe('task-execution-result-listener', () => {
  let taskService: TaskService

  beforeEach(async () => {
    taskService = await makeTaskService()
  })

  describe('recordAgentSessionCompletion', () => {
    it('writes status and agentSessionId', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      await recordAgentSessionCompletion(taskService, task.id, { status: 'review', agentSessionId: 'sess-1' })
      const updated = await taskService.get(task.id)
      expect(updated?.status).toBe('review')
      expect(updated?.agentSessionId).toBe('sess-1')
    })

    it('writes status only when agentSessionId is omitted (failure path)', async () => {
      const task = await taskService.create({
        title: 'T', type: 'task', priority: 'low',
        reporterId: 'reporter-001', visibility: 'team',
      })
      await recordAgentSessionCompletion(taskService, task.id, { status: 'blocked' })
      const updated = await taskService.get(task.id)
      expect(updated?.status).toBe('blocked')
      expect(updated?.agentSessionId).toBeNull()
    })
  })

  describe('recordOrchestrationRunCompletion', () => {
    it('maps a completed run to review and clears activeExecutionTaskId', async () => {
      const task = await taskService.create({
        title: 'Epic', type: 'epic', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team',
      })
      await taskService.update(task.id, { activeExecutionTaskId: 'row-root' })

      await recordOrchestrationRunCompletion(taskService, task.id, {
        runId: 'run-1', status: 'completed', completedTasks: ['row-root'], failedTasks: [], escalations: [],
      })

      const updated = await taskService.get(task.id)
      expect(updated?.status).toBe('review')
      expect(updated?.activeExecutionTaskId).toBeNull()
    })

    it('maps a failed run to blocked and clears activeExecutionTaskId', async () => {
      const task = await taskService.create({
        title: 'Epic', type: 'epic', priority: 'high',
        reporterId: 'reporter-001', visibility: 'team',
      })
      await taskService.update(task.id, { activeExecutionTaskId: 'row-root' })

      await recordOrchestrationRunCompletion(taskService, task.id, {
        runId: 'run-1', status: 'failed', completedTasks: [], failedTasks: ['row-child'], escalations: [],
      })

      const updated = await taskService.get(task.id)
      expect(updated?.status).toBe('blocked')
      expect(updated?.activeExecutionTaskId).toBeNull()
    })
  })
})
