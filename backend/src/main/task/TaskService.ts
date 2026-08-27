/**
 * TaskService — CRUD + Tree operations for the task graph (TDD-18)
 *
 * Implements 15 methods:
 * - CRUD: create, get, update, delete
 * - Tree: getChildren, getAncestors, getSubtree
 * - DAG edges: addEdge, removeEdge, getDependencies, getDependents
 * - Progress: recalculateProgress
 * - Query: list, findByRef
 * - Comments: addComment
 *
 * Column mapping (snake_case DB → camelCase):
 *   project_id → projectId, parent_id → parentId
 *   reporter_id → reporterId, assignee_id → assigneeId
 *   estimated_hours → estimatedHours, progress_percent → progressPercent
 *   ai_context → aiContext, prompt_template → promptTemplate
 *   labels: JSON parse/stringify
 *   created_at / updated_at: new Date(timestamp)
 *
 * @module main/task/TaskService
 */

import { randomUUID } from 'node:crypto'
import type { IConnectionPool } from '../db/pool'
import type { BindValue } from '../db/types'
import type { TaskDAGValidator } from './TaskDAGValidator'
import type {
  OrcaTask,
  CreateTaskParams,
  TaskStatus,
  TaskEdgeType,
} from '../../shared/task-types'
import { TASK_STATUS_PROGRESS } from '../../shared/task-types'
import { Tracers } from '../../shared/trace/tracers'
import type { TaskRow, EdgeRow } from './task-row-mapping'
import { rowToTask, TASK_SELECT } from './task-row-mapping'

export type ListTasksFilter = {
  projectId?: string
  parentId?: string
  assigneeId?: string
  status?: TaskStatus
  type?: OrcaTask['type']
  limit?: number
}

// ── TaskService ────────────────────────────────────────────────────────────────

export class TaskService {
  constructor(
    private readonly pool: IConnectionPool,
    private readonly dagValidator: TaskDAGValidator
  ) {}

  // ── CRUD ───────────────────────────────────────────────────────────────────

