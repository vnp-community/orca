/**
 * Error mapping of "revise the analysis by prompt" (CR-REQ-041): every failure becomes a kind the panel can act on plus a
 * sentence for the reviewer. Codes are matched on the backend code because request-errors.ts has no entry for the
 * feedback-length one.
 *
 * @module components/request/solution/analysis-revision-errors
 */

import { translate } from '@/i18n/i18n'
import type { RequestRpcError } from '../../../../../shared/request-errors'
import { requestErrorMessage, requestErrorMessageForError } from '../request-error-message'
import { backendErrorCode } from './solution-error-classification'
import { FEEDBACK_MAX_LENGTH } from './solution-view-model'

export type AnalysisRevisionErrorKind =
  | 'empty'
  | 'tooLong'
  | 'forbidden'
  | 'wrongState'
  | 'noConnection'
  | 'rateLimited'
  | 'network'
  | 'other'

export type AnalysisRevisionError = { kind: AnalysisRevisionErrorKind; message: string }

export function analysisRevisionMessage(kind: AnalysisRevisionErrorKind): string | null {
  switch (kind) {
    case 'empty':
      return translate(
        'auto.components.request.AnalysisRevision.errorEmpty',
        'Describe what should change before you send it.'
      )
    case 'tooLong':
      return translate(
        'auto.components.request.AnalysisRevision.errorTooLong',
        'Keep the request to {{max}} characters or fewer.',
        {
          max: FEEDBACK_MAX_LENGTH
        }
      )
    case 'forbidden':
      return translate(
        'auto.components.request.AnalysisRevision.errorForbidden',
        'Only the approvers of this analysis can request changes. Your text is kept.'
      )
    case 'wrongState':
      return translate(
        'auto.components.request.AnalysisRevision.errorWrongState',
        'Someone else is already revising this analysis or has decided. It was refreshed; your text is kept.'
      )
    case 'noConnection':
      return translate(
        'auto.components.request.AnalysisRevision.errorNoConnection',
        'No dev server is connected, so the analysis cannot be revised right now. The current version is unchanged and your text is kept.'
      )
    default:
      return null
  }
}

export function classifyAnalysisRevisionError(error: RequestRpcError): AnalysisRevisionError {
  const code = backendErrorCode(error)
  const kind = ((): AnalysisRevisionErrorKind => {
    if (code === 'REQUEST_SOLUTION_FEEDBACK_TOO_LONG') {
      return 'tooLong'
    }
    if (code === 'REQUEST_SOLUTION_WRONG_STATE' || error.kind === 'conflict') {
      return 'wrongState'
    }
    if (code === 'REQUEST_ANALYSIS_NO_CONNECTION' || code === 'REQUEST_SOLUTION_NO_CONNECTION') {
      return 'noConnection'
    }
    if (error.kind === 'forbidden') {
      return 'forbidden'
    }
    if (error.kind === 'rate_limited') {
      return 'rateLimited'
    }
    return error.kind === 'network' ? 'network' : 'other'
  })()
  const fixed = analysisRevisionMessage(kind)
  if (fixed !== null) {
    return { kind, message: fixed }
  }
  return {
    kind,
    message:
      kind === 'network' ? requestErrorMessage('network') : requestErrorMessageForError(error)
  }
}
