/**
 * TaskOrchestrationBridge — seeds a multi-agent coordinator run from an
 * OrcaTask subtree and wires it back onto the task (Source→Plan→Execute
 * pipeline "complex" path — docs/guides/task-automation-orchestration-
 * integration.md §9.2/§9.4.2/§9.4.4).
 *
 * Split out of TaskAgentExecutor.ts (rather than inlined there) so that file
 * stays under the repo's 300-line .ts ceiling without an eslint max-lines
 * exception (AGENTS.md "Lint Rules: Do Not Disable Max Lines").
 *
 * @module main/task/TaskOrchestrationBridge
 */

import type { TaskService } from './TaskService'
import type { OrcaTask } from '../../shared/task-types'
import type { CoordinatorRuntime } from '../runtime/orchestration/coordinator'
import { Coordinator } from '../runtime/orchestration/coordinator'
import type { OrchestrationDb } from '../runtime/orchestration/db'
import { buildTaskAgentPrompt } from './task-agent-prompt'
import { recordOrchestrationRunCompletion } from './task-execution-result-listener'

/**
 * Narrow structural type for what dispatch() needs from the runtime.
 * OrcaRuntimeService (backend/src/main/runtime/orca-runtime.ts) already
 * implements every CoordinatorRuntime method directly plus getOrchestrationDb()
 * — it satisfies this type as-is. Declared narrowly here (instead of importing
 * the concrete class) to avoid pulling the whole runtime module into the task
 * domain for 7 methods' worth of surface.
 */
export type TaskOrchestrationRuntime = CoordinatorRuntime & {
  getOrchestrationDb(): OrchestrationDb
}

export type DispatchOptions = {
  /** Forwarded to Coordinator as the worktree selector for dispatched workers. */
  worktree?: string
  /** Forwarded to Coordinator — mirrors orchestration.run's same-named param. */
  pollIntervalMs?: number
  /** Forwarded to Coordinator — mirrors orchestration.run's same-named param. */
  maxConcurrent?: number
}

export class TaskOrchestrationBridge {
  constructor(
    private readonly taskService: TaskService,
    private readonly runtime: TaskOrchestrationRuntime
  ) {}

  /**
   * Seed an OrchestrationDb TaskRow tree from `taskId`'s subtree and start a
   * coordinator run over it. Returns the root TaskRow id (already persisted
   * onto OrcaTask.activeExecutionTaskId by the time this resolves).
   */
  async dispatch(taskId: string, options: DispatchOptions = {}): Promise<{ taskRowId: string }> {
    const root = await this.taskService.get(taskId)
    if (!root) {
      throw new Error(`TASK_NOT_FOUND: ${taskId}`)
    }

    const db = this.runtime.getOrchestrationDb()
    const existingRun = db.getActiveCoordinatorRun()
    if (existingRun) {
      throw new Error(`TASK_ORCHESTRATION_BUSY: coordinator already running (${existingRun.id})`)
    }

    const subtree = await this.taskService.getSubtree(taskId)
    const rootRowId = this.seedTaskRows(db, root, subtree)

    await this.taskService.update(taskId, { activeExecutionTaskId: rootRowId })

    const coordinatorHandle = `task-executor:${taskId}`
    const runSpec = buildTaskAgentPrompt(root)
    const coordinator = new Coordinator(db, this.runtime, {
      spec: runSpec,
      coordinatorHandle,
      worktree: options.worktree,
      pollIntervalMs: options.pollIntervalMs,
      maxConcurrent: options.maxConcurrent,
    })
    const run = db.createCoordinatorRun({ spec: runSpec, coordinatorHandle })

    // Why: fire-and-forget, matching orchestration.run's RPC handler
    // (orchestration-gates.ts) — the coordinator loop runs in the background;
    // write-back happens once its promise resolves. No discrete completion
    // event exists on Coordinator (see task-execution-result-listener.ts).
    coordinator
      .runFromExistingRun(run.id)
      .then((result) => recordOrchestrationRunCompletion(this.taskService, taskId, result))
      .catch(() => {
        // Best-effort write-back; matches TaskAgentExecutor's
        // .catch(() => {}) pattern on its own status-write-on-failure path.
        return this.taskService.update(taskId, { status: 'blocked', activeExecutionTaskId: null })
      })
      .catch(() => {})

    return { taskRowId: rootRowId }
  }

  /**
   * Create one TaskRow per node in `subtree` plus the root itself, deepest
   * descendants first, so a parent's TaskRow.deps can reference its
   * already-created children's TaskRow ids (§9.4.2 step 2 — "subTaskIds →
   * TaskRow.deps"). Reuses the existing parentId links from getSubtree()
   * rather than re-deriving parent/child relationships.
   *
   * TaskRow.parent_id is intentionally left unset: OrcaTask.id and TaskRow.id
   * are different id spaces in different SQLite databases (see audit §7b) —
   * only the dependency edge (deps) is part of the mapping this bridge owns.
   */
  private seedTaskRows(db: OrchestrationDb, root: OrcaTask, subtree: OrcaTask[]): string {
    const childrenByParent = new Map<string, OrcaTask[]>()
    for (const task of subtree) {
      const key = task.parentId ?? ''
      const list = childrenByParent.get(key) ?? []
      list.push(task)
      childrenByParent.set(key, list)
    }

    const rowIdByTaskId = new Map<string, string>()
    const createRow = (task: OrcaTask): string => {
      const childRowIds = (childrenByParent.get(task.id) ?? [])
        .map((child) => rowIdByTaskId.get(child.id))
        .filter((id): id is string => id !== undefined)
      const row = db.createTask({
        spec: buildTaskAgentPrompt(task),
        taskTitle: task.title,
        deps: childRowIds,
      })
      rowIdByTaskId.set(task.id, row.id)
      return row.id
    }
    const visit = (task: OrcaTask): void => {
      for (const child of childrenByParent.get(task.id) ?? []) {
        visit(child)
      }
      createRow(task)
    }

    for (const task of subtree) {
      if (!rowIdByTaskId.has(task.id)) {
        visit(task)
      }
    }
    return createRow(root)
  }
}
