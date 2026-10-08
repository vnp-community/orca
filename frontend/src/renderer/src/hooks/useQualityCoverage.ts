/**
 * useQualityCoverage.ts — FE-CV-TASK-087-15
 *
 * Coverage report of a worktree from the quality slice. `report: null` is an empty state with
 * the backend's reason, never 0%. No RPC unless quality support is `enabled`.
 *
 * @module hooks/useQualityCoverage
 */

import { useCallback, useEffect, useMemo } from 'react'
import { useAppStore } from '@/store'
import { resolveQualityBlockStatus } from '../components/review-map/quality/quality-block-status-model'
import type { QualityBlockStatus } from '../components/review-map/quality/quality-block-status-model'
import type { CodeIntelRpcError } from '../runtime/code-intel-client'
import type { CoverageReport } from '../../../shared/code-intel-quality-visualization-types'
import { useQualitySupport } from './useQualitySupport'
import type { QualitySupport } from './useQualitySupport'

export type UseQualityCoverageResult = {
  support: QualitySupport
  status: QualityBlockStatus
  report: CoverageReport | null
  /** Why there is no report (backend `reason`), when it gave one. */
  reason: string | null
  error: CodeIntelRpcError | null
  refetch: () => void
}

export function useQualityCoverage(
  worktreeId: string | null | undefined,
  runId?: string
): UseQualityCoverageResult {
  const support = useQualitySupport(worktreeId)
  const enabled = support === 'enabled' && Boolean(worktreeId)
  const entry = useAppStore((s) =>
    worktreeId ? s.codeIntelQualityByWorktree[worktreeId]?.coverage : null
  )
  const loading = useAppStore((s) =>
    worktreeId ? (s.codeIntelQualityByWorktree[worktreeId]?.loading.coverage ?? false) : false
  )
  const error = useAppStore((s) =>
    worktreeId ? (s.codeIntelQualityByWorktree[worktreeId]?.errors.coverage ?? null) : null
  )
  const epoch = useAppStore((s) =>
    worktreeId ? (s.codeIntelQualityByWorktree[worktreeId]?.epoch ?? 0) : 0
  )
  const resync = useAppStore((s) => s.codeIntelResyncCounter)

  useEffect(() => {
    if (enabled && worktreeId) {
      void useAppStore.getState().loadQualityCoverage(worktreeId, { runId })
    }
  }, [enabled, worktreeId, runId, epoch, resync])

  const refetch = useCallback(() => {
    if (enabled && worktreeId) {
      void useAppStore.getState().loadQualityCoverage(worktreeId, { runId, force: true })
    }
  }, [enabled, worktreeId, runId])

  return useMemo(() => {
    const data = enabled ? entry?.data : undefined
    return {
      support,
      status: resolveQualityBlockStatus({
        enabled,
        hasData: Boolean(data),
        isEmpty: Boolean(data) && data!.report === null,
        loading,
        failed: error !== null,
        stale: entry?.stale ?? false
      }),
      report: data?.report ?? null,
      reason: data?.reason ?? null,
      error: enabled ? error : null,
      refetch
    }
  }, [support, enabled, entry, loading, error, refetch])
}
