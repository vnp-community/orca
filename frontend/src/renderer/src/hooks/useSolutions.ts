/**
 * useSolutions — CR-REQ-018-03
 *
 * Fetches solutions for a request and provides generate/choose actions.
 *
 * @module hooks/useSolutions
 */

import { useState, useEffect, useCallback } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { parseSolution } from '../../../shared/request-wire-parsers'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import type { Result } from '../runtime/request-rpc-client'
import type { Solution } from '../../../shared/request-types'

type UseSolutionsResult = {
  solutions: Solution[]
  isLoading: boolean
  error: string | null
  generate: (params?: { feedback?: string; idempotencyKey?: string }) => Promise<Result<unknown>>
  choose: (params: { solutionId: string; optionId: string; comment?: string }) => Promise<Result<unknown>>
  refetch: () => void
}

export function useSolutions(requestId: string | null): UseSolutionsResult {
  const [solutions, setSolutions] = useState<Solution[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [refetchTrigger, setRefetchTrigger] = useState(0)

  const refetch = useCallback(() => setRefetchTrigger((n) => n + 1), [])

  useEffect(() => {
    if (!requestId) {
      setSolutions([])
      return
    }

    let cancelled = false
    setIsLoading(true)
    setError(null)

    callRequestRpc<{ solutions: unknown[] }>(REQUEST_RPC_METHODS.SOLUTION_LIST, { requestId }).then((result) => {
      if (cancelled) {return}
      setIsLoading(false)

      if (!result.ok) {
        setError(result.error.kind)
        return
      }

      setSolutions((result.value.solutions ?? []).map(parseSolution))
    })

    return () => { cancelled = true }
  }, [requestId, refetchTrigger])

  const generate = useCallback(async (params?: { feedback?: string; idempotencyKey?: string }): Promise<Result<unknown>> => {
    const idempotencyKey = params?.idempotencyKey ?? crypto.randomUUID()
    return callRequestRpc(REQUEST_RPC_METHODS.SOLUTION_GENERATE, {
      requestId,
      feedback: params?.feedback,
      idempotencyKey
    })
  }, [requestId])

  const choose = useCallback(async (params: {
    solutionId: string
    optionId: string
    comment?: string
    rationale?: string
  }): Promise<Result<unknown>> => {
    return callRequestRpc(REQUEST_RPC_METHODS.SOLUTION_CHOOSE, { requestId, ...params })
  }, [requestId])

  return { solutions, isLoading, error, generate, choose, refetch }
}
