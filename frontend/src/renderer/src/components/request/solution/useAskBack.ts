/**
 * useAskBack — CONTRACT-request-ui-api 2.6
 *
 * Sends `clarification.request` for a Solution's blocking open questions. One call at a
 * time (sync ref); the server returns the same Clarification for a repeated call.
 *
 * @module components/request/solution/useAskBack
 */

import { useCallback, useRef, useState } from 'react'
import { callRequestRpc, type Result } from '../../../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../../../shared/request-rpc-methods'
import { buildAskBackQuestions, type AskBackItem } from './solution-ask-back'

export function useAskBack(params: { requestId: string; solutionId: string }) {
  const { requestId, solutionId } = params
  const [submitting, setSubmitting] = useState(false)
  const busy = useRef(false)

  const ask = useCallback(
    async (items: AskBackItem[]): Promise<Result<unknown>> => {
      if (busy.current) {
        return {
          ok: false,
          error: {
            kind: 'invalid_state',
            code: 'SUBMIT_IN_FLIGHT',
            message: 'A submit is already in flight'
          }
        }
      }
      busy.current = true
      setSubmitting(true)
      try {
        return await callRequestRpc<unknown>(REQUEST_RPC_METHODS.CLARIFICATION_REQUEST, {
          requestId,
          source: 'solution_open_question',
          sourceRef: solutionId,
          questions: buildAskBackQuestions(items)
        })
      } finally {
        busy.current = false
        setSubmitting(false)
      }
    },
    [requestId, solutionId]
  )

  return { ask, submitting }
}
