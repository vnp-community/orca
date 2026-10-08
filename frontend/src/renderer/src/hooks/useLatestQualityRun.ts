/**
 * useLatestQualityRun.ts — FE-CV-TASK-087-10
 *
 * Latest finished local run of a worktree, read from the cached run list (loaded once when
 * `enabled`). Diff annotations key their eligibility on this run's HEAD and dirtiness.
 *
 * @module hooks/useLatestQualityRun
 */

import { useEffect, useMemo } from 'react'
import { useAppStore } from '@/store'
import type { QualityRun } from '../../../shared/code-intel-quality-types'

export function pickLatestFinishedLocalRun(
  runs: readonly QualityRun[] | undefined
): QualityRun | null {
  let best: QualityRun | null = null
  for (const run of runs ?? []) {
    if (run.source === 'ci' || (run.status !== 'succeeded' && run.status !== 'failed')) {
      continue
    }
    if (!best || (run.finishedAt ?? '') > (best.finishedAt ?? '')) {
      best = run
    }
  }
  return best
}

export function useLatestQualityRun(
  worktreeId: string | null | undefined,
  enabled: boolean
): QualityRun | null {
  const runs = useAppStore((s) =>
    worktreeId ? s.codeIntelQualityByWorktree[worktreeId]?.runs?.data : undefined
  )
  const loaded = runs !== undefined
  useEffect(() => {
    if (enabled && worktreeId && !loaded) {
      void useAppStore.getState().loadQualityRuns(worktreeId)
    }
  }, [enabled, worktreeId, loaded])
  return useMemo(() => (enabled ? pickLatestFinishedLocalRun(runs) : null), [enabled, runs])
}
