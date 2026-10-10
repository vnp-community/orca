/**
 * useSolutions — CR-REQ-018-03
 *
 * Fetches solutions for a request and provides generate/choose actions.
 *
 * @module hooks/useSolutions
 */

import { useState, useEffect, useCallback } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { latestAnalysisRun, parseAnalysisRun } from '../../../shared/request-analysis-run-parsers'
import { parseSolution } from '../../../shared/request-wire-parsers'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import type { Result } from '../runtime/request-rpc-client'
import type { AnalysisRun, Solution } from '../../../shared/request-types'

type UseSolutionsResult = {
  solutions: Solution[]
  /** The newest AI run for this request, or null before any run. A failed run is the only record of why nothing was generated. */
  latestRun: AnalysisRun | null
  isLoading: boolean
  error: string | null
  generate: (params?: { feedback?: string; idempotencyKey?: string }) => Promise<Result<unknown>>
  /** Generates only when a fresh read shows no Solution and no running run; otherwise reports `skipped`. */
  generateWhenIdle: () => Promise<Result<{ skipped: boolean }>>
  choose: (params: {
    solutionId: string
    optionId: string
    comment?: string
  }) => Promise<Result<unknown>>
  refetch: () => void
}

export function useSolutions(requestId: string | null): UseSolutionsResult {
  const [solutions, setSolutions] = useState<Solution[]>([])
  const [latestRun, setLatestRun] = useState<AnalysisRun | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [refetchTrigger, setRefetchTrigger] = useState(0)

  const refetch = useCallback(() => setRefetchTrigger((n) => n + 1), [])

  useEffect(() => {
    if (!requestId) {
      setSolutions([])
      setLatestRun(null)
      return
    }

    let cancelled = false
    setIsLoading(true)
    setError(null)

    callRequestRpc<{ solutions: unknown[]; runs?: unknown[] }>(REQUEST_RPC_METHODS.SOLUTION_LIST, {
      requestId
    }).then((result) => {
      if (cancelled) {
        return
      }
      setIsLoading(false)

      if (!result.ok) {
        setError(result.error.kind)
        return
      }

      setSolutions((result.value.solutions ?? []).map(parseSolution))
      setLatestRun(latestAnalysisRun((result.value.runs ?? []).map(parseAnalysisRun)))
    })

    return () => {
      cancelled = true
    }
  }, [requestId, refetchTrigger])

  const generate = useCallback(
    async (params?: { feedback?: string; idempotencyKey?: string }): Promise<Result<unknown>> => {
      const idempotencyKey = params?.idempotencyKey ?? crypto.randomUUID()
      return callRequestRpc(REQUEST_RPC_METHODS.SOLUTION_GENERATE, {
        requestId,
        feedback: params?.feedback,
        idempotencyKey
      })
    },
    [requestId]
  )

  const generateWhenIdle = useCallback(async (): Promise<Result<{ skipped: boolean }>> => {
    // Why: the cached list may predate a run the backend started on its own; a second run would duplicate it.
    const listed = await callRequestRpc<{ solutions: unknown[]; runs?: unknown[] }>(
      REQUEST_RPC_METHODS.SOLUTION_LIST,
      { requestId }
    )
    if (!listed.ok) {
      return listed
    }
    const run = latestAnalysisRun((listed.value.runs ?? []).map(parseAnalysisRun))
    if ((listed.value.solutions ?? []).length > 0 || run?.status === 'running') {
      return { ok: true, value: { skipped: true } }
    }
    const res = await generate()
    return res.ok ? { ok: true, value: { skipped: false } } : res
  }, [requestId, generate])

  const choose = useCallback(
    async (params: {
      solutionId: string
      optionId: string
      comment?: string
      rationale?: string
    }): Promise<Result<unknown>> => {
      return callRequestRpc(REQUEST_RPC_METHODS.SOLUTION_CHOOSE, { requestId, ...params })
    },
    [requestId]
  )

  return { solutions, latestRun, isLoading, error, generate, generateWhenIdle, choose, refetch }
}
