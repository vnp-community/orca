/**
 * useCodeIntelIndexStatus.ts — FE-CV-TASK-050-14
 *
 * Index status for a worktree. `codeIntel.status` returns a flat IndexStatus (no envelope).
 * Loads on mount, `refresh({force})` re-asks with `refresh: true`, and polls every 30 s only while
 * the push stream is unavailable (codeIntelEventsState === 'polling').
 *
 * @module hooks/useCodeIntelIndexStatus
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import { useAppStore } from '@/store'
import { useCodeIntelSelector } from '@/lib/code-intel-worktree-selector'
import { parseIndexStatus } from '../../../shared/code-intel-parsers'
import { CODE_INTEL_RPC_METHODS } from '../../../shared/code-intel-rpc-methods'
import type { IndexOverall, IndexStatus } from '../../../shared/code-intel-types'
import { defaultCodeIntelCall } from './useCodeIntelQuery'
import type { CodeIntelCallFn, CodeIntelQueryError } from './useCodeIntelQuery'

export const INDEX_STATUS_POLL_MS = 30_000

const STALE_OVERALL: ReadonlySet<IndexOverall> = new Set<IndexOverall>(['STALE', 'OVERLAY'])

/** Stale when the overall state says so or any tool reports stale freshness. */
export function isIndexStale(status: IndexStatus | null): boolean {
  if (!status) {return false}
  return STALE_OVERALL.has(status.overall) || status.tools.some((t) => t.freshness === 'stale')
}

export type UseCodeIntelIndexStatusResult = {
  status: IndexStatus | null
  overall: IndexOverall | null
  tools: IndexStatus['tools']
  indexBasis: IndexStatus['indexBasis']
  activeJob: IndexStatus['activeJob'] | null
  scopeMismatch: boolean
  stale: boolean
  isLoading: boolean
  error: CodeIntelQueryError | null
  refresh: (opts?: { force?: boolean }) => Promise<void>
}

export function useCodeIntelIndexStatus(
  worktreeId: string | null,
  environmentId: string | null,
  callFn: CodeIntelCallFn = defaultCodeIntelCall
): UseCodeIntelIndexStatusResult {
  const supportState = useAppStore((s) => s.codeIntelSupportState.state)
  const eventsState = useAppStore((s) => s.codeIntelEventsState)
  const resyncCounter = useAppStore((s) => s.codeIntelResyncCounter)
  const selector = useCodeIntelSelector(worktreeId ?? '')
  const enabled = Boolean(worktreeId) && supportState === 'enabled' && selector.state === 'ready'

  const [status, setStatus] = useState<IndexStatus | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<CodeIntelQueryError | null>(null)
  const callRef = useRef(callFn)
  callRef.current = callFn
  const generationRef = useRef(0)

  const load = useCallback(
    async (force: boolean, signal?: AbortSignal): Promise<void> => {
      if (!worktreeId) {return}
      const generation = ++generationRef.current
      setIsLoading(true)
      const outcome = await callRef
        .current(
          worktreeId,
          CODE_INTEL_RPC_METHODS.STATUS,
          force ? { refresh: true } : {},
          signal ?? new AbortController().signal,
          environmentId
        )
        .catch(
          (): { ok: false; error: CodeIntelQueryError } => ({
            ok: false,
            error: { kind: 'unknown', code: null, message: 'Unexpected error', retryable: false }
          })
        )
      // Drop results of superseded or aborted loads.
      if (generation !== generationRef.current || signal?.aborted) {return}
      setIsLoading(false)
      if (outcome.ok) {
        setStatus(parseIndexStatus(outcome.result))
        setError(null)
      } else {
        setError(outcome.error)
      }
    },
    [worktreeId, environmentId]
  )

  useEffect(() => {
    if (!enabled) {return}
    const ctrl = new AbortController()
    void load(false, ctrl.signal)
    const generation = generationRef
    return () => {
      ctrl.abort()
      generation.current++
    }
  }, [enabled, load, resyncCounter])

  useEffect(() => {
    if (!enabled || eventsState !== 'polling') {return}
    const timer = setInterval(() => void load(false), INDEX_STATUS_POLL_MS)
    return () => clearInterval(timer)
  }, [enabled, eventsState, load])

  const refresh = useCallback(
    async (opts?: { force?: boolean }) => {
      if (!enabled) {return}
      await load(opts?.force === true)
    },
    [enabled, load]
  )

  return {
    status,
    overall: status?.overall ?? null,
    tools: status?.tools ?? [],
    indexBasis: status?.indexBasis ?? [],
    activeJob: status?.activeJob ?? null,
    scopeMismatch: status?.scopeMismatch ?? false,
    stale: isIndexStale(status),
    isLoading,
    error,
    refresh
  }
}
