/**
 * useBacklog — CR-REQ-018-03
 *
 * Fetches backlog items for one of three views: requests, tasks, execute.
 *
 * @module hooks/useBacklog
 */

import { useState, useEffect, useCallback } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { parseBacklogItem } from '../../../shared/request-wire-parsers'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import type { BacklogView, BacklogItem } from '../../../shared/request-types'

const VIEW_TO_METHOD: Record<BacklogView, string> = {
  requests: REQUEST_RPC_METHODS.BACKLOG_REQUESTS,
  tasks: REQUEST_RPC_METHODS.BACKLOG_TASKS,
  execute: REQUEST_RPC_METHODS.BACKLOG_EXECUTE
}

type UseBacklogResult = {
  items: BacklogItem[]
  isLoading: boolean
  error: string | null
  nextPage: () => void
  nextPageToken: string | null
  refetch: () => void
}

export function useBacklog(view: BacklogView, filters: object = {}): UseBacklogResult {
  const [items, setItems] = useState<BacklogItem[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [nextPageToken, setNextPageToken] = useState<string | null>(null)
  const [refetchTrigger, setRefetchTrigger] = useState(0)

  const refetch = useCallback(() => setRefetchTrigger((n) => n + 1), [])
  const nextPage = useCallback(() => {
    if (nextPageToken) setRefetchTrigger((n) => n + 1)
  }, [nextPageToken])

  useEffect(() => {
    let cancelled = false
    setIsLoading(true)
    setError(null)

    const method = VIEW_TO_METHOD[view]
    callRequestRpc<{ items: unknown[]; nextPageToken?: string }>(method as never, filters).then((result) => {
      if (cancelled) return
      setIsLoading(false)

      if (!result.ok) {
        setError(result.error.kind)
        return
      }

      setItems((result.value.items ?? []).map((item) => parseBacklogItem(view, item)))
      setNextPageToken(result.value.nextPageToken ?? null)
    })

    return () => { cancelled = true }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [view, refetchTrigger])

  return { items, isLoading, error, nextPage, nextPageToken, refetch }
}
