/**
 * useQualityTrend.ts — FE-CV-TASK-087-15
 *
 * Trend points of a worktree for one grouping (`turn` or `commit`), read from the quality slice.
 * No RPC unless quality support is `enabled`; the lens mounts the block only while it is open.
 *
 * @module hooks/useQualityTrend
 */

import { useCallback, useEffect, useMemo } from 'react'
import { useAppStore } from '@/store'
import { resolveQualityBlockStatus } from '../components/review-map/quality/quality-block-status-model'
import type { QualityBlockStatus } from '../components/review-map/quality/quality-block-status-model'
import type { CodeIntelRpcError } from '../runtime/code-intel-client'
import type { QualityTrendPoint } from '../../../shared/code-intel-quality-visualization-types'
import type { QualityTrendGroup } from '../store/slices/code-intel-quality-state-types'
import { useQualitySupport } from './useQualitySupport'
import type { QualitySupport } from './useQualitySupport'

const NO_POINTS: readonly QualityTrendPoint[] = []

export type UseQualityTrendResult = {
  support: QualitySupport
  status: QualityBlockStatus
  points: readonly QualityTrendPoint[]
  truncated: boolean
  totalCount: number
  error: CodeIntelRpcError | null
  refetch: () => void
}

export function useQualityTrend(
  worktreeId: string | null | undefined,
  groupBy: QualityTrendGroup
): UseQualityTrendResult {
  const support = useQualitySupport(worktreeId)
  const enabled = support === 'enabled' && Boolean(worktreeId)
  const entry = useAppStore((s) =>
    worktreeId ? s.codeIntelQualityByWorktree[worktreeId]?.trend[groupBy] : undefined
  )
  const loading = useAppStore((s) =>
    worktreeId ? (s.codeIntelQualityByWorktree[worktreeId]?.loading.trend ?? false) : false
  )
  const error = useAppStore((s) =>
    worktreeId ? (s.codeIntelQualityByWorktree[worktreeId]?.errors.trend ?? null) : null
  )
  const epoch = useAppStore((s) =>
    worktreeId ? (s.codeIntelQualityByWorktree[worktreeId]?.epoch ?? 0) : 0
  )
  const resync = useAppStore((s) => s.codeIntelResyncCounter)

  useEffect(() => {
    if (enabled && worktreeId) {
      // Why: a finished run (epoch) or a stream resync changes the series; refetch rather than trust the cache.
      void useAppStore.getState().loadQualityTrend(worktreeId, groupBy)
    }
  }, [enabled, worktreeId, groupBy, epoch, resync])

  const refetch = useCallback(() => {
    if (enabled && worktreeId) {
      void useAppStore.getState().loadQualityTrend(worktreeId, groupBy, { force: true })
    }
  }, [enabled, worktreeId, groupBy])

  return useMemo(() => {
    const data = enabled ? entry?.data : undefined
    return {
      support,
      status: resolveQualityBlockStatus({
        enabled,
        hasData: Boolean(data),
        isEmpty: Boolean(data) && data!.points.length === 0,
        loading,
        failed: error !== null,
        stale: entry?.stale ?? false
      }),
      points: data?.points ?? NO_POINTS,
      truncated: data?.truncated ?? false,
      totalCount: data?.totalCount ?? 0,
      error: enabled ? error : null,
      refetch
    }
  }, [support, enabled, entry, loading, error, refetch])
}
