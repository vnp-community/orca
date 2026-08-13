/**
 * task-row-mapping — orca_tasks SQL row shape and its mapping to OrcaTask.
 *
 * Split out of TaskService.ts (pure data-shape/mapping, no service logic) to
 * keep that file under the repo's 300-line .ts ceiling without an eslint
 * max-lines exception (AGENTS.md "Lint Rules: Do Not Disable Max Lines").
 * TaskService's public API is unchanged — only this private implementation
 * detail moved.
 *
 * Column mapping (snake_case DB → camelCase):
 *   project_id → projectId, parent_id → parentId
 *   reporter_id → reporterId, assignee_id → assigneeId
 *   estimated_hours → estimatedHours, progress_percent → progressPercent
 *   ai_context → aiContext, prompt_template → promptTemplate
 *   active_execution_task_id → activeExecutionTaskId, agent_session_id → agentSessionId
 *   labels: JSON parse/stringify
 *   created_at / updated_at: new Date(timestamp)
 *
 * @module main/task/task-row-mapping
 */

import type { OrcaTask, TaskStatus } from '../../shared/task-types'

export type TaskRow = {
  id: string
  projectId: string | null
  parentId: string | null
  title: string
  description: string | null
  type: string
  status: string
  priority: string
  labels: string // JSON
  visibility: string
  reporterId: string | null
  assigneeId: string | null
  estimatedHours: number | null
  progressPercent: number
  aiContext: string | null
  promptTemplate: string | null
  dueDate: number | null
  activeExecutionTaskId: string | null
  agentSessionId: string | null
  createdAt: number
  updatedAt: number
}

export type EdgeRow = {
  fromTaskId: string
  toTaskId: string
  edgeType: string
  createdAt: number | null
}

export function rowToTask(r: TaskRow): OrcaTask {
  return {
    id: r.id,
    projectId: r.projectId ?? undefined,
    parentId: r.parentId ?? undefined,
    title: r.title,
    description: r.description ?? undefined,
    type: r.type as OrcaTask['type'],
    status: r.status as TaskStatus,
    priority: r.priority as OrcaTask['priority'],
    labels: JSON.parse(r.labels) as string[],
    visibility: r.visibility as OrcaTask['visibility'],
    reporterId: r.reporterId ?? undefined,
    assigneeId: r.assigneeId ?? undefined,
    estimatedHours: r.estimatedHours ?? undefined,
    progressPercent: r.progressPercent,
    aiContext: r.aiContext ?? undefined,
    promptTemplate: r.promptTemplate ?? undefined,
    dueDate: r.dueDate ? new Date(r.dueDate) : undefined,
    // Why: not normalized to undefined (unlike the fields above) — callers need to
    // tell "cleared to null" apart from "never set" (see TaskService.update()'s null guard).
    activeExecutionTaskId: r.activeExecutionTaskId,
    agentSessionId: r.agentSessionId,
    createdAt: new Date(r.createdAt),
    updatedAt: new Date(r.updatedAt),
  }
}

export const TASK_SELECT = `
  SELECT id,
         project_id       as projectId,
         parent_id        as parentId,
         title, description, type, status, priority, labels, visibility,
         reporter_id      as reporterId,
         assignee_id      as assigneeId,
         estimated_hours  as estimatedHours,
         progress_percent as progressPercent,
         ai_context       as aiContext,
         prompt_template  as promptTemplate,
         due_date         as dueDate,
         active_execution_task_id as activeExecutionTaskId,
         agent_session_id as agentSessionId,
         created_at       as createdAt,
         updated_at       as updatedAt
  FROM orca_tasks
`
