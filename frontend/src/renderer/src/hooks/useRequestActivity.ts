/**
 * useRequestActivity — FE-REQ-TASK-037-02
 *
 * The History of a request: what happened to it, newest first, read page by page from `request.activity`. Changing the filter
 * starts over. It refreshes on request events while enabled; a gateway without the channel is `unsupported`.
 *
 * @module hooks/useRequestActivity
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { parseActivityPage } from '../../../shared/request-execution-parsers'
import type {
  RequestActivityEntry,
  RequestActivityFilter
} from '../../../shared/request-execution-types'
import { useRequestSubscription } from './useRequestSubscription'

const PAGE_SIZE = 30

type Options = { enabled?: boolean; filter?: RequestActivityFilter }

export type UseRequestActivityResult = {
  entries: RequestActivityEntry[]
  hasMore: boolean
  isLoading: boolean
  isLoadingMore: boolean
  error: string | null
  unsupported: boolean
  loadMore: () => void
  refetch: () => void
}

function paramsOf(
  requestId: string,
  filter: RequestActivityFilter | undefined,
  pageToken: string | undefined
): object {
  return {
    id: requestId,
    pageSize: PAGE_SIZE,
    ...(pageToken ? { pageToken } : {}),
    ...(filter?.kinds?.length ? { kinds: filter.kinds } : {}),
    ...(filter?.minSeverity && filter.minSeverity !== 'info'
      ? { minSeverity: filter.minSeverity }
      : {}),
    ...(filter?.actorKind ? { actorKind: filter.actorKind } : {}),
    ...(filter?.subjectId ? { subjectId: filter.subjectId } : {})
  }
}

export function useRequestActivity(
  requestId: string | null,
  options: Options = {}
): UseRequestActivityResult {
  const { enabled = true, filter } = options
  const filterKey = JSON.stringify(filter ?? {})
  const [entries, setEntries] = useState<RequestActivityEntry[]>([])
  const [nextToken, setNextToken] = useState<string | undefined>()
  const [isLoading, setIsLoading] = useState(false)
  const [isLoadingMore, setIsLoadingMore] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [unsupported, setUnsupported] = useState(false)
  const generation = useRef(0)
  const filterRef = useRef(filter)
  filterRef.current = filter

  const loadFirst = useCallback(() => {
    if (!requestId || !enabled) {
      return
    }
    const mine = ++generation.current
    setIsLoading(true)
    void callRequestRpc<unknown>(
      REQUEST_RPC_METHODS.ACTIVITY,
      paramsOf(requestId, filterRef.current, undefined)
    ).then((res) => {
      if (mine !== generation.current) {
        return
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
      const page = parseActivityPage(res.value)
      setUnsupported(false)
      setError(null)
      setEntries(page.entries)
      setNextToken(page.nextPageToken)
    })
  }, [requestId, enabled])

  const loadMore = useCallback(() => {
    if (!requestId || !enabled || !nextToken || isLoadingMore) {
      return
    }
    const mine = generation.current
    setIsLoadingMore(true)
    void callRequestRpc<unknown>(
      REQUEST_RPC_METHODS.ACTIVITY,
      paramsOf(requestId, filterRef.current, nextToken)
    ).then((res) => {
      if (mine !== generation.current) {
        return
      }
      setIsLoadingMore(false)
      if (!res.ok) {
        setError(res.error.kind)
        return
      }
      const page = parseActivityPage(res.value)
      setEntries((cur) => {
        const seen = new Set(cur.map((e) => e.id))
        return [...cur, ...page.entries.filter((e) => !seen.has(e.id))]
      })
      setNextToken(page.nextPageToken)
    })
  }, [requestId, enabled, nextToken, isLoadingMore])

  useEffect(() => {
    setEntries([])
    setNextToken(undefined)
    setError(null)
    loadFirst()
    return () => {
      generation.current++
    }
  }, [loadFirst, filterKey])

  useRequestSubscription({ requestId, onEvent: () => loadFirst() })

  return {
    entries,
    hasMore: nextToken !== undefined,
    isLoading,
    isLoadingMore,
    error,
    unsupported,
    loadMore,
    refetch: loadFirst
  }
}
