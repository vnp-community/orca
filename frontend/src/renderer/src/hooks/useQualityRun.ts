/**
 * useQualityRun.ts — FE-CV-TASK-087-03
 *
 * Run state machine of a worktree for the quality lens: current run, start/cancel, and
 * re-attachment to a run already in progress (reopened tab, run started elsewhere). A stream
 * resync re-reads the active run once so a missed `quality.finished` cannot leave a stale spinner.
 *
 * @module hooks/useQualityRun
 */

import { useCallback, useEffect } from 'react'
import { useAppStore } from '@/store'
import { retainQualityEventSync } from '../lib/code-intel-quality-event-sync'
import type {
  ActiveQualityRun,
  QualityRunError,
  QualityStartRequest
} from '../store/slices/code-intel-quality-state'
import { useQualitySupport } from './useQualitySupport'

export type UseQualityRunResult = {
  run: ActiveQualityRun | null
  runError: QualityRunError | null
  start: (request: QualityStartRequest) => Promise<void>
  cancel: () => Promise<void>
  dismiss: () => void
}

export function useQualityRun(worktreeId: string | null | undefined): UseQualityRunResult {
  const support = useQualitySupport(worktreeId)
  const enabled = support === 'enabled' && Boolean(worktreeId)
  const run = useAppStore((s) =>
    worktreeId ? (s.codeIntelQualityByWorktree[worktreeId]?.run ?? null) : null
  )
  const runError = useAppStore((s) =>
    worktreeId ? (s.codeIntelQualityByWorktree[worktreeId]?.runError ?? null) : null
  )
  const resync = useAppStore((s) => s.codeIntelResyncCounter)

  useEffect(() => {
    if (!enabled) {
      return
    }
    return retainQualityEventSync()
  }, [enabled])

  useEffect(() => {
    if (enabled && worktreeId) {
      void useAppStore.getState().loadQualityRuns(worktreeId, { force: true })
    }
  }, [enabled, worktreeId])

  useEffect(() => {
    if (enabled && worktreeId && resync > 0) {
      void useAppStore.getState().refreshQualityRun(worktreeId)
    }
  }, [enabled, worktreeId, resync])

  const start = useCallback(
    async (request: QualityStartRequest) => {
      if (enabled && worktreeId) {
        await useAppStore.getState().startQualityRun(worktreeId, request)
      }
    },
    [enabled, worktreeId]
  )
  const cancel = useCallback(async () => {
    if (enabled && worktreeId) {
      await useAppStore.getState().cancelQualityRun(worktreeId)
    }
  }, [enabled, worktreeId])
  const dismiss = useCallback(() => {
    if (worktreeId) {
      useAppStore.getState().dismissQualityRun(worktreeId)
    }
  }, [worktreeId])

  return { run: enabled ? run : null, runError: enabled ? runError : null, start, cancel, dismiss }
}
