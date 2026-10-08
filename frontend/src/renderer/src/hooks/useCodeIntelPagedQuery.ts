/**
 * useCodeIntelPagedQuery.ts — FE-CV-TASK-050-13
 *
 * Paged view of an envelope channel (`limit` + opaque `pageToken`, `nextPageToken` in the envelope).
 * The first page goes through useCodeIntelQuery (cache, retry, stale handling); later pages are
 * appended by fetchNextPage, which ignores calls while a page is in flight.
 *
 * @module hooks/useCodeIntelPagedQuery
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import { defaultCodeIntelCall, buildCodeIntelCacheKey, useCodeIntelQuery } from './useCodeIntelQuery'
import type {
  CodeIntelCallFn,
  CodeIntelQueryError,
  CodeIntelQueryOpts,
  CodeIntelQueryResult
} from './useCodeIntelQuery'
import { toCodeIntelMethod } from '../../../shared/code-intel-rpc-methods'

export type CodeIntelPagedQueryOpts<TPage, TItem> = CodeIntelQueryOpts & {
  /** Pick the items out of one page's data */
  select: (page: TPage) => TItem[]
}

export type CodeIntelPagedQueryResult<TItem> = Omit<CodeIntelQueryResult<unknown>, 'data'> & {
  items: TItem[]
  hasNextPage: boolean
  isFetchingNextPage: boolean
  nextPageError: CodeIntelQueryError | null
  fetchNextPage: () => void
}

export function useCodeIntelPagedQuery<TPage = unknown, TItem = unknown>(
  worktreeId: string | null,
  environmentId: string | null,
  opts: CodeIntelPagedQueryOpts<TPage, TItem>,
  callFn: CodeIntelCallFn = defaultCodeIntelCall
): CodeIntelPagedQueryResult<TItem> {
  const { select, ...queryOpts } = opts
  const first = useCodeIntelQuery<TPage>(worktreeId, environmentId, queryOpts, callFn)
  const queryKey = buildCodeIntelCacheKey(queryOpts.method, queryOpts.params, queryOpts.scopeKey)

  const [extra, setExtra] = useState<{ key: string; items: TItem[]; token: string | null } | null>(null)
  const [isFetchingNextPage, setFetching] = useState(false)
  const [nextPageError, setNextPageError] = useState<CodeIntelQueryError | null>(null)
  const inFlightRef = useRef(false)
  const generationRef = useRef(0)
  const selectRef = useRef(select)
  selectRef.current = select

  // Params change or a reload of page 1 invalidates appended pages and any page in flight.
  const firstData = first.data
  useEffect(() => {
    generationRef.current++
    inFlightRef.current = false
    setExtra(null)
    setFetching(false)
    setNextPageError(null)
  }, [queryKey, firstData])

  const firstToken = first.meta?.nextPageToken ?? null
  const appended = extra && extra.key === queryKey ? extra : null
  const token = appended ? appended.token : firstToken

  const fetchNextPage = useCallback(() => {
    if (!worktreeId || !token || inFlightRef.current || first.status !== 'success') {return}
    inFlightRef.current = true
    setFetching(true)
    setNextPageError(null)
    const generation = generationRef.current
    const ctrl = new AbortController()
    void callFn(
      worktreeId,
      toCodeIntelMethod(queryOpts.method),
      { ...queryOpts.params, pageToken: token },
      ctrl.signal,
      environmentId
    )
      .then((outcome) => {
        if (generation !== generationRef.current) {return}
        if (!outcome.ok) {
          setNextPageError(outcome.error)
          return
        }
        let pageItems: TItem[]
        try {
          pageItems = selectRef.current(outcome.result as TPage)
        } catch {
          setNextPageError({ kind: 'tool-failed', code: null, message: 'Result shape mismatch', retryable: false })
          return
        }
        const nextToken = outcome.meta?.nextPageToken ?? null
        setExtra((prev) => ({
          key: queryKey,
          items: [...(prev && prev.key === queryKey ? prev.items : []), ...pageItems],
          token: nextToken
        }))
      })
      .catch(() => {
        if (generation !== generationRef.current) {return}
        setNextPageError({ kind: 'unknown', code: null, message: 'Unexpected error', retryable: false })
      })
      .finally(() => {
        if (generation === generationRef.current) {
          inFlightRef.current = false
          setFetching(false)
        }
      })
  }, [worktreeId, token, first.status, callFn, queryOpts.method, queryOpts.params, environmentId, queryKey])

  const firstItems = firstData ? select(firstData) : []
  const { data: _data, ...rest } = first

  return {
    ...rest,
    items: appended ? [...firstItems, ...appended.items] : firstItems,
    hasNextPage: token !== null,
    isFetchingNextPage,
    nextPageError,
    fetchNextPage
  }
}
