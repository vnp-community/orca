/**
 * execution-task-state.ts — FE-REQ-TASK-037-05
 *
 * What one task of a run shows. "Failed" is not a TaskStatus: it is derived from the run summary (F4), so the row can say how
 * many attempts went and why they ended.
 *
 * @module components/request/execution/execution-task-state
 */

import type { ExecutionSummary, FailureCause } from '../../../../../shared/request-execution-types'
import type { OrcaTask } from '../../../../../shared/task-types'

export type TaskDisplayState = 'queued' | 'waiting' | 'running' | 'done' | 'failed' | 'blocked'

export type TaskDisplay = {
  state: TaskDisplayState
  attempt?: number
  maxAttempts?: number
  cause?: FailureCause
  lastError?: string
  lastAt?: string
  /** For `waiting`: the title of the earlier task this one comes after. */
  waitsOn?: string
}

export type TaskDisplayContext = {
  /** The task is in the step the run is at (or the plan has no steps). */
  inCurrentStep: boolean
  /** The unfinished task right before this one in the same step, if any. */
  previousOpen?: OrcaTask
}

const FINISHED = new Set(['done', 'cancelled'])

export function deriveTaskDisplayState(
  task: OrcaTask,
  summary: ExecutionSummary | null,
  context: TaskDisplayContext
): TaskDisplay {
  if (FINISHED.has(task.status)) {
    return { state: 'done' }
  }
  const failed = summary?.failed.find((f) => f.taskId === task.id)
  if (failed) {
    return {
      state: 'failed',
      attempt: failed.attempts,
      maxAttempts: failed.maxAttempts,
      cause: failed.lastCause,
      lastError: failed.lastError,
      lastAt: failed.lastAt
    }
  }
  const running = summary?.running.find((r) => r.taskId === task.id)
  if (running) {
    return { state: 'running', attempt: running.attempt, maxAttempts: running.maxAttempts }
  }
  if (task.status === 'in_progress' || task.status === 'review') {
    return { state: 'running', maxAttempts: summary?.maxAttempts }
  }
  if (task.status === 'blocked') {
    return { state: 'blocked' }
  }
  if (context.inCurrentStep && context.previousOpen) {
    return { state: 'waiting', waitsOn: context.previousOpen.title }
  }
  return { state: 'queued' }
}
