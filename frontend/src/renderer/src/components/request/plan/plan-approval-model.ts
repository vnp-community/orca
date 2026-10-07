/**
 * plan-approval-model.ts — CR-REQ-021-02
 *
 * Pure functions for attaching approvals to a plan subtree
 * and computing phase stats.
 *
 * @module components/request/plan/plan-approval-model
 */

import type { OrcaTask } from '../../../../../shared/task-types'
import type { Approval } from '../../../../../shared/request-types'
import type { PlanSubtree } from '../../../../../shared/task-hierarchy'

// ---------------------------------------------------------------------------
// attachApprovals
// ---------------------------------------------------------------------------

export type ApprovalMap = {
  plan: Approval | null
  taskList: Approval | null
  preDeployList: Approval[]
  byPhaseId: Record<string, Approval | null>
}

/** The approval subject kinds we care about */
const PLAN_SUBJECTS = new Set<string>(['plan', 'task_list', 'phase', 'pre_deploy'])

/**
 * Pick the latest approval for a given subjectType+subjectId combination.
 * "Latest" = largest updatedAt string (ISO8601 lexicographic comparison works).
 */
function latestForSubject(
  approvals: Approval[],
  subjectType: string,
  subjectId: string
): Approval | null {
  const matches = approvals.filter(
    (a) => a.subjectType === subjectType && a.subjectId === subjectId
  )
  if (matches.length === 0) return null
  return matches.reduce((best, curr) =>
    curr.updatedAt > best.updatedAt ? curr : best
  )
}

/**
 * Attach the most-recent approval to each plan/taskList/phase.
 * pre_deploy approvals are returned as a list (can be multiple).
 */
export function attachApprovals(tree: PlanSubtree, approvals: Approval[]): ApprovalMap {
  const relevant = approvals.filter((a) => PLAN_SUBJECTS.has(a.subjectType))

  const planApproval = tree.plan
    ? latestForSubject(relevant, 'plan', tree.plan.id)
    : null

  const taskListApproval = tree.plan
    ? latestForSubject(relevant, 'task_list', tree.plan.id)
    : null

  const byPhaseId: Record<string, Approval | null> = {}
  for (const phase of tree.phases) {
    byPhaseId[phase.id] = latestForSubject(relevant, 'phase', phase.id)
  }

  const preDeployList = relevant.filter((a) => a.subjectType === 'pre_deploy')

  return {
    plan: planApproval,
    taskList: taskListApproval,
    preDeployList,
    byPhaseId
  }
}

// ---------------------------------------------------------------------------
// computePhaseStats
// ---------------------------------------------------------------------------

export type PhaseStats = {
  done: number
  total: number
  blocked: number
  running: number
}

/**
 * Compute completion stats for a phase's task list.
 */
export function computePhaseStats(tasks: OrcaTask[]): PhaseStats {
  let done = 0
  let blocked = 0
  let running = 0

  for (const t of tasks) {
    if (t.status === 'done' || t.status === 'cancelled') done++
    else if (t.status === 'blocked') blocked++
    else if (t.status === 'in_progress') running++
  }

  return { done, total: tasks.length, blocked, running }
}
