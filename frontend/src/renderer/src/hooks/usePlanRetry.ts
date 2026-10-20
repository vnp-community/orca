/**
 * usePlanRetry
 *
 * The Retry of a failed Plan run. When the server writes the Plan on its own it restarts there, so closing the tab cannot
 * strand the run; otherwise (switch off, older gateway) it falls back to the manual flow the Generate button uses.
 *
 * @module hooks/usePlanRetry
 */

import { useCallback, useRef, useState } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { PLAN_AUTO_DISABLED_CODE, type PlanRunView } from '../../../shared/request-plan-auto'
import { supportsPlanAuto } from '../store/slices/request'
import { useAppStore } from '../store'
import { backendErrorCode } from '../components/request/solution/solution-error-classification'
import { requestErrorMessageForError } from '../components/request/request-error-message'
import type { PlanDecisionOutcome } from './usePlanDecision'
import type { OrcaRequest } from '../../../shared/request-types'

export type PlanRetry = {
  retry: () => Promise<void>
  retrying: boolean
  /** Message of the last failed retry; null otherwise. */
  error: string | null
}

type Input = {
  request: Pick<OrcaRequest, 'id'>
  run: PlanRunView | null
  /** The manual generate-and-save flow. */
  regenerateManually: () => Promise<PlanDecisionOutcome>
  /** Read the run state again (usePlanRunState.refresh). */
  refresh: () => void
}

export function usePlanRetry({ request, run, regenerateManually, refresh }: Input): PlanRetry {
  const serverSupportsAuto = useAppStore(supportsPlanAuto)
  const [retrying, setRetrying] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const inFlight = useRef(false)
  const viaServer = serverSupportsAuto && run?.autoPlan?.enabled === true

  const retry = useCallback(async (): Promise<void> => {
    // Why: a ref blocks a second click synchronously; state lags one render behind.
    if (inFlight.current) {
      return
    }
    inFlight.current = true
    setRetrying(true)
    setError(null)
    try {
      if (viaServer) {
        // Idempotent: the server hands back the job already running when the button is pressed twice.
        const res = await callRequestRpc(
          REQUEST_RPC_METHODS.RETRY_PLAN,
          { id: request.id },
          { idempotent: true }
        )
        if (res.ok) {
          refresh()
          return
        }
        if ((res.error.backendCode || backendErrorCode(res.error)) !== PLAN_AUTO_DISABLED_CODE) {
          setError(requestErrorMessageForError(res.error))
          return
        }
      }
      const outcome = await regenerateManually()
      // IN_FLIGHT: a manual generate is already running; its own busy state tells the user.
      if (!outcome.ok && outcome.error && outcome.error.code !== 'IN_FLIGHT') {
        setError(requestErrorMessageForError(outcome.error))
      }
    } finally {
      inFlight.current = false
      setRetrying(false)
    }
  }, [viaServer, request.id, refresh, regenerateManually])

  return { retry, retrying, error }
}
