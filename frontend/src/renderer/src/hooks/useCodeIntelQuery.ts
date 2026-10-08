/**
 * useCodeIntelQuery.ts — FE-CV-TASK-050-13
 *
 * The one data-fetching hook for code-intel views (lenses never call RPC themselves).
 * - Cache-first reads (per-worktree LRU from the code-intel slice)
 * - Envelope channels (`Env<...>` in the contract) return `data` + `meta` + `truncated`
 * - `CODEINTEL_TIMEOUT {inProgress}` auto-retries up to 90 s; abort on unmount / param change
 * - A plain `changed` push only sets `staleSignal`; `applyNow()` reloads. `resync` reloads at once
 * - No-op unless support is 'enabled' and the worktree selector is addressable
 *
 * @module hooks/useCodeIntelQuery
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import { useAppStore } from '@/store'
import { useCodeIntelSelector } from '@/lib/code-intel-worktree-selector'
import {
  CODE_INTEL_ENVELOPE_METHODS,
  toCodeIntelMethod
} from '../../../shared/code-intel-rpc-methods'
import type { CodeIntelEnvelope } from '../../../shared/code-intel-types'
import type { CodeIntelErrorKind } from '../../../shared/code-intel-parsers'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type CodeIntelQueryStatus = 'idle' | 'loading' | 'success' | 'error'

export type CodeIntelQueryError = {
  kind: CodeIntelErrorKind
  code: string | null
  message: string
  retryable: boolean
  /** Parsed error `data` (e.g. retryAfterMs, inProgress for CODEINTEL_TIMEOUT) */
  data?: Record<string, unknown> | null
}

export type CodeIntelQueryMeta = Omit<CodeIntelEnvelope<unknown>, 'data'>

export type CodeIntelQueryResult<T> = {
  data: T | null
  /** Envelope metadata (etag, sources, nextPageToken...) for envelope channels, else null */
  meta: CodeIntelQueryMeta | null
  status: CodeIntelQueryStatus
  error: CodeIntelQueryError | null
  stale: boolean
  truncated: boolean
  staleSignal: boolean
  refetch: () => void
  applyNow: () => void
}

export type CodeIntelQueryOpts = {
  /** Disable the query (skip all calls) */
  enabled?: boolean
  /** Scope key included in the cache key so different scopes never share entries */
  scopeKey?: string
  /** Channel; `quality.trace` and `codeIntel.quality.trace` are equivalent */
  method: string
  params: Record<string, unknown>
  /** Validate/shape the result; throwing yields error.kind 'tool-failed' */
  parseResult?: (raw: unknown) => unknown
}

export type CodeIntelCallOutcome =
  | { ok: true; result: unknown; meta?: CodeIntelQueryMeta | null }
  | { ok: false; error: CodeIntelQueryError }

export type CodeIntelCallFn = (
  worktreeId: string,
  method: string,
  params: Record<string, unknown>,
  signal: AbortSignal,
  environmentId: string | null
) => Promise<CodeIntelCallOutcome>

type CachedEntry = { data: unknown; meta: CodeIntelQueryMeta | null }

// ---------------------------------------------------------------------------
// Constants and helpers
// ---------------------------------------------------------------------------

export const MAX_RETRY_DURATION_MS = 90_000
export const DEFAULT_RETRY_DELAY_MS = 3_000

function stableStringify(value: unknown): string {
  if (Array.isArray(value)) {return `[${value.map(stableStringify).join(',')}]`}
  if (typeof value === 'object' && value !== null) {
    const entries = Object.entries(value as Record<string, unknown>)
      .filter(([, v]) => v !== undefined)
      .sort(([a], [b]) => (a < b ? -1 : 1))
    return `{${entries.map(([k, v]) => `${JSON.stringify(k)}:${stableStringify(v)}`).join(',')}}`
  }
  return JSON.stringify(value) ?? 'null'
}

export function buildCodeIntelCacheKey(
  method: string,
  params: Record<string, unknown>,
  scopeKey?: string
): string {
  return `${toCodeIntelMethod(method)}|${scopeKey ?? ''}|${stableStringify(params)}`
}

export async function defaultCodeIntelCall(
  worktreeId: string,
  method: string,
  params: Record<string, unknown>,
  signal: AbortSignal,
  environmentId: string | null
): Promise<CodeIntelCallOutcome> {
  // Lazy import keeps the client (and the store it reads) out of this module's load graph.
  const { getCodeIntelClient } = await import('../runtime/code-intel-client')
  const client = getCodeIntelClient()
  if (CODE_INTEL_ENVELOPE_METHODS.has(method)) {
    const res = await client.callEnvelope(worktreeId, method, params, (d) => d, { environmentId, signal })
    if (!res.ok) {return res}
    const { data, ...meta } = res.envelope
    return { ok: true, result: data, meta }
  }
  return client.call(worktreeId, method, params, { environmentId, signal })
}

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

