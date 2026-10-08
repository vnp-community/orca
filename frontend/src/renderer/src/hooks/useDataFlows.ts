/**
 * useDataFlows.ts — FE-CV-TASK-056-05
 *
 * Server-side search/filter (debounced) with opaque pageToken paging. We never filter the
 * loaded page on the client: the server decides what `query` matches.
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import type { DataFlowSummary } from '../../../shared/code-intel-architecture-types'
import { CODE_INTEL_RPC_METHODS } from '../../../shared/code-intel-rpc-methods'
import { getCodeIntelClient } from '../runtime/code-intel-client'
import type { CodeIntelRpcError } from '../runtime/code-intel-client'

export const DATA_FLOWS_PAGE_SIZE = 100
export const DATA_FLOWS_QUERY_DEBOUNCE_MS = 300
const QUERY_MAX_CHARS = 128

export type DataFlowsFilter = { query: string; triggerKind: string | null; service: string | null }

export type UseDataFlowsResult = {
  status: 'loading' | 'ready' | 'error'
  flows: DataFlowSummary[]
  total: number
  hasMore: boolean
  loadingMore: boolean
  error: CodeIntelRpcError | null
  loadMore: () => void
  refetch: () => void
}

export function useDataFlows(
  worktreeId: string,
  environmentId: string | null,
  filter: DataFlowsFilter
): UseDataFlowsResult {
  const [debounced, setDebounced] = useState(filter.query)
  const [state, setState] = useState<{
    status: UseDataFlowsResult['status']
    flows: DataFlowSummary[]
    total: number
    token: string | null
    error: CodeIntelRpcError | null
  }>({ status: 'loading', flows: [], total: 0, token: null, error: null })
  const [loadingMore, setLoadingMore] = useState(false)
  const [tick, setTick] = useState(0)
  const ctrlRef = useRef<AbortController | null>(null)

  useEffect(() => {
    const t = setTimeout(() => setDebounced(filter.query), DATA_FLOWS_QUERY_DEBOUNCE_MS)
    return () => clearTimeout(t)
  }, [filter.query])

  const request = useCallback(
    async (pageToken: string | null, signal: AbortSignal) => {
      const q = debounced.trim().slice(0, QUERY_MAX_CHARS)
      return getCodeIntelClient().callEnvelope<{ flows: DataFlowSummary[] }>(
        worktreeId,
        CODE_INTEL_RPC_METHODS.DATA_FLOWS,
        {
          limit: DATA_FLOWS_PAGE_SIZE,
          ...(q ? { query: q } : {}),
          ...(filter.triggerKind ? { triggerKind: filter.triggerKind } : {}),
          ...(filter.service ? { service: filter.service } : {}),
          ...(pageToken ? { pageToken } : {})
        },
        (raw) => {
          const flows = (raw as { flows?: unknown } | null)?.flows
          return { flows: Array.isArray(flows) ? (flows as DataFlowSummary[]) : [] }
        },
        { environmentId, signal }
      )
    },
    [worktreeId, environmentId, debounced, filter.triggerKind, filter.service]
  )

  useEffect(() => {
    const ctrl = new AbortController()
    ctrlRef.current?.abort()
    ctrlRef.current = ctrl
    setState({ status: 'loading', flows: [], total: 0, token: null, error: null })
    setLoadingMore(false)
    void (async () => {
      try {
        const res = await request(null, ctrl.signal)
        if (ctrl.signal.aborted) {
          return
        }
        if (!res.ok) {
          setState({ status: 'error', flows: [], total: 0, token: null, error: res.error })
          return
        }
        const flows = res.envelope.data?.flows ?? []
        setState({
          status: 'ready',
          flows,
          total: res.envelope.totalCount || flows.length,
          token: res.envelope.nextPageToken ?? null,
          error: null
        })
      } catch (err) {
        if (!ctrl.signal.aborted) {
          setState({
            status: 'error', flows: [], total: 0, token: null,
            error: { kind: 'unknown', code: null, message: err instanceof Error ? err.message : 'failed', data: null, retryable: true }
          })
        }
      }
    })()
    return () => ctrl.abort()
  }, [request, tick])

  const loadMore = useCallback(() => {
    const token = state.token
    const ctrl = ctrlRef.current
    if (!token || loadingMore || !ctrl || ctrl.signal.aborted) {
      return
    }
    setLoadingMore(true)
    void (async () => {
      try {
        const res = await request(token, ctrl.signal)
        if (ctrl.signal.aborted) {
          return
        }
        if (res.ok) {
          const next = res.envelope.data?.flows ?? []
          setState((prev) => {
            const seen = new Set(prev.flows.map((f) => f.id))
            return {
              ...prev,
              flows: [...prev.flows, ...next.filter((f) => !seen.has(f.id))],
              token: res.envelope.nextPageToken ?? null
            }
          })
        } else {
          setState((prev) => ({ ...prev, error: res.error }))
        }
      } finally {
        if (!ctrl.signal.aborted) {
          setLoadingMore(false)
        }
      }
    })()
  }, [state.token, loadingMore, request])

  const refetch = useCallback(() => setTick((n) => n + 1), [])

  return {
    status: state.status,
    flows: state.flows,
    total: state.total,
    hasMore: state.token !== null,
    loadingMore,
    error: state.error,
    loadMore,
    refetch
  }
}
