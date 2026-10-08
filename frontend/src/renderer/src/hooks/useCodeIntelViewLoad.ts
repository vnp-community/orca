/**
 * useCodeIntelViewLoad.ts — FE-CV-TASK-055-02 / 056-05
 *
 * Minimal envelope-channel loader for review lenses that need abort-on-change and a small
 * in-memory cache (architecture, dataFlow). It talks to the code-intel client directly so it
 * does not depend on the code-intel slice cache.
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import { getCodeIntelClient } from '../runtime/code-intel-client'
import type { CodeIntelRpcError } from '../runtime/code-intel-client'

export type ViewLoadStatus = 'idle' | 'loading' | 'ready' | 'error'

export type ViewLoadMeta = {
  truncated: boolean
  totalCount: number
  nextPageToken?: string
  stale: boolean
}

export type ViewLoadResult<T> = {
  status: ViewLoadStatus
  data: T | null
  meta: ViewLoadMeta | null
  error: CodeIntelRpcError | null
  /** Re-fetches, bypassing the cache. */
  refetch: () => void
}

type CacheEntry = { data: unknown; meta: ViewLoadMeta }
const CACHE_LIMIT = 24
const cache = new Map<string, CacheEntry>()

function cacheKeyFor(worktreeId: string, method: string, params: Record<string, unknown>): string {
  return `${worktreeId}\u0000${method}\u0000${JSON.stringify(params)}`
}

/** Drops cached responses of a method (e.g. architecture after c4.save). */
export function invalidateCodeIntelViewCache(worktreeId: string, method: string): void {
  const prefix = `${worktreeId}\u0000${method}\u0000`
  for (const key of cache.keys()) {
    if (key.startsWith(prefix)) {
      cache.delete(key)
    }
  }
}

export function resetCodeIntelViewCache(): void {
  cache.clear()
}

export function useCodeIntelViewLoad<T>(args: {
  worktreeId: string
  environmentId: string | null
  method: string
  /** null disables the load (idle). */
  params: Record<string, unknown> | null
  parse: (raw: unknown) => T
  useCache?: boolean
}): ViewLoadResult<T> {
  const { worktreeId, environmentId, method, params, parse, useCache = true } = args
  const [state, setState] = useState<{
    status: ViewLoadStatus
    data: T | null
    meta: ViewLoadMeta | null
    error: CodeIntelRpcError | null
  }>({ status: 'idle', data: null, meta: null, error: null })
  const [tick, setTick] = useState(0)
  const bypass = useRef(false)
  const parseRef = useRef(parse)
  parseRef.current = parse
  const paramsKey = params === null ? null : JSON.stringify(params)

  useEffect(() => {
    if (paramsKey === null) {
      setState({ status: 'idle', data: null, meta: null, error: null })
      return
    }
    const p = JSON.parse(paramsKey) as Record<string, unknown>
    const key = cacheKeyFor(worktreeId, method, p)
    const hit = !bypass.current && useCache ? cache.get(key) : undefined
    bypass.current = false
    if (hit) {
      setState({ status: 'ready', data: hit.data as T, meta: hit.meta, error: null })
      return
    }
    const ctrl = new AbortController()
    setState({ status: 'loading', data: null, meta: null, error: null })
    void (async () => {
      try {
        const res = await getCodeIntelClient().callEnvelope<T>(worktreeId, method, p, (raw) => parseRef.current(raw), {
          environmentId,
          signal: ctrl.signal
        })
        if (ctrl.signal.aborted) {
          return
        }
        if (!res.ok) {
          setState({ status: 'error', data: null, meta: null, error: res.error })
          return
        }
        const e = res.envelope
        const meta: ViewLoadMeta = {
          truncated: e.truncated,
          totalCount: e.totalCount,
          ...(e.nextPageToken ? { nextPageToken: e.nextPageToken } : {}),
          stale: e.stale
        }
        if (e.data === undefined) {
          setState({ status: 'error', data: null, meta: null, error: unavailableError() })
          return
        }
        if (useCache) {
          cache.set(key, { data: e.data, meta })
          while (cache.size > CACHE_LIMIT) {
            cache.delete(cache.keys().next().value as string)
          }
        }
        setState({ status: 'ready', data: e.data, meta, error: null })
      } catch (err) {
        if (!ctrl.signal.aborted) {
          setState({
            status: 'error',
            data: null,
            meta: null,
            error: { kind: 'unknown', code: null, message: err instanceof Error ? err.message : 'failed', data: null, retryable: true }
          })
        }
      }
    })()
    return () => ctrl.abort()
  }, [worktreeId, environmentId, method, paramsKey, tick, useCache])

  const refetch = useCallback(() => {
    bypass.current = true
    setTick((n) => n + 1)
  }, [])

  return { ...state, refetch }
}

function unavailableError(): CodeIntelRpcError {
  return { kind: 'unknown', code: null, message: 'empty response', data: null, retryable: true }
}
