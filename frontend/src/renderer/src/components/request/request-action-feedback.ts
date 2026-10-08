/**
 * request-action-feedback.ts — CR-REQ-019-03
 *
 * Shared failure handling for request mutations: hooks never toast, so
 * components route failures through here.
 *
 * @module components/request/request-action-feedback
 */

import { toast } from 'sonner'
import { requestErrorMessage } from './request-error-message'
import type { RequestRpcError } from '../../../../shared/request-errors'

/** Toast the failure; for stale-state kinds also reload so the UI matches the server. */
export function notifyRequestActionFailure(error: RequestRpcError, refetch: () => void): void {
  toast.error(requestErrorMessage(error.kind))
  if (error.kind === 'conflict' || error.kind === 'invalid_state') {refetch()}
}
