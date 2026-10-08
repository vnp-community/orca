/**
 * useRequestSummaries — CR-REQ-022-03
 *
 * approval.listPending and backlog.* rows carry no Request title/type, so the
 * screens resolve them through request.get with a small concurrency cap and
 * the shared requestsById cache.
 *
 * @module hooks/useRequestSummaries
 */

import { useEffect, useMemo, useRef, useState } from 'react'
import { useAppStore } from '../store'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { parseRequest } from '../../../shared/request-wire-parsers'
import type { OrcaRequest } from '../../../shared/request-types'

export const REQUEST_SUMMARY_CONCURRENCY = 4
export const REQUEST_SUMMARY_TTL_MS = 5 * 60_000

type Failure = { notFound: boolean }

export type RequestSummaries = {
  byId: Record<string, OrcaRequest>
  isLoading: boolean
  failedIds: Set<string>
  /** Subset of failedIds the server said no longer exist. */
  notFoundIds: Set<string>
}

export function useRequestSummaries(requestIds: string[]): RequestSummaries {
  const byId = useAppStore((s) => s.requestsById)
  const upsertRequests = useAppStore((s) => s.upsertRequests)
  const fetchedAt = useRef(new Map<string, number>())
  const failures = useRef(new Map<string, Failure>())
  const [isLoading, setIsLoading] = useState(false)
  const [failureVersion, setFailureVersion] = useState(0)

  // Why: callers pass a fresh array every render; key by content so the effect only reruns on real changes.
  const key = useMemo(() => [...new Set(requestIds.filter(Boolean))].join(','), [requestIds])

  useEffect(() => {
    if (!key) {return}
    const ids = key.split(',')
    const now = Date.now()
    const queue = ids.filter((id) => {
      if (failures.current.has(id)) {return false}
      const at = fetchedAt.current.get(id)
      if (at !== undefined) {return now - at > REQUEST_SUMMARY_TTL_MS}
      return useAppStore.getState().requestsById[id] === undefined
    })
    if (queue.length === 0) {return}

    let cancelled = false
    setIsLoading(true)
    let cursor = 0

    async function worker(): Promise<void> {
      while (!cancelled && cursor < queue.length) {
        const id = queue[cursor++]
        const result = await callRequestRpc<{ request?: unknown } & Record<string, unknown>>(
          REQUEST_RPC_METHODS.GET,
          { id }
        )
        if (cancelled) {return}
        if (result.ok) {
          fetchedAt.current.set(id, Date.now())
          upsertRequests([parseRequest(result.value.request ?? result.value)])
        } else {
          failures.current.set(id, { notFound: (result.error.message.split(':')[0] ?? '').endsWith('NOT_FOUND') || result.error.kind === 'not_found' })
          setFailureVersion((v) => v + 1)
        }
      }
    }

    void Promise.all(
      Array.from({ length: Math.min(REQUEST_SUMMARY_CONCURRENCY, queue.length) }, () => worker())
    ).then(() => {
      if (!cancelled) {setIsLoading(false)}
    })

    return () => {
      cancelled = true
    }
  }, [key, upsertRequests])

  return useMemo(() => {
    const failedIds = new Set<string>()
    const notFoundIds = new Set<string>()
    for (const [id, f] of failures.current) {
      failedIds.add(id)
      if (f.notFound) {notFoundIds.add(id)}
    }
    return { byId, isLoading, failedIds, notFoundIds }
    // failureVersion invalidates the memo when the ref-held failure map changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [byId, isLoading, failureVersion])
}
