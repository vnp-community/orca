/**
 * Error mapping of "revise the plan by prompt" (CR-REQ-045): every failure becomes a kind the panel can act on plus a sentence
 * for the reviewer. Codes are matched on the backend code; request-errors.ts has no entry for most of them.
 *
 * @module components/request/plan/revision/plan-revision-errors
 */

import { translate } from '@/i18n/i18n'
import type { RequestRpcError } from '../../../../../../shared/request-errors'
import { requestErrorMessage, requestErrorMessageForError } from '../../request-error-message'
import { backendErrorCode } from '../../solution/solution-error-classification'
import { FEEDBACK_MAX_LENGTH } from '../../solution/solution-view-model'

export type PlanRevisionErrorKind =
  | 'empty'
  | 'tooLong'
  | 'forbidden'
  | 'wrongState'
  | 'noConnection'
  | 'notApplicable'
  | 'noChange'
  | 'baseRefGone'
  | 'rateLimited'
  | 'network'
  | 'other'

export type PlanRevisionError = { kind: PlanRevisionErrorKind; message: string }

type MessageOptions = {
  /** The server saves the revised plan, so approvers may ask; otherwise only the reporter and admins can. */
  serverCommit: boolean
}

export function planRevisionMessage(
  kind: PlanRevisionErrorKind,
  { serverCommit }: MessageOptions = { serverCommit: true }
): string | null {
  switch (kind) {
    case 'empty':
      return translate(
        'auto.components.request.plan.revision.errorEmpty',
        'Describe what should change before you send it.'
      )
    case 'tooLong':
      return translate(
        'auto.components.request.plan.revision.errorTooLong',
        'Keep the request to {{max}} characters or fewer.',
        { max: FEEDBACK_MAX_LENGTH }
      )
    case 'forbidden':
      return serverCommit
        ? translate(
            'auto.components.request.plan.revision.errorForbidden',
            'Only the approvers of this plan can request changes. Your text is kept.'
          )
        : translate(
            'auto.components.request.plan.revision.errorForbiddenReporter',
            'Only the person who reported this request or a project admin can request changes to the plan. Your text is kept.'
          )
    case 'wrongState':
      return translate(
        'auto.components.request.plan.revision.errorWrongState',
        'Someone else is already revising this plan or has decided. It was refreshed; your text is kept.'
      )
    case 'noConnection':
      return translate(
        'auto.components.request.plan.revision.errorNoConnection',
        'No dev server is connected, so the plan cannot be revised right now. The current plan and its approval are unchanged and your text is kept.'
      )
    case 'notApplicable':
      return translate(
        'auto.components.request.plan.revision.errorNotApplicable',
        'This kind of plan cannot be revised with a request.'
      )
    case 'noChange':
      return translate(
        'auto.components.request.plan.revision.errorNoChange',
        'The agent did not change any document. Say more precisely what to change.'
      )
    case 'baseRefGone':
      return translate(
        'auto.components.request.plan.revision.errorBaseRefGone',
        'The branch this request starts from no longer exists, so the plan cannot be revised. Your text is kept.'
      )
    case 'rateLimited':
    case 'network':
    case 'other':
      return null
  }
}

export function classifyPlanRevisionKind(error: RequestRpcError): PlanRevisionErrorKind {
  const code = backendErrorCode(error)
  if (code === 'REQUEST_PLAN_FEEDBACK_TOO_LONG') {
    return 'tooLong'
  }
  if (
    code === 'REQUEST_STATE_STALE' ||
    code === 'REQUEST_PLAN_REVISION_IN_PROGRESS' ||
    error.kind === 'conflict'
  ) {
    return 'wrongState'
  }
  if (code === 'REQUEST_PLAN_AI_UNAVAILABLE' || error.kind === 'no_dev_server') {
    return 'noConnection'
  }
  if (code === 'REQUEST_PLAN_REVISION_NOT_APPLICABLE') {
    return 'notApplicable'
  }
  if (code === 'REQUEST_PLAN_REVISION_NO_CHANGE') {
    return 'noChange'
  }
  if (code === 'REQUEST_BASE_REF_NOT_FOUND') {
    return 'baseRefGone'
  }
  if (code === 'REQUEST_PLAN_FORBIDDEN' || error.kind === 'forbidden') {
    return 'forbidden'
  }
  if (error.kind === 'rate_limited') {
    return 'rateLimited'
  }
  return error.kind === 'network' ? 'network' : 'other'
}

export function classifyPlanRevisionError(
  error: RequestRpcError,
  options: MessageOptions = { serverCommit: true }
): PlanRevisionError {
  const kind = classifyPlanRevisionKind(error)
  const fixed = planRevisionMessage(kind, options)
  if (fixed !== null) {
    return { kind, message: fixed }
  }
  return {
    kind,
    message:
      kind === 'network' ? requestErrorMessage('network') : requestErrorMessageForError(error)
  }
}
