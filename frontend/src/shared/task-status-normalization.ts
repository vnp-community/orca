/**
 * Task Status Normalisation — CR-REQ-018-06
 *
 * The backend has always only had 6 live status values:
 *   open, blocked, in_progress, review, done, cancelled
 *
 * 'backlog' was a frontend-only value never persisted by the task-service.
 * 'todo' exists in the frontend type but is also not sent by the current
 * backend; kept for legacy compat but normalised to 'open'.
 *
 * Any task fetched from the wire with status 'backlog' or any unknown string
 * is normalised to 'open' so it appears in the Board's Open column instead
 * of disappearing entirely.
 *
 * @module shared/task-status-normalization
 */

import type { OrcaTask, TaskStatus } from './task-types'

const VALID_STATUSES = new Set<TaskStatus>([
  'open',
  'todo',
  'in_progress',
  'review',
  'done',
  'blocked',
  'cancelled'
])

/**
 * Normalise a raw wire status value to a valid TaskStatus.
 * 'backlog' and any other unrecognised string → 'open'.
 */
export function normalizeTaskStatus(raw: unknown): TaskStatus {
  if (typeof raw === 'string' && VALID_STATUSES.has(raw as TaskStatus)) {
    return raw as TaskStatus
  }
  // 'backlog' is the primary legacy value we need to coerce; anything else
  // unknown also falls to 'open' so tasks are never silently lost.
  return 'open'
}

/**
 * Return a copy of `task` with its status field normalised.
 * Non-mutating — the original object is not modified.
 */
export function normalizeTask(task: OrcaTask): OrcaTask {
  const normalised = normalizeTaskStatus(task.status)
  if (normalised === task.status) return task
  return { ...task, status: normalised }
}
