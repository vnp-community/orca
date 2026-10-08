/**
 * useBacklog — CR-REQ-023-02
 *
 * Paged backlog.requests / backlog.tasks / backlog.execute. Each instance owns
 * the state of one view and only talks to the server while `active`, so
 * BacklogTab keeps one instance per view: switching views reuses the cache and
 * a failure in one view cannot blank another.
 *
 * @module hooks/useBacklog
 */

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useAppStore } from '../store'
import { callRequestRpc } from '../runtime/request-rpc-client'
import {
  BACKLOG_RPC_BY_VIEW, parseRequestBacklogPage, parseTaskBacklogPage,
  type BacklogGroupData, type BacklogPage, type RequestBacklogRowData
} from '../../../shared/request-backlog-types'
import { useRequestSubscription } from './useRequestSubscription'
import type { RequestRpcError } from '../../../shared/request-errors'
import type { BacklogView } from '../../../shared/request-types'

export const BACKLOG_POLL_MS = 30_000
export const BACKLOG_EVENT_DEBOUNCE_MS = 500
const MAX_EMPTY_PAGE_HOPS = 3

// Why: pageSize counts Requests for the grouped views (CR-015), so pages are smaller there.
const PAGE_SIZE: Record<BacklogView, number> = { requests: 50, tasks: 20, execute: 20 }

const REFRESH_EVENTS = new Set([
  'request.returned', 'request.status_changed', 'plan.generated', 'phase.started', 'phase.completed',
  'approval.decided'
])

export type BacklogFilters = { projectId?: string; planTaskId?: string; phaseTaskId?: string }
export type BacklogErrorKind = 'network' | 'forbidden' | 'unknown'
export type BacklogError = { kind: BacklogErrorKind; code: string }

export type BacklogState<T> = {
  items: T[]
  nextPageToken: string | null
  isLoading: boolean
  isLoadingMore: boolean
  error: BacklogError | null
  loadedOnce: boolean
  hasMore: boolean
  supported: boolean
  loadMore: () => void
  refetch: () => void
  /** Loaded rows/groups with `plus` when more pages exist (the server sends no total). */
  countLabel: () => { count: number; plus: boolean }
}

function eventSuffix(eventType: string): string {
  return eventType.replace(/^orca\.request\./, '')
}

function classify(error: RequestRpcError): BacklogError {
  const code = error.message.split(':')[0] ?? error.code
  if (error.kind === 'forbidden') {return { kind: 'forbidden', code }}
  // TASK_SERVICE_UNAVAILABLE has no partial result; it is surfaced like a network failure for that view only.
  if (error.kind === 'network' || error.kind === 'unavailable' || /BACKLOG_TASK_SERVICE_UNAVAILABLE/.test(code)) {
    return { kind: 'network', code }
  }
  return { kind: 'unknown', code }
}

function keyOfItem(view: BacklogView, item: RequestBacklogRowData | BacklogGroupData): string {
  if (view === 'requests') {return (item as RequestBacklogRowData).requestId}
  const g = item as BacklogGroupData
  return g.phaseTaskId ?? g.planTaskId ?? g.requestId
}

