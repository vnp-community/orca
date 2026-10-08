/**
 * quality-block-error-message.ts — FE-CV-TASK-087-16
 *
 * Plain-language error line of a lens block. Errors render inline with a retry action, never as
 * a toast; raw backend messages are not shown.
 *
 * @module components/review-map/quality/quality-block-error-message
 */

import { qv } from './quality-visualization-copy'

export function describeQualityBlockError(error: { kind: string } | null): string {
  if (error?.kind === 'timeout') {
    return qv('error.timeout')
  }
  if (error?.kind === 'too-large') {
    return qv('error.tooLarge')
  }
  return qv('error.generic')
}
