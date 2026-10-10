/**
 * useSolutionStart — TASK-FE-E2E-009
 *
 * Explicit "Generate solution" for a Request that sits in `analyzing` with nothing running.
 * The backend may start the run on its own after Clarification, so the call re-reads the run
 * list first and never starts a second run.
 *
 * @module components/request/solution/useSolutionStart
 */

import { useCallback, useRef, useState } from 'react'
import type { Result } from '../../../runtime/request-rpc-client'
import type { RequestRpcError } from '../../../../../shared/request-errors'

export type UseSolutionStartParams = {
  generateWhenIdle: () => Promise<Result<{ skipped: boolean }>>
  /** Refetch solutions/approvals after the call so the panel sees the new run. */
  onSettled: () => void
}

export function useSolutionStart({ generateWhenIdle, onSettled }: UseSolutionStartParams) {
  const [submitting, setSubmitting] = useState(false)
  // Why: a started run is only visible after the next list read; until then do not offer the button again.
  const [started, setStarted] = useState(false)
  const [failure, setFailure] = useState<RequestRpcError | null>(null)
  const busy = useRef(false)

  const start = useCallback(async (): Promise<void> => {
    // Why: a ref blocks double clicks synchronously; state lags a render behind.
    if (busy.current) {
      return
    }
    busy.current = true
    setSubmitting(true)
    setFailure(null)
    try {
      const res = await generateWhenIdle()
      if (res.ok) {
        setStarted(true)
      } else {
        setFailure(res.error)
      }
      onSettled()
    } finally {
      busy.current = false
      setSubmitting(false)
    }
  }, [generateWhenIdle, onSettled])

  return { start, submitting, started, failure }
}
