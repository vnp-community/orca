/**
 * Solution error classification — CR-REQ-020-04
 *
 * request-errors.ts maps only some backend codes to a kind; the decision flow
 * also needs the finer APPROVAL_* / SOLUTION_* codes, so match on the code here.
 *
 * @module components/request/solution/solution-error-classification
 */

import { splitErrorCode } from '../../../../../shared/request-errors'
import type { RequestRpcError } from '../../../../../shared/request-errors'

export type SolutionErrorClass =
  | 'conflict'
  | 'alreadyDecided'
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

export function classifySolutionError(error: Pick<RequestRpcError, 'kind' | 'code' | 'message'>): SolutionErrorClass {
  const code = backendErrorCode(error)
  if (code === 'APPROVAL_VERSION_CONFLICT' || code === 'SOLUTION_VERSION_CONFLICT') {return 'conflict'}
  if (code === 'APPROVAL_ALREADY_DECIDED') {return 'alreadyDecided'}
  if (code === 'APPROVAL_EXPIRED') {return 'expired'}
  if (code === 'APPROVAL_NOT_APPROVER') {return 'notApprover'}
  if (code === 'REQUEST_RATE_LIMITED') {return 'rateLimited'}
  switch (error.kind) {
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
