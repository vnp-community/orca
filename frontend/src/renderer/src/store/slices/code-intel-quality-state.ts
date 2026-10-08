/**
 * code-intel-quality-state.ts — FE-CV-TASK-087-02
 *
 * Zustand slice for quality state per worktree: `codeIntelQualityByWorktree`.
 * In-memory only (findings and run output must not reach localStorage). Entries are created by
 * the first load/run action for a worktree and removed by both worktree-removal paths.
 *
 * @module store/slices/code-intel-quality-state
 */

import { createQualityLoadActions } from './code-intel-quality-load-actions'
import { createQualityRunActions } from './code-intel-quality-run-actions'
import { defaultQualityCall, patchWorktree } from './code-intel-quality-slice-context'
import type { QualityCall, QualitySliceContext, QualitySliceData } from './code-intel-quality-slice-context'
import type { QualityLoadActions } from './code-intel-quality-load-actions'
import type { QualityRunActions } from './code-intel-quality-run-actions'
import type { QualityUiState, QualityWorktreeState } from './code-intel-quality-state-types'

export * from './code-intel-quality-state-types'
export { isQualityRunActive, QUALITY_RUN_POLL_INTERVAL_MS } from './code-intel-quality-run-actions'
export type { QualityPushEvent, QualityStartRequest } from './code-intel-quality-run-actions'

export type CodeIntelQualitySlice = QualityLoadActions &
  Omit<QualityRunActions, 'attachFromLoadedRuns' | 'stopPollingFor'> & {
    codeIntelQualityByWorktree: Record<string, QualityWorktreeState>
    invalidateQuality: (worktreeId: string) => void
    setQualityUi: (worktreeId: string, patch: Partial<QualityUiState>) => void
    pruneCodeIntelQualityWorktrees: (liveWorktreeIds: string[]) => void
  }

/** Keys that must be pruned when a worktree is removed (both removal paths). */
export const CODE_INTEL_QUALITY_WORKTREE_KEYS = ['codeIntelQualityByWorktree'] as const

export type QualitySliceDeps = { call?: QualityCall; now?: () => number }

export function createCodeIntelQualitySlice(
  set: QualitySliceContext['set'],
  get: QualitySliceContext['get'],
  deps: QualitySliceDeps = {}
): CodeIntelQualitySlice {
  const ctx: QualitySliceContext = {
    set,
    get,
    call: deps.call ?? defaultQualityCall,
    now: deps.now ?? Date.now,
    sequences: new Map(),
    timers: new Map()
  }
  const loaders = createQualityLoadActions(ctx)
  const { attachFromLoadedRuns, stopPollingFor, ...runActions } = createQualityRunActions(ctx, loaders)

  return {
    codeIntelQualityByWorktree: {},
    ...loaders,
    ...runActions,

    // Re-attach to a run that is already in progress once the run list is known.
    loadQualityRuns: async (worktreeId, opts) => {
      await loaders.loadQualityRuns(worktreeId, opts)
      attachFromLoadedRuns(worktreeId)
    },

    invalidateQuality(worktreeId) {
      patchWorktree(ctx, worktreeId, (s) => ({
        epoch: s.epoch + 1,
        findingsByFile: {},
        ...(s.gate ? { gate: { ...s.gate, stale: true } } : {}),
        ...(s.runs ? { runs: { ...s.runs, stale: true } } : {}),
        ...(s.coverage ? { coverage: { ...s.coverage, stale: true } } : {})
      }))
    },

    setQualityUi(worktreeId, patch) {
      patchWorktree(ctx, worktreeId, (s) => ({ ui: { ...s.ui, ...patch } }))
    },

    pruneCodeIntelQualityWorktrees(liveWorktreeIds) {
      const live = new Set(liveWorktreeIds)
      for (const id of ctx.timers.keys()) {
        if (!live.has(id)) {
          stopPollingFor(id)
        }
      }
      for (const key of ctx.sequences.keys()) {
        if (!live.has(key.split('|')[0])) {
          ctx.sequences.delete(key)
        }
      }
      set((prev: QualitySliceData) => {
        const next: Record<string, QualityWorktreeState> = {}
        for (const [id, state] of Object.entries(prev.codeIntelQualityByWorktree)) {
          if (live.has(id)) {
            next[id] = state
          }
        }
        return { codeIntelQualityByWorktree: next }
      })
    }
  }
}
