/**
 * useRequest — CR-REQ-018-03
 *
 * Fetches a single OrcaRequest by id, including its type history and links.
 *
 * @module hooks/useRequest
 */

import { useState, useEffect, useCallback } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { parseRequest, parseRequestTypeHistoryEntry } from '../../../shared/request-wire-parsers'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { useAppStore } from '../store'
import type { OrcaRequest, RequestTypeHistoryEntry, RequestLink } from '../../../shared/request-types'

type UseRequestResult = {
  request: OrcaRequest | null
  history: RequestTypeHistoryEntry[]
  links: RequestLink[]
  linksSupported: boolean
  isLoading: boolean
  error: string | null
  refetch: () => void
}

export function useRequest(id: string | null): UseRequestResult {
  const upsertRequests = useAppStore((s) => s.upsertRequests)

  const [request, setRequest] = useState<OrcaRequest | null>(null)
  const [history, setHistory] = useState<RequestTypeHistoryEntry[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [refetchTrigger, setRefetchTrigger] = useState(0)

  const refetch = useCallback(() => setRefetchTrigger((n) => n + 1), [])

  useEffect(() => {
    if (!id) {
      setRequest(null)
      setHistory([])
      return
    }

    let cancelled = false
    setIsLoading(true)
    setError(null)

    Promise.all([
      callRequestRpc<unknown>(REQUEST_RPC_METHODS.GET, { id }),
      callRequestRpc<{ entries: unknown[] }>(REQUEST_RPC_METHODS.TYPE_HISTORY, { id })
    ]).then(([requestResult, historyResult]) => {
      if (cancelled) return
      setIsLoading(false)

      if (!requestResult.ok) {
        setError(requestResult.error.kind)
        return
      }

      const parsed = parseRequest(requestResult.value)
      setRequest(parsed)
      upsertRequests([parsed])

      if (historyResult.ok) {
        setHistory((historyResult.value.entries ?? []).map(parseRequestTypeHistoryEntry))
      }
    })

    return () => { cancelled = true }
  }, [id, refetchTrigger, upsertRequests])

  const links = request?.links ?? []
  // links is 'supported' when the request.get response includes a links field
  const linksSupported = Array.isArray(request?.links)

  return {
    request,
    history,
    links,
    linksSupported,
    isLoading,
    error,
    refetch
  }
}
