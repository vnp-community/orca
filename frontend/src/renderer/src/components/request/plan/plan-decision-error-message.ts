/**
 * plan-decision-error-message.ts — CR-REQ-021-04
 *
 * @module components/request/plan/plan-decision-error-message
 */

import { translate } from '@/i18n/i18n'
import { requestErrorMessage } from '../request-error-message'
import { splitErrorCode } from '../../../../../shared/request-errors'
import { backendErrorCode } from '../solution/solution-error-classification'
import type { PlanDecisionFailure } from '../../../hooks/usePlanDecision'

const P = 'auto.components.request.plan.'

/** Inline copy for a failed plan decision. Phase-start refusals show the backend's own text. */
export function planDecisionErrorMessage(failure: PlanDecisionFailure): string {
  const code = backendErrorCode(failure.error)
  // Phase-start refusals carry the backend's own explanation; show it without the code prefix.
  if (code === 'REQUEST_TRANSITION_NOT_ALLOWED' || code.startsWith('REQUEST_PHASE_')) {
    return splitErrorCode(failure.error.message ?? '').rest || requestErrorMessage('invalid_state')
  }
  if (failure.errorClass === 'notApprover' || failure.errorClass === 'forbidden') {
    return translate(`${P}PlanApprovalBar.notApprover`, 'You are not allowed to approve this.')
  }
  if (failure.errorClass === 'conflict') {
    return translate(
      `${P}PlanApprovalBar.conflict`,
      'This changed while you were reviewing it. The latest version was loaded; review it again.'
    )
  }
  if (failure.errorClass === 'alreadyDecided') {
    return translate(`${P}PlanApprovalBar.alreadyDecided`, 'This was already decided.')
  }
  if (failure.errorClass === 'expired') {
    return translate(
      `${P}PlanApprovalBar.expired`,
      'This approval expired. The latest version was loaded.'
    )
  }
  if (failure.errorClass === 'rateLimited') {
    return requestErrorMessage('rate_limited')
  }
  return requestErrorMessage(failure.error.kind)
}
