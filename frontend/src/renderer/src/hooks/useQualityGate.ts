/**
 * useQualityGate.ts — FE-CV-TASK-087-03
 *
 * Reads the cached gate of a worktree and loads it when a surface needs it. No RPC unless
 * quality support is `enabled`; the push listener is retained only while a hook is mounted.
 *
 * @module hooks/useQualityGate
 */

import { useCallback, useEffect, useMemo } from 'react'
import { useAppStore } from '@/store'
import { retainQualityEventSync } from '../lib/code-intel-quality-event-sync'
import type { CodeIntelRpcError } from '../runtime/code-intel-client'
import type { QualityGateResponse } from '../../../shared/code-intel-quality-types'
import { useQualitySupport } from './useQualitySupport'

export type QualityGateStatus = 'idle' | 'loading' | 'ready' | 'error'

export type UseQualityGateResult = {
  support: ReturnType<typeof useQualitySupport>
  status: QualityGateStatus
  response: QualityGateResponse | null
  /** Cached response was invalidated by a later event and is being refreshed. */
  cacheStale: boolean
  error: CodeIntelRpcError | null
  refetch: () => void
}

export function useQualityGate(
  worktreeId: string | null | undefined,
  opts: { base?: string; profileName?: string } = {}
): UseQualityGateResult {
  const support = useQualitySupport(worktreeId)
  const enabled = support === 'enabled' && Boolean(worktreeId)
  const entry = useAppStore((s) =>
    worktreeId ? s.codeIntelQualityByWorktree[worktreeId]?.gate : null
  )
  const loading = useAppStore((s) =>
    worktreeId ? (s.codeIntelQualityByWorktree[worktreeId]?.loading.gate ?? false) : false
  )
  const error = useAppStore((s) =>
    worktreeId ? (s.codeIntelQualityByWorktree[worktreeId]?.errors.gate ?? null) : null
  )
  const resync = useAppStore((s) => s.codeIntelResyncCounter)
  const { base, profileName } = opts

  useEffect(() => {
    if (!enabled) {
      return
    }
    return retainQualityEventSync()
  }, [enabled])

  useEffect(() => {
    if (!enabled || !worktreeId) {
      return
    }
    // Why: a changed base/profile or a stream resync must refetch; otherwise a cached gate is reused.
    void useAppStore.getState().loadQualityGate(worktreeId, { base, profileName })
  }, [enabled, worktreeId, base, profileName, resync])

  const refetch = useCallback(() => {
    if (enabled && worktreeId) {
      void useAppStore.getState().loadQualityGate(worktreeId, { base, profileName, force: true })
    }
  }, [enabled, worktreeId, base, profileName])

  return useMemo(() => {
    let status: QualityGateStatus = 'idle'
    if (enabled) {
      if (entry) {
        status = 'ready'
      } else if (error) {
        status = 'error'
      } else {
        status = loading ? 'loading' : 'idle'
      }
    }
    return {
      support,
      status,
      response: enabled ? (entry?.data ?? null) : null,
      cacheStale: enabled ? (entry?.stale ?? false) : false,
      error: enabled ? error : null,
      refetch
    }
  }, [support, enabled, entry, loading, error, refetch])
}