  async create(params: CreateTaskParams): Promise<OrcaTask> {
    const id = randomUUID()
    const now = Date.now()
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT INTO orca_tasks
           (id, project_id, parent_id, title, description, type, status, priority,
            labels, visibility, reporter_id, assignee_id, estimated_hours,
            progress_percent, ai_context, prompt_template, due_date, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, 'backlog', ?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?, ?)`,
        [
          id,
          params.projectId ?? null,
          params.parentId ?? null,
          params.title,
          params.description ?? null,
          params.type ?? 'task',
          params.priority ?? 'medium',
          JSON.stringify(params.labels ?? []),
          params.visibility ?? 'team',
          params.reporterId ?? null,
          params.assigneeId ?? null,
          params.estimatedHours ?? null,
          params.aiContext ?? null,
          params.promptTemplate ?? null,
          params.dueDate ? params.dueDate.getTime() : null,
          now,
          now,
        ]
      )
    )
    return (await this.get(id))!
  }

  async get(taskId: string): Promise<OrcaTask | null> {
    const rows = await this.pool.withConnection((db) =>
      db.query<TaskRow>(`${TASK_SELECT} WHERE id = ?`, [taskId])
    )
    return rows[0] ? rowToTask(rows[0]) : null
  }

  async update(taskId: string, patch: Partial<Omit<OrcaTask, 'id' | 'createdAt'>>): Promise<void> {
    const sets: string[] = ['updated_at = ?']
    const values: BindValue[] = [Date.now()]

    if (patch.title !== undefined) { sets.push('title = ?'); values.push(patch.title) }
    if (patch.description !== undefined) { sets.push('description = ?'); values.push(patch.description) }
    if (patch.status !== undefined) { sets.push('status = ?'); values.push(patch.status) }
    if (patch.priority !== undefined) { sets.push('priority = ?'); values.push(patch.priority) }
    if (patch.labels !== undefined) { sets.push('labels = ?'); values.push(JSON.stringify(patch.labels)) }
    if (patch.assigneeId !== undefined) { sets.push('assignee_id = ?'); values.push(patch.assigneeId) }
    if (patch.estimatedHours !== undefined) { sets.push('estimated_hours = ?'); values.push(patch.estimatedHours) }
    if (patch.progressPercent !== undefined) { sets.push('progress_percent = ?'); values.push(patch.progressPercent) }
    if (patch.aiContext !== undefined) { sets.push('ai_context = ?'); values.push(patch.aiContext) }
    if (patch.visibility !== undefined) { sets.push('visibility = ?'); values.push(patch.visibility) }
    if (patch.dueDate !== undefined) { sets.push('due_date = ?'); values.push(patch.dueDate?.getTime() ?? null) }
    if (patch.activeExecutionTaskId !== undefined) { sets.push('active_execution_task_id = ?'); values.push(patch.activeExecutionTaskId) }
    if (patch.agentSessionId !== undefined) { sets.push('agent_session_id = ?'); values.push(patch.agentSessionId) }

    values.push(taskId)
    await this.pool.withConnection((db) =>
      db.query(`UPDATE orca_tasks SET ${sets.join(', ')} WHERE id = ?`, values)
    )
  }

  async delete(taskId: string): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query(`DELETE FROM orca_tasks WHERE id = ?`, [taskId])
    )
  }

  // ── Tree ops ──────────────────────────────────────────────────────────────

  async getChildren(taskId: string): Promise<OrcaTask[]> {
    const rows = await this.pool.withConnection((db) =>
      db.query<TaskRow>(`${TASK_SELECT} WHERE parent_id = ? ORDER BY created_at ASC`, [taskId])
    )
    return rows.map(rowToTask)
  }

  async getAncestors(taskId: string): Promise<OrcaTask[]> {
    const ancestors: OrcaTask[] = []
    let current = await this.get(taskId)
    while (current?.parentId) {
      const parent = await this.get(current.parentId)
      if (!parent) {break}
      ancestors.unshift(parent) // prepend so root is first
      current = parent
    }
    return ancestors
  }

  async getSubtree(taskId: string): Promise<OrcaTask[]> {
    const result: OrcaTask[] = []
    const queue = [taskId]
    while (queue.length > 0) {
      const id = queue.shift()!
      const children = await this.getChildren(id)
      result.push(...children)
      queue.push(...children.map(c => c.id))
    }
    return result
  }

  // ── DAG edges ─────────────────────────────────────────────────────────────

  async addEdge(fromTaskId: string, toTaskId: string, edgeType: TaskEdgeType): Promise<void> {
    const span = Tracers.taskGraphEdgeFlow.start({ fromTaskId, toTaskId, edgeType })

    // wouldCreateCycle() is DFS (stack-based, see TaskDAGValidator docstring) — NOT BFS as
    // some earlier flow docs described. Still worth a dedicated step() per CR-TRACE-000 §5
    // rule 1+3: can be slow on large graphs (N sequential SELECTs along DFS depth) and is
    // the key branch point for troubleshooting "why was this edge rejected".
    const wouldCycle = await this.dagValidator.wouldCreateCycle(fromTaskId, toTaskId, edgeType)
    span.step('cycle-check', { wouldCycle })

    if (wouldCycle) {
      span.fail('TASK_DEPENDENCY_CYCLE', { fromTaskId, toTaskId, edgeType })
      throw new Error(`TASK_DAG_CYCLE: adding edge ${fromTaskId} → ${toTaskId} (${edgeType}) would create a cycle`)
    }

    await this.pool.withConnection((db) =>
      db.query(
        `INSERT OR IGNORE INTO orca_task_edges (from_task_id, to_task_id, edge_type, created_at)
         VALUES (?, ?, ?, ?)`,
        [fromTaskId, toTaskId, edgeType, Date.now()]
      )
    )
    span.ok({ fromTaskId, toTaskId, edgeType })
  }

  async removeEdge(fromTaskId: string, toTaskId: string, edgeType: TaskEdgeType): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query(
        `DELETE FROM orca_task_edges WHERE from_task_id = ? AND to_task_id = ? AND edge_type = ?`,
        [fromTaskId, toTaskId, edgeType]
      )
    )
  }

  async getDependencies(taskId: string): Promise<{ task: OrcaTask; edgeType: TaskEdgeType }[]> {
    const rows = await this.pool.withConnection((db) =>
      db.query<EdgeRow>(
        `SELECT from_task_id as "fromTaskId", to_task_id as "toTaskId", edge_type as "edgeType", created_at as "createdAt"
         FROM orca_task_edges WHERE from_task_id = ?`,
        [taskId]
      )
    )
    const result: { task: OrcaTask; edgeType: TaskEdgeType }[] = []
    for (const row of rows) {
      const task = await this.get(row.toTaskId)
      if (task) {result.push({ task, edgeType: row.edgeType as TaskEdgeType })}
    }
    return result
  }

  async getDependents(taskId: string): Promise<{ task: OrcaTask; edgeType: TaskEdgeType }[]> {
    const rows = await this.pool.withConnection((db) =>
      db.query<EdgeRow>(
        `SELECT from_task_id as "fromTaskId", to_task_id as "toTaskId", edge_type as "edgeType", created_at as "createdAt"
         FROM orca_task_edges WHERE to_task_id = ?`,
        [taskId]
      )
    )
    const result: { task: OrcaTask; edgeType: TaskEdgeType }[] = []
    for (const row of rows) {
      const task = await this.get(row.fromTaskId)
      if (task) {result.push({ task, edgeType: row.edgeType as TaskEdgeType })}
    }
    return result
  }

  // ── Progress ──────────────────────────────────────────────────────────────

  /**
   * Recalculate progress percent for a task.
   * - Leaf node: mapped from status via TASK_STATUS_PROGRESS
   * - Parent node: average of all direct children's recalculated progress (recursive)
   */
  async recalculateProgress(taskId: string): Promise<number> {
    const children = await this.getChildren(taskId)
    if (children.length === 0) {
      // Leaf node
      const task = await this.get(taskId)
      const progress = task ? (TASK_STATUS_PROGRESS[task.status] ?? 0) : 0
      await this.update(taskId, { progressPercent: progress })
      return progress
    }

    // Parent: recursive average of children's progress
    const childProgresses = await Promise.all(
      children.map(c => this.recalculateProgress(c.id))
    )
    const avg = Math.round(childProgresses.reduce((s, p) => s + p, 0) / childProgresses.length)
    await this.update(taskId, { progressPercent: avg })
    return avg
  }

  // ── Query ─────────────────────────────────────────────────────────────────

  async list(filters: ListTasksFilter = {}): Promise<OrcaTask[]> {
    const clauses: string[] = []
    const params: BindValue[] = []

    if (filters.projectId) { clauses.push('project_id = ?'); params.push(filters.projectId) }
    if (filters.parentId !== undefined) { clauses.push('parent_id = ?'); params.push(filters.parentId) }
    if (filters.assigneeId) { clauses.push('assignee_id = ?'); params.push(filters.assigneeId) }
    if (filters.status) { clauses.push('status = ?'); params.push(filters.status) }
    if (filters.type) { clauses.push('type = ?'); params.push(filters.type) }

    const where = clauses.length > 0 ? `WHERE ${clauses.join(' AND ')}` : ''
    const limit = filters.limit ?? 200
    params.push(limit)

    const rows = await this.pool.withConnection((db) =>
      db.query<TaskRow>(
        `${TASK_SELECT} ${where} ORDER BY created_at DESC LIMIT ?`,
        params
      )
    )
    return rows.map(rowToTask)
  }

  /**
   * Search by id prefix (short ref) or exact title match.
   */
  async findByRef(ref: string): Promise<OrcaTask | null> {
    const rows = await this.pool.withConnection((db) =>
      db.query<TaskRow>(
        `${TASK_SELECT} WHERE id LIKE ? OR title = ? LIMIT 1`,
        [`${ref}%`, ref]
      )
    )
    return rows[0] ? rowToTask(rows[0]) : null
  }

  // ── Comments ──────────────────────────────────────────────────────────────

  async addComment(
    taskId: string,
    userId: string,
    content: string,
    type: 'comment' | 'activity' = 'comment'
  ): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT INTO orca_task_comments (task_id, user_id, content, type, created_at)
         VALUES (?, ?, ?, ?, ?)`,
        [taskId, userId, content, type, Date.now()]
      )
    )
  }
}
