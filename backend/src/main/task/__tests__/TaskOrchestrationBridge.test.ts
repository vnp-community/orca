/**
 * Tests for TaskOrchestrationBridge — the complex-task ((b)) path's seeding
 * of an OrchestrationDb TaskRow tree from an OrcaTask subtree, and its
 * write-back once the coordinator run converges.
 */

import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { SqliteSingleConnectionPool } from '../../db/sqlite/sqlite-pool'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations'
import { TaskService } from '../TaskService'
import { TaskDAGValidator } from '../TaskDAGValidator'
import { TaskOrchestrationBridge, type TaskOrchestrationRuntime } from '../TaskOrchestrationBridge'
import { OrchestrationDb } from '../../runtime/orchestration/db'
import type { CoordinatorRuntime } from '../../runtime/orchestration/coordinator'

function createMockRuntime(db: OrchestrationDb): TaskOrchestrationRuntime & {
  terminals: { handle: string; worktreeId: string; connected: boolean; writable: boolean }[]
} {
  const mock: CoordinatorRuntime & { terminals: { handle: string; worktreeId: string; connected: boolean; writable: boolean }[] } = {
    terminals: [{ handle: 'term_a', worktreeId: 'wt1', connected: true, writable: true }],
    async sendTerminalAgentPrompt(handle: string) {
      return { handle, accepted: true }
    },
    async listTerminals() {
      return { terminals: mock.terminals }
    },
    async createTerminal(_worktree, opts) {
      return { handle: 'term_new', worktreeId: 'wt1', title: opts?.title ?? '' } as { handle: string; worktreeId: string }
    },
    async waitForTerminal(handle: string) {
      return { handle, condition: 'exit' }
    },
    async probeWorktreeDrift() {
      return null
    },
  }
  return Object.assign(mock, { getOrchestrationDb: () => db })
}

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

async function waitUntil(predicate: () => Promise<boolean>, timeoutMs = 2000, stepMs = 20): Promise<void> {
  const start = Date.now()
  while (Date.now() - start < timeoutMs) {
    if (await predicate()) {return}
    await new Promise((r) => setTimeout(r, stepMs))
  }
  throw new Error('waitUntil: timed out')
}

