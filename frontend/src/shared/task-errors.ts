/**
 * Task channel error helpers (TASK-FE-E2E-007).
 *
 * task-service answers an unknown id with TASK_NOT_FOUND; depending on the layer the text
 * arrives as `TASK_NOT_FOUND: ...`, a bare `NOT_FOUND` or a gRPC `code = NotFound`.
 *
 * @module shared/task-errors
 */

// Word boundaries keep TASK_NOT_FOUND_BY_NUMBER and PROJECT_NOT_FOUND from matching.
const NOT_FOUND_RE = /\bTASK_NOT_FOUND\b|\bNOT_FOUND\b|code = NotFound/

/** True when a task call failed only because the task no longer exists. */
export function isTaskNotFoundError(err: unknown): boolean {
  if (err === null || typeof err !== 'object') {
    return false
  }
  const { code, message } = err as { code?: unknown; message?: unknown }
  if (typeof code === 'string' && /^(task_not_found|not_found)$/i.test(code)) {
    return true
  }
  return typeof message === 'string' && NOT_FOUND_RE.test(message)
}
