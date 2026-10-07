/**
 * useCodeIntelQuery.ts — FE-CV-TASK-050-13
 *
 * Generic data-fetching hook for code-intel RPC calls.
 * Features:
 * - Cache-first reads (LRU via 050-10 slice)
 * - Auto-retry for TIMEOUT+inProgress up to 90s
 * - AbortController cleanup on unmount or param change
 * - Stale signal / applyNow pattern
 * - No-op when support !== 'enabled' or selector 'unsupported'
 *
 * @module hooks/useCodeIntelQuery
 */

import { useEffect, useRef, useState, useCallback } from 'react'
import { useAppStore } from '@/store'
import type { CodeIntelSupportState } from '../store/slices/code-intel'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type CodeIntelQueryStatus = 'idle' | 'loading' | 'success' | 'error'

export type CodeIntelQueryError = {
  kind: string
  code: string | null
  message: string
  retryable: boolean
}

export type CodeIntelQueryResult<T> = {
  data: T | null
  status: CodeIntelQueryStatus
  error: CodeIntelQueryError | null
  stale: boolean
  staleSignal: boolean
  refetch: () => void
  applyNow: () => void
}

export type CodeIntelQueryOpts = {
  /** Disable the query (skip all calls) */
  enabled?: boolean
  /** Scope key — included in cache key to avoid cross-scope collisions */
  scopeKey?: string
  /** Method to call */
  method: string
  /** Params to pass */
  params: Record<string, unknown>
  /** Parse / validate result shape */
  parseResult?: (raw: unknown) => unknown
}

type CallFn = (
  worktreeId: string,
  method: string,
  params: Record<string, unknown>,
  signal: AbortSignal,
  environmentId: string | null
) => Promise<{ ok: true; result: unknown } | { ok: false; error: CodeIntelQueryError }>

// ---------------------------------------------------------------------------
// Retry constants
// ---------------------------------------------------------------------------

const MAX_RETRY_DURATION_MS = 90_000
const DEFAULT_RETRY_DELAY_MS = 3_000

// ---------------------------------------------------------------------------
// Cache key builder
// ---------------------------------------------------------------------------

function buildCacheKey(method: string, params: Record<string, unknown>, scopeKey?: string): string {
  const paramsKey = JSON.stringify(params, Object.keys(params).sort())
  return scopeKey ? `${method}|${scopeKey}|${paramsKey}` : `${method}|${paramsKey}`
}

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

export function useCodeIntelQuery<T = unknown>(
  worktreeId: string | null,
  environmentId: string | null,
  opts: CodeIntelQueryOpts,
  callFn?: CallFn
): CodeIntelQueryResult<T> {
  const { enabled = true, scopeKey, method, params, parseResult } = opts

  const supportState = useAppStore((s) => {
    const state = s as Record<string, unknown>
    return (state.codeIntelSupportState as CodeIntelSupportState | undefined)?.state ?? 'unknown'
  })

  const getCacheResult = useAppStore((s) => (s as Record<string, unknown>).getCacheResult as ((wt: string, key: string) => unknown) | undefined)
  const setCacheResult = useAppStore((s) => (s as Record<string, unknown>).setCacheResult as ((wt: string, key: string, val: unknown) => void) | undefined)
  const resyncCounter = useAppStore((s) => {
    const state = s as Record<string, unknown>
    return (state.codeIntelResyncCounter as number | undefined) ?? 0
  })

  const [data, setData] = useState<T | null>(null)
  const [status, setStatus] = useState<CodeIntelQueryStatus>('idle')
  const [error, setError] = useState<CodeIntelQueryError | null>(null)
  const [staleSignal, setStaleSignal] = useState(false)

  const abortRef = useRef<AbortController | null>(null)
  const retryStartRef = useRef<number>(0)
  const retryTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const isQueryEnabled = enabled && Boolean(worktreeId) && supportState === 'enabled'
  const cacheKey = buildCacheKey(method, params, scopeKey)

  const executeQuery = useCallback(async (signal: AbortSignal) => {
    if (!worktreeId || !isQueryEnabled) return

    // Check cache first
    const cached = getCacheResult?.(worktreeId, cacheKey)
    if (cached !== undefined) {
      setData(cached as T)
      setStatus('success')
      setStaleSignal(false)
      return
    }

    setStatus('loading')
    retryStartRef.current = Date.now()

    const doCall = async (): Promise<void> => {
      if (signal.aborted) return

      let callResult: { ok: true; result: unknown } | { ok: false; error: CodeIntelQueryError }

      try {
        if (callFn) {
          callResult = await callFn(worktreeId, method, params, signal, environmentId)
        } else {
          const { getCodeIntelClient } = await import('../runtime/code-intel-client')
          const client = getCodeIntelClient()
          callResult = await client.call(worktreeId, method, params, { environmentId, signal })
        }
      } catch {
        if (signal.aborted) return
        setStatus('error')
        setError({ kind: 'unknown', code: null, message: 'Unexpected error', retryable: false })
        return
      }

      if (signal.aborted) return

      if (!callResult.ok) {
        const err = callResult.error

        // Retry for timeout+inProgress within 90s
        const isTimeoutRetryable =
          (err.kind === 'unknown' || err.kind === 'rate_limited') &&
          err.message.includes('inProgress') &&
          Date.now() - retryStartRef.current < MAX_RETRY_DURATION_MS

        if (isTimeoutRetryable) {
          const retryAfterMs = extractRetryAfterMs(err.message) ?? DEFAULT_RETRY_DELAY_MS
          retryTimerRef.current = setTimeout(() => void doCall(), retryAfterMs)
          return
        }

        setStatus('error')
        setError(err)
        return
      }

      const raw = callResult.result
      let parsed: unknown = raw
      if (parseResult) {
        try {
          parsed = parseResult(raw)
        } catch {
          setStatus('error')
          setError({ kind: 'tool_failed', code: null, message: 'Result shape mismatch', retryable: false })
          return
        }
      }

      setCacheResult?.(worktreeId, cacheKey, parsed)
      setData(parsed as T)
      setStatus('success')
      setError(null)
      setStaleSignal(false)
    }

    await doCall()
  }, [worktreeId, environmentId, method, cacheKey, isQueryEnabled, callFn, getCacheResult, setCacheResult])

  useEffect(() => {
    if (!isQueryEnabled) {
      setStatus('idle')
      return
    }

    abortRef.current?.abort()
    if (retryTimerRef.current) clearTimeout(retryTimerRef.current)

    const ctrl = new AbortController()
    abortRef.current = ctrl

    void executeQuery(ctrl.signal)

    return () => {
      ctrl.abort()
      if (retryTimerRef.current) clearTimeout(retryTimerRef.current)
    }
  }, [isQueryEnabled, resyncCounter, cacheKey, executeQuery])

  const refetch = useCallback(() => {
    if (!worktreeId || !isQueryEnabled) return
    abortRef.current?.abort()
    const ctrl = new AbortController()
    abortRef.current = ctrl
    void executeQuery(ctrl.signal)
  }, [worktreeId, isQueryEnabled, executeQuery])

  const applyNow = useCallback(() => {
    setStaleSignal(false)
    refetch()
  }, [refetch])

  return {
    data,
    status,
    error,
    stale: staleSignal,
    staleSignal,
    refetch,
    applyNow
  }
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

function extractRetryAfterMs(message: string): number | null {
  const match = message.match(/"retryAfterMs"\s*:\s*(\d+)/)
  return match ? parseInt(match[1], 10) : null
}