export function useCodeIntelQuery<T = unknown>(
  worktreeId: string | null,
  environmentId: string | null,
  opts: CodeIntelQueryOpts,
  callFn: CodeIntelCallFn = defaultCodeIntelCall
): CodeIntelQueryResult<T> {
  const { enabled = true, scopeKey, params, parseResult } = opts
  const method = toCodeIntelMethod(opts.method)

  const supportState = useAppStore((s) => s.codeIntelSupportState.state)
  const resyncCounter = useAppStore((s) => s.codeIntelResyncCounter)
  const worktreeStale = useAppStore((s) =>
    worktreeId ? (s.codeIntelWorktreeState[worktreeId]?.stale ?? false) : false
  )
  const selector = useCodeIntelSelector(worktreeId ?? '')

  const [data, setData] = useState<T | null>(null)
  const [meta, setMeta] = useState<CodeIntelQueryMeta | null>(null)
  const [status, setStatus] = useState<CodeIntelQueryStatus>('idle')
  const [error, setError] = useState<CodeIntelQueryError | null>(null)

  const abortRef = useRef<AbortController | null>(null)
  const retryTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  // Latest values for the effect without re-running it on every render.
  const paramsRef = useRef(params)
  paramsRef.current = params
  const parseRef = useRef(parseResult)
  parseRef.current = parseResult
  const callRef = useRef(callFn)
  callRef.current = callFn

  const isQueryEnabled =
    enabled && Boolean(worktreeId) && supportState === 'enabled' && selector.state === 'ready'
  const cacheKey = buildCodeIntelCacheKey(method, params, scopeKey)

  const execute = useCallback(
    async (signal: AbortSignal, useCache: boolean): Promise<void> => {
      if (!worktreeId) {return}
      const store = useAppStore.getState()

      if (useCache) {
        const cached = store.getCacheResult(worktreeId, cacheKey) as CachedEntry | undefined
        if (cached !== undefined) {
          setData(cached.data as T)
          setMeta(cached.meta)
          setStatus('success')
          setError(null)
          return
        }
      }

      setStatus('loading')
      const startedAt = Date.now()

      const attempt = async (): Promise<void> => {
        if (signal.aborted) {return}
        let outcome: CodeIntelCallOutcome
        try {
          outcome = await callRef.current(worktreeId, method, paramsRef.current, signal, environmentId)
        } catch {
          if (signal.aborted) {return}
          setStatus('error')
          setError({ kind: 'unknown', code: null, message: 'Unexpected error', retryable: false })
          return
        }
        if (signal.aborted) {return}

        if (!outcome.ok) {
          const err = outcome.error
          const canRetry =
            err.kind === 'timeout' &&
            err.data?.inProgress === true &&
            Date.now() - startedAt < MAX_RETRY_DURATION_MS
          if (canRetry) {
            const delay =
              typeof err.data?.retryAfterMs === 'number' ? err.data.retryAfterMs : DEFAULT_RETRY_DELAY_MS
            retryTimerRef.current = setTimeout(() => void attempt(), delay)
            return
          }
          setStatus('error')
          setError(err)
          return
        }

        let parsed: unknown = outcome.result
        if (parseRef.current) {
          try {
            parsed = parseRef.current(outcome.result)
          } catch {
            setStatus('error')
            setError({ kind: 'tool-failed', code: null, message: 'Result shape mismatch', retryable: false })
            return
          }
        }

        const entry: CachedEntry = { data: parsed, meta: outcome.meta ?? null }
        useAppStore.getState().setCacheResult(worktreeId, cacheKey, entry)
        useAppStore.getState().markCodeIntelWorktreeFresh(worktreeId)
        setData(parsed as T)
        setMeta(entry.meta)
        setError(null)
        setStatus('success')
      }

      await attempt()
    },
    [worktreeId, environmentId, method, cacheKey]
  )

  useEffect(() => {
    if (!isQueryEnabled) {
      setStatus('idle')
      return
    }
    const ctrl = new AbortController()
    abortRef.current = ctrl
    void execute(ctrl.signal, true)
    return () => {
      ctrl.abort()
      if (retryTimerRef.current) {clearTimeout(retryTimerRef.current)}
    }
    // resyncCounter forces a reload (the push stream bumps it on resync / reconnect).
  }, [isQueryEnabled, resyncCounter, execute])

  const refetch = useCallback(() => {
    if (!isQueryEnabled) {return}
    abortRef.current?.abort()
    if (retryTimerRef.current) {clearTimeout(retryTimerRef.current)}
    const ctrl = new AbortController()
    abortRef.current = ctrl
    void execute(ctrl.signal, false)
  }, [isQueryEnabled, execute])

  const staleSignal = worktreeStale && status === 'success'

  return {
    data,
    meta,
    status,
    error,
    stale: staleSignal || meta?.stale === true,
    truncated: meta?.truncated === true,
    staleSignal,
    refetch,
    applyNow: refetch
  }
}