function useBacklogPages<T extends RequestBacklogRowData | BacklogGroupData>(
  view: BacklogView,
  filters: BacklogFilters,
  active: boolean
): BacklogState<T> {
  const setRequestFlowSupport = useAppStore((s) => s.setRequestFlowSupport)
  const supportState = useAppStore((s) => s.requestFlowSupport)
  const [items, setItems] = useState<T[]>([])
  const [nextPageToken, setNextPageToken] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [isLoadingMore, setIsLoadingMore] = useState(false)
  const [error, setError] = useState<BacklogError | null>(null)
  const [loadedOnce, setLoadedOnce] = useState(false)
  const [refetchKey, setRefetchKey] = useState(0)
  const generation = useRef(0)
  const alive = useRef(true)
  const tokenRetried = useRef(false)

  useEffect(() => {
    alive.current = true
    return () => { alive.current = false }
  }, [])

  // Compare filters by value so a new object each render does not refetch.
  const filterKey = JSON.stringify([filters.projectId, filters.planTaskId, filters.phaseTaskId])

  const fetchPage = useCallback(
    async (token: string | null): Promise<{ page: BacklogPage<T> } | { error: RequestRpcError }> => {
      let current = token
      let collected: T[] = []
      for (let hop = 0; hop <= MAX_EMPTY_PAGE_HOPS; hop++) {
        const params: Record<string, unknown> = { pageSize: PAGE_SIZE[view] }
        if (filters.projectId) {params.projectId = filters.projectId}
        if (view === 'tasks' && filters.planTaskId) {params.planTaskId = filters.planTaskId}
        if (view === 'execute' && filters.phaseTaskId) {params.phaseTaskId = filters.phaseTaskId}
        if (current) {params.pageToken = current}
        const result = await callRequestRpc<unknown>(BACKLOG_RPC_BY_VIEW[view], params)
        if (!result.ok) {return { error: result.error }}
        const page = (view === 'requests'
          ? parseRequestBacklogPage(result.value)
          : parseTaskBacklogPage(result.value)) as BacklogPage<T>
        collected = collected.concat(page.items)
        // An empty page with a token is legal (CR-015 2.5): keep going, but not forever.
        if (page.items.length > 0 || !page.nextPageToken || hop === MAX_EMPTY_PAGE_HOPS) {
          return { page: { items: collected, nextPageToken: page.nextPageToken } }
        }
        current = page.nextPageToken
      }
      return { page: { items: collected, nextPageToken: null } }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [view, filterKey]
  )

  const refetch = useCallback(() => {
    tokenRetried.current = false
    setRefetchKey((n) => n + 1)
  }, [])

  useEffect(() => {
    if (!active) {return}
    const gen = ++generation.current
    setIsLoading(true)
    void fetchPage(null).then((res) => {
      if (!alive.current || gen !== generation.current) {return}
      setIsLoading(false)
      if ('error' in res) {
        if (res.error.kind === 'unsupported') {setRequestFlowSupport('unsupported')}
        setError(classify(res.error)) // keep old items: the UI dims them
        return
      }
      setError(null)
      setItems(res.page.items)
      setNextPageToken(res.page.nextPageToken)
      setLoadedOnce(true)
    })
  }, [active, fetchPage, refetchKey, setRequestFlowSupport])

  const loadMore = useCallback(() => {
    if (!nextPageToken || isLoadingMore) {return}
    const gen = generation.current
    setIsLoadingMore(true)
    void fetchPage(nextPageToken).then((res) => {
      if (!alive.current || gen !== generation.current) {return}
      setIsLoadingMore(false)
      if ('error' in res) {
        // A stale cursor (data changed under us) restarts from page one once.
        if (/BAD_PAGE_TOKEN/.test(res.error.message) && !tokenRetried.current) {
          tokenRetried.current = true
          setNextPageToken(null)
          setRefetchKey((n) => n + 1)
          return
        }
        setError(classify(res.error))
        return
      }
      setItems((prev) => {
        const seen = new Set(prev.map((i) => keyOfItem(view, i)))
        return [...prev, ...res.page.items.filter((i) => !seen.has(keyOfItem(view, i)))]
      })
      setNextPageToken(res.page.nextPageToken)
    })
  }, [fetchPage, isLoadingMore, nextPageToken, view])

  useEffect(() => {
    if (!active) {return}
    const tick = (): void => {
      if (document.visibilityState === 'visible') {refetch()}
    }
    const timer = setInterval(tick, BACKLOG_POLL_MS)
    return () => clearInterval(timer)
  }, [active, refetch])

  const debounce = useRef<ReturnType<typeof setTimeout> | null>(null)
  useEffect(() => () => { if (debounce.current) {clearTimeout(debounce.current)} }, [])
  useRequestSubscription({
    onEvent: (event) => {
      if (!active || !REFRESH_EVENTS.has(eventSuffix(event.eventType))) {return}
      if (debounce.current) {clearTimeout(debounce.current)}
      debounce.current = setTimeout(refetch, BACKLOG_EVENT_DEBOUNCE_MS)
    }
  })

  const countLabel = useCallback(
    () => ({ count: items.length, plus: nextPageToken !== null }),
    [items.length, nextPageToken]
  )

  return useMemo(
    () => ({
      items, nextPageToken, isLoading, isLoadingMore, error, loadedOnce,
      hasMore: nextPageToken !== null, supported: supportState !== 'unsupported',
      loadMore, refetch, countLabel
    }),
    [items, nextPageToken, isLoading, isLoadingMore, error, loadedOnce, supportState, loadMore, refetch, countLabel]
  )
}

export function useRequestBacklog(filters: BacklogFilters = {}, options: { active?: boolean } = {}) {
  return useBacklogPages<RequestBacklogRowData>('requests', filters, options.active ?? true)
}
export function useTaskBacklog(filters: BacklogFilters = {}, options: { active?: boolean } = {}) {
  return useBacklogPages<BacklogGroupData>('tasks', filters, options.active ?? true)
}
export function useExecuteBacklog(filters: BacklogFilters = {}, options: { active?: boolean } = {}) {
  return useBacklogPages<BacklogGroupData>('execute', filters, options.active ?? true)
}

export function useBacklog(view: 'requests', filters?: BacklogFilters, options?: { active?: boolean }): BacklogState<RequestBacklogRowData>
export function useBacklog(view: 'tasks' | 'execute', filters?: BacklogFilters, options?: { active?: boolean }): BacklogState<BacklogGroupData>
export function useBacklog(
  view: BacklogView,
  filters: BacklogFilters = {},
  options: { active?: boolean } = {}
): BacklogState<RequestBacklogRowData> | BacklogState<BacklogGroupData> {
  return useBacklogPages<RequestBacklogRowData | BacklogGroupData>(view, filters, options.active ?? true) as never
}