describe('TaskOrchestrationBridge', () => {
  let taskService: TaskService
  let orchestrationDb: OrchestrationDb

  beforeEach(async () => {
    taskService = await makeTaskService()
    orchestrationDb = new OrchestrationDb(':memory:')
  })

  afterEach(() => {
    orchestrationDb.close()
  })

  it('throws TASK_NOT_FOUND for an unknown taskId', async () => {
    const runtime = createMockRuntime(orchestrationDb)
    const bridge = new TaskOrchestrationBridge(taskService, runtime)
    await expect(bridge.dispatch('unknown-id')).rejects.toThrow('TASK_NOT_FOUND')
  })

  it('seeds a TaskRow per subtree node with children as deps, and writes activeExecutionTaskId', async () => {
    const epic = await taskService.create({
      title: 'Epic', type: 'epic', priority: 'high',
      reporterId: 'reporter-001', visibility: 'team',
    })
    const sub1 = await taskService.create({
      title: 'Sub 1', type: 'subtask', priority: 'high',
      reporterId: 'reporter-001', visibility: 'team', parentId: epic.id,
    })
    const sub2 = await taskService.create({
      title: 'Sub 2', type: 'subtask', priority: 'high',
      reporterId: 'reporter-001', visibility: 'team', parentId: epic.id,
    })

    const runtime = createMockRuntime(orchestrationDb)
    // Why: keep the background coordinator loop from racing dispatch in this
    // synchronous-assertions test — a huge poll interval means the first
    // tick (which this test doesn't wait for) never fires before assertions run.
    const bridge = new TaskOrchestrationBridge(taskService, runtime)
    const { taskRowId } = await bridge.dispatch(epic.id, { pollIntervalMs: 60_000 })

    const rootRow = orchestrationDb.getTask(taskRowId)
    expect(rootRow).toBeDefined()
    expect(rootRow?.task_title).toBe('Epic')
    const rootDeps = JSON.parse(rootRow!.deps) as string[]
    expect(rootDeps).toHaveLength(2)
    // Root has children → starts 'pending' (waits for subtasks to complete first)
    expect(rootRow?.status).toBe('pending')

    const allRows = orchestrationDb.listTasks()
    expect(allRows).toHaveLength(3) // epic + 2 subtasks
    const childRows = allRows.filter((r) => r.id !== taskRowId)
    expect(childRows).toHaveLength(2)
    expect(childRows.map((r) => r.task_title).sort()).toEqual(['Sub 1', 'Sub 2'])
    // Leaf subtasks have no deps → immediately 'ready'
    for (const row of childRows) {
      expect(JSON.parse(row.deps)).toEqual([])
      expect(row.status).toBe('ready')
    }
    expect(rootDeps.sort()).toEqual(childRows.map((r) => r.id).sort())

    const updatedEpic = await taskService.get(epic.id)
    expect(updatedEpic?.activeExecutionTaskId).toBe(taskRowId)

    void sub1
    void sub2
  })

  it('throws TASK_ORCHESTRATION_BUSY when a coordinator run is already active', async () => {
    orchestrationDb.createCoordinatorRun({ spec: 'other run', coordinatorHandle: 'someone-else' })

    const epic = await taskService.create({
      title: 'Epic', type: 'epic', priority: 'high',
      reporterId: 'reporter-001', visibility: 'team',
    })
    await taskService.create({
      title: 'Sub', type: 'subtask', priority: 'high',
      reporterId: 'reporter-001', visibility: 'team', parentId: epic.id,
    })

    const runtime = createMockRuntime(orchestrationDb)
    const bridge = new TaskOrchestrationBridge(taskService, runtime)
    await expect(bridge.dispatch(epic.id)).rejects.toThrow('TASK_ORCHESTRATION_BUSY')
  })

  it('writes status=review and clears activeExecutionTaskId once the coordinator run converges', async () => {
    const epic = await taskService.create({
      title: 'Epic', type: 'epic', priority: 'high',
      reporterId: 'reporter-001', visibility: 'team',
    })
    const sub = await taskService.create({
      title: 'Sub', type: 'subtask', priority: 'high',
      reporterId: 'reporter-001', visibility: 'team', parentId: epic.id,
    })

    const runtime = createMockRuntime(orchestrationDb)
    const bridge = new TaskOrchestrationBridge(taskService, runtime)
    const { taskRowId: rootRowId } = await bridge.dispatch(epic.id, { pollIntervalMs: 30 })

    // Find the seeded subtask TaskRow and complete it via a worker_done message
    // — the same mechanism runtime/orchestration/coordinator.test.ts uses.
    const rows = orchestrationDb.listTasks()
    const subRow = rows.find((r) => r.id !== rootRowId)!
    expect(subRow.task_title).toBe(sub.title)

    await waitUntil(async () => orchestrationDb.getDispatchContext(subRow.id) !== undefined)
    const dispatch = orchestrationDb.getDispatchContext(subRow.id)!
    orchestrationDb.insertMessage({
      from: dispatch.assignee_handle ?? 'term_a',
      to: `task-executor:${epic.id}`,
      subject: 'Done',
      type: 'worker_done',
      payload: JSON.stringify({ taskId: subRow.id, dispatchId: dispatch.id }),
    })

    // Root TaskRow depends on subRow — once subRow completes, root becomes
    // ready, gets dispatched, and completes the same way.
    await waitUntil(async () => {
      const rootDispatch = orchestrationDb.getDispatchContext(rootRowId)
      return rootDispatch !== undefined
    })
    const rootDispatch = orchestrationDb.getDispatchContext(rootRowId)!
    orchestrationDb.insertMessage({
      from: rootDispatch.assignee_handle ?? 'term_a',
      to: `task-executor:${epic.id}`,
      subject: 'Done',
      type: 'worker_done',
      payload: JSON.stringify({ taskId: rootRowId, dispatchId: rootDispatch.id }),
    })

    await waitUntil(async () => {
      const updated = await taskService.get(epic.id)
      return updated?.status === 'review'
    })
    const finalEpic = await taskService.get(epic.id)
    expect(finalEpic?.status).toBe('review')
    expect(finalEpic?.activeExecutionTaskId).toBeNull()
  })
})
