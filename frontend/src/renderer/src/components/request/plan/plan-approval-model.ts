/**
 * plan-approval-model.ts — CR-REQ-021-02
 *
 * Pure functions for attaching approvals to a plan subtree
 * and computing phase stats.
 *
 * @module components/request/plan/plan-approval-model
 */

import { TASK_STATUS_PROGRESS, type OrcaTask } from '../../../../../shared/task-types'
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
  if (matches.length === 0) {
    return null
  }
  return matches.reduce((best, curr) => (curr.updatedAt > best.updatedAt ? curr : best))
}

/**
 * Attach the most-recent approval to each plan/taskList/phase.
 * pre_deploy approvals are returned as a list (can be multiple).
 */
export function attachApprovals(tree: PlanSubtree, approvals: Approval[]): ApprovalMap {
  const relevant = approvals.filter((a) => PLAN_SUBJECTS.has(a.subjectType))

  const planApproval = tree.plan ? latestForSubject(relevant, 'plan', tree.plan.id) : null

  const taskListApproval = tree.plan ? latestForSubject(relevant, 'task_list', tree.plan.id) : null

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
    if (t.status === 'done' || t.status === 'cancelled') {
      done++
    } else if (t.status === 'blocked') {
      blocked++
    } else if (t.status === 'in_progress') {
      running++
    }
  }

  return { done, total: tasks.length, blocked, running }
}

// ---------------------------------------------------------------------------
// Progress
// ---------------------------------------------------------------------------

export type ResolvedProgress = { value: number; estimated: boolean }

/**
 * Backend cascades progressPercent (CR-REQ-011, not shipped yet): trust it when
 * positive, otherwise estimate from child statuses and flag it as an estimate.
 */
export function resolveProgress(node: OrcaTask, children: OrcaTask[]): ResolvedProgress {
  if (node.progressPercent > 0) {
    return { value: Math.round(node.progressPercent), estimated: false }
  }
  if (children.length === 0) {
    return { value: 0, estimated: false }
  }
  const sum = children.reduce((acc, c) => acc + (TASK_STATUS_PROGRESS[c.status] ?? 0), 0)
  const value = Math.round(sum / children.length)
  return { value, estimated: value > 0 }
}

/** All work tasks of the plan, phase tasks first-class, in display order. */
export function listPlanWorkTasks(tree: PlanSubtree): OrcaTask[] {
  return [...tree.phases.flatMap((p) => tree.tasksByPhase[p.id] ?? []), ...tree.flatTasks]
}

// ---------------------------------------------------------------------------
// Execution gates
// ---------------------------------------------------------------------------

export type ExecutionGateMap = Record<string, 'phase_not_approved' | 'plan_not_approved'>

/**
 * Tasks the UI should not offer "Run" for yet. Backend (CR-REQ-013) stays the source
 * of truth; this only hides the button early.
 */
export function computeExecutionGates(
  tree: PlanSubtree,
  approvals: ApprovalMap,
  planApprovalRequired: boolean
): ExecutionGateMap {
  const gates: ExecutionGateMap = {}
  const planApproval = approvals.plan ?? approvals.taskList
  const planBlocked = planApproval ? planApproval.status !== 'approved' : planApprovalRequired
  for (const phase of tree.phases) {
    const approval = approvals.byPhaseId[phase.id]
    const phaseBlocked = !!approval && approval.status !== 'approved'
    for (const t of tree.tasksByPhase[phase.id] ?? []) {
      if (planBlocked) {
        gates[t.id] = 'plan_not_approved'
      } else if (phaseBlocked) {
        gates[t.id] = 'phase_not_approved'
      }
    }
  }
  if (planBlocked) {
    for (const t of tree.flatTasks) {
      gates[t.id] = 'plan_not_approved'
    }
  }
  return gates
}
