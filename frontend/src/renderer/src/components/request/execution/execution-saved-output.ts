import type { ExecutionResult } from '../../../../../shared/request-artifact-types'
import type { ExecutionResultStatus } from '../../../hooks/useExecutionResult'

/** True when the Result tab has something to read: a structured record, or a non-empty saved output. */
export function hasSavedExecutionOutput(
  status: ExecutionResultStatus,
  result: ExecutionResult | null
): boolean {
  if (status !== 'ready' || !result) {
    return false
  }
  return result.source === 'last_execution_output' ? Boolean(result.stdoutTail?.trim()) : true
}
