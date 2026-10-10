/**
 * Solution error classification — CR-REQ-020-04
 *
 * request-errors.ts maps only some backend codes to a kind; the decision flow
 * also needs the finer APPROVAL_* / SOLUTION_* codes, so match on the code here.
 *
 * @module components/request/solution/solution-error-classification
 */

import { canonicalApprovalSolutionCode, splitErrorCode } from '../../../../../shared/request-errors'
import type { RequestRpcError } from '../../../../../shared/request-errors'

export type SolutionErrorClass =
  | 'conflict'
  | 'alreadyDecided'
  | 'blockingQuestions'
  | 'unconfirmedAssumptions'
  | 'expired'
  | 'notApprover'
  | 'rateLimited'
  | 'forbidden'
  | 'validation'
  | 'network'
  | 'other'

export function backendErrorCode(error: Pick<RequestRpcError, 'code' | 'message'>): string {
  return (splitErrorCode(error.message ?? '').code || error.code || '').toUpperCase()
}

export function classifySolutionError(
  error: Pick<RequestRpcError, 'kind' | 'code' | 'message'>
): SolutionErrorClass {
  // Why: the server sends REQUEST_APPROVAL_*; the short spelling predates the CONTRACT prefix.
  const rawCode = backendErrorCode(error)
  const code = canonicalApprovalSolutionCode(rawCode)
  if (code === 'APPROVAL_VERSION_CONFLICT' || code === 'SOLUTION_VERSION_CONFLICT') {
    return 'conflict'
  }
  if (code === 'APPROVAL_ALREADY_DECIDED') {
    return 'alreadyDecided'
  }
  if (code === 'SOLUTION_BLOCKING_QUESTIONS') {
    return 'blockingQuestions'
  }
  // Why: the Solution on screen was superseded by a newer one (e.g. after a Clarification answer); re-read, do not show a raw error.
  if (code === 'SOLUTION_NOT_PROPOSED') {
    return 'conflict'
  }
  if (rawCode === 'REQUEST_PLAN_UNCONFIRMED_ASSUMPTIONS') {
    return 'unconfirmedAssumptions'
  }
  if (code === 'APPROVAL_EXPIRED') {
    return 'expired'
  }
  if (code === 'APPROVAL_NOT_APPROVER') {
    return 'notApprover'
  }
  if (rawCode === 'REQUEST_RATE_LIMITED') {
    return 'rateLimited'
  }
  switch (error.kind) {
    case 'blocking_questions':
      return 'blockingQuestions'
    case 'unconfirmed_assumptions':
      return 'unconfirmedAssumptions'
    case 'conflict':
      return 'conflict'
    case 'invalid_state':
      return 'alreadyDecided'
    case 'rate_limited':
      return 'rateLimited'
    case 'forbidden':
      return 'forbidden'
    case 'validation':
      return 'validation'
    case 'network':
      return 'network'
    default:
      return 'other'
  }
}
