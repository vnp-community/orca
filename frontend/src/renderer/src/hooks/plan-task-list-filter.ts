/**
 * plan-task-list-filter — the one place that knows how the Plan tab asks `task.list` for a
 * Request's Plan/Phase/Task nodes (field names follow the gateway's optional task.list filters).
 *
 * @module hooks/plan-task-list-filter
 */

import type { OrcaTask } from '../../../shared/task-types'

// Why: task-service hides `plan`/`phase` from task.list unless the caller names the types, and a Plan's leaf
// tasks may be any type the plan validator allows (task, bug, feature, epic). `feature` is not in the
// shared TaskType union yet, so the wire list is plain strings.
export const PLAN_TREE_TASK_TYPES: readonly string[] = [
  'plan',
  'phase',
  'task',
  'bug',
  'feature',
  'epic'
]

export type PlanTaskListParams = {
  projectId: string
  requestId: string
  taskTypes: string[]
  pageSize: number
  pageToken?: string
}

export function planTaskListParams(args: {
  projectId: string
  requestId: string
  pageSize: number
  pageToken?: string
}): PlanTaskListParams {
  return {
    projectId: args.projectId,
    requestId: args.requestId,
    taskTypes: [...PLAN_TREE_TASK_TYPES],
    pageSize: args.pageSize,
    ...(args.pageToken ? { pageToken: args.pageToken } : {})
  }
}

/**
 * Fallback for a server that ignores the filters and returns the whole project: drop tasks that
 * belong to another Request. Tasks without a requestId stay; the tree is still built from planTaskId.
 */
export function keepTasksOfRequest(tasks: readonly OrcaTask[], requestId: string): OrcaTask[] {
  return tasks.filter((t) => !t.requestId || t.requestId === requestId)
}
