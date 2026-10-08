/**
 * useRequests — CR-REQ-018-03
 *
 * Fetches a paginated list of OrcaRequest items, deduplicates by id,
 * and keeps the store updated via upsertRequests.
 *
 * @module hooks/useRequests
 */

import { useState, useEffect, useCallback, useRef } from 'react'
import { useAppStore } from '../store'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { parseRequest } from '../../../shared/request-wire-parsers'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import type { OrcaRequest, RequestStatus, RequestType, RequestSourceProvider } from '../../../shared/request-types'

export type RequestListFilters = {
  projectId?: string
  status?: RequestStatus[]
  type?: RequestType[]
  sourceProvider?: RequestSourceProvider
  pageSize?: number
}

type UseRequestsResult = {
  requests: OrcaRequest[]
  isLoading: boolean
  error: string | null
  /** True once the first fetch for the current filters has settled (avoids an empty-state flash). */
  hasLoaded: boolean
  nextPageToken: string | null
  nextPage: () => void
  refetch: () => void
  supported: boolean
}

export function useRequests(filters: RequestListFilters = {}): UseRequestsResult {
  const upsertRequests = useAppStore((s) => s.upsertRequests)
  const requestFlowSupport = useAppStore((s) => s.requestFlowSupport)

  const [requests, setRequests] = useState<OrcaRequest[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [hasLoaded, setHasLoaded] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [nextPageToken, setNextPageToken] = useState<string | null>(null)
  const [refetchTrigger, setRefetchTrigger] = useState(0)
  const [pageTokens, setPageTokens] = useState<string[]>([]) // stack of page tokens to fetch

  const filtersKey = JSON.stringify(filters)
  const filtersKeyRef = useRef(filtersKey)

  const refetch = useCallback(() => setRefetchTrigger((n) => n + 1), [])

  const nextPage = useCallback(() => {
    if (nextPageToken) {
      setPageTokens((prev) => [...prev, nextPageToken])
    }
  }, [nextPageToken])

  // Reset when filters change
  useEffect(() => {
    if (filtersKeyRef.current !== filtersKey) {
      filtersKeyRef.current = filtersKey
      setRequests([])
      setHasLoaded(false)
      setPageTokens([])
      setNextPageToken(null)
    }
  }, [filtersKey])

  useEffect(() => {
    if (requestFlowSupport === 'unsupported') {return}

    let cancelled = false
    setIsLoading(true)
    setError(null)

    const currentPageToken = pageTokens.at(-1)

    callRequestRpc<{ requests: unknown[]; nextPageToken?: string }>(
      REQUEST_RPC_METHODS.LIST,
      {
        ...filters,
        pageSize: filters.pageSize ?? 50,
        ...(currentPageToken ? { pageToken: currentPageToken } : {})
      }
    ).then((result) => {
      if (cancelled) {return}
      setIsLoading(false)
      setHasLoaded(true)

      if (!result.ok) {
        setError(result.error.kind)
        return
      }

      const parsed = (result.value.requests ?? []).map(parseRequest)
      upsertRequests(parsed)

      // Deduplicate across pages by id
      if (currentPageToken) {
        setRequests((prev) => {
          const seen = new Set(prev.map((r) => r.id))
          const newItems = parsed.filter((r) => !seen.has(r.id))
          return [...prev, ...newItems]
        })
      } else {
        setRequests(parsed)
      }

      setNextPageToken(result.value.nextPageToken ?? null)
    })

    return () => { cancelled = true }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filtersKey, refetchTrigger, pageTokens.length, requestFlowSupport])

  return {
    requests,
    isLoading,
    hasLoaded,
    error,
    nextPageToken,
    nextPage,
    refetch,
    supported: requestFlowSupport === 'supported'
  }
}
