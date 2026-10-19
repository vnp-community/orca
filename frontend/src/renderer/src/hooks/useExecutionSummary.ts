/**
 * useExecutionSummary — FE-REQ-TASK-037-02
 *
 * How the run of a request is going (progress, running and failed tasks, what waits for the viewer), from
 * `request.executionSummary`. It refreshes on every event of the request and, while enabled, on a timer that stops when the
 * page is hidden. A gateway without the channel is `unsupported`, not an error.
 *
 * @module hooks/useExecutionSummary
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { parseExecutionSummary } from '../../../shared/request-execution-parsers'
import type { ExecutionSummary } from '../../../shared/request-execution-types'
import { useRequestSubscription } from './useRequestSubscription'

export const EXECUTION_SUMMARY_POLL_MS = 5000

type Options = {
  /** Reads happen only while enabled (the tab is open and the request is in a state worth following). */
  enabled?: boolean
  /** Poll interval while enabled; 0 turns polling off and leaves the events. */
  pollMs?: number
}

export type UseExecutionSummaryResult = {
  summary: ExecutionSummary | null
  isLoading: boolean
  error: string | null
  /** The gateway has no such channel (an older runtime). */
  unsupported: boolean
  refetch: () => void
}

function pageVisible(): boolean {
  return typeof document === 'undefined' || document.visibilityState === 'visible'
}

export function useExecutionSummary(
  requestId: string | null,
  options: Options = {}
): UseExecutionSummaryResult {
  const { enabled = true, pollMs = EXECUTION_SUMMARY_POLL_MS } = options
  const [summary, setSummary] = useState<ExecutionSummary | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [unsupported, setUnsupported] = useState(false)
  const generation = useRef(0)

  const load = useCallback(() => {
    if (!requestId || !enabled) {
      return
    }
    const mine = ++generation.current
    setIsLoading(true)
    void callRequestRpc<{ summary?: unknown }>(REQUEST_RPC_METHODS.EXECUTION_SUMMARY, {
      id: requestId
    }).then((res) => {
      if (mine !== generation.current) {
        return // a newer read is on its way, or the request changed
      }
      setIsLoading(false)
      if (!res.ok) {
        if (res.error.kind === 'unsupported') {
          setUnsupported(true)
          setError(null)
        } else {
          setError(res.error.kind)
        }
        return
      }
      setUnsupported(false)
      setError(null)
      setSummary(parseExecutionSummary(res.value?.summary))
    })
  }, [requestId, enabled])

  useEffect(() => {
    setSummary(null)
    setError(null)
    setUnsupported(false)
    load()
    return () => {
      generation.current++
    }
  }, [load])

  useRequestSubscription({ requestId, onEvent: () => load() })

  useEffect(() => {
    if (!requestId || !enabled || pollMs <= 0 || unsupported) {
      return undefined
    }
    const timer = setInterval(() => {
      if (pageVisible()) {
        load()
      }
    }, pollMs)
    return () => clearInterval(timer)
  }, [requestId, enabled, pollMs, unsupported, load])

  return { summary, isLoading, error, unsupported, refetch: load }
}
