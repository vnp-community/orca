/**
 * plan-decision-error-message.ts — CR-REQ-021-04
 *
 * @module components/request/plan/plan-decision-error-message
 */

import { translate } from '@/i18n/i18n'
import { requestErrorMessage, requestErrorMessageForError } from '../request-error-message'
import { splitErrorCode } from '../../../../../shared/request-errors'
import {
  isRunAllAlreadyPinned,
  isRunAllProgrammingError
} from '../../../../../shared/request-run-all'
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
  // Why: approving a plan starts the run as the approver, so someone who cannot execute is refused up front.
  if (code === 'REQUEST_EXECUTION_ACTOR_NOT_ALLOWED') {
    return translate(
      `${P}PlanApprovalBar.executionNotAllowed`,
      'You can approve plans only if you are allowed to run their execution. Ask someone who is, or request access.'
    )
  }
  // Why: the Request was pinned to step by step on an earlier run; "run all" cannot apply, a normal approval still can.
  if (isRunAllAlreadyPinned(failure.error)) {
    return translate(
      `${P}PlanApprovalBar.runAllAlreadyPinned`,
      'This request already runs step by step because it was started that way before. Approve it normally.'
    )
  }
  if (isRunAllProgrammingError(failure.error)) {
    return translate(
      `${P}PlanApprovalBar.runAllNotPossible`,
      "Couldn't start the run this way. Approve it normally."
    )
  }
  // Execution waits for the test specs generated beside the Plan; the panel above the Plan shows their state.
  if (code === 'REQUEST_TESTSPEC_NOT_READY') {
    return translate(
      `${P}testSpecNotReady`,
      'The test specs are not finished. Execution starts once they are.'
    )
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
  return requestErrorMessageForError(failure.error)
}
