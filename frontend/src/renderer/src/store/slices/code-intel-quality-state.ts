/**
 * code-intel-quality-state.ts — FE-CV-TASK-087-02
 *
 * Zustand slice for quality scan state per worktree.
 * Store shape: codeIntelQualityByWorktree: Record<worktreeId, QualityWorktreeState>
 *
 * Actions:
 * - startQualityRun / cancelQualityRun
 * - loadQualityGate / loadQualityRuns / loadQualityProfiles
 * - attachQualityRun (when external run detected)
 * - invalidateQuality
 * - setQualityUi
 *
 * @module store/slices/code-intel-quality-state
 */

import type { QualityGate, QualityRun, QualityProfile } from '../../../../shared/code-intel-quality-types'

// ---------------------------------------------------------------------------
// State shape
// ---------------------------------------------------------------------------

export type QualityRunPhase =
  | 'idle'
  | 'starting'
  | 'running'
  | 'cancelling'
  | 'completed'
  | 'failed'
  | 'cancelled'
  | 'interrupted'

export type ActiveQualityRun = {
  runId: string
  phase: QualityRunPhase
  percent: number | null
  error: string | null
  /** Is this run started by this client (vs detected from push events) */
  isOwned: boolean
}

export type QualityWorktreeState = {
  gate: QualityGate | null
  gateStale: boolean
  runs: QualityRun[]
  profile: QualityProfile | null
  activeRun: ActiveQualityRun | null
  /** Whether gate/runs/findings data is loading */
  isLoading: boolean
  /** UI state: which tab is active in the quality panel */
  uiTab: 'scorecard' | 'findings' | 'trend' | 'coverage'
}

const DEFAULT_WORKTREE_STATE: QualityWorktreeState = {
  gate: null,
  gateStale: false,
  runs: [],
  profile: null,
  activeRun: null,
  isLoading: false,
  uiTab: 'scorecard'
}

// ---------------------------------------------------------------------------
// Slice state type
// ---------------------------------------------------------------------------

export type CodeIntelQualitySlice = {
  codeIntelQualityByWorktree: Record<string, QualityWorktreeState>

  startQualityRun: (worktreeId: string) => void
  cancelQualityRun: (worktreeId: string) => void
  attachQualityRun: (worktreeId: string, runId: string) => void
  loadQualityGate: (worktreeId: string, opts?: { force?: boolean }) => void
  loadQualityRuns: (worktreeId: string, opts?: { force?: boolean }) => void
  loadQualityProfiles: (worktreeId: string, opts?: { force?: boolean }) => void
  invalidateQuality: (worktreeId: string) => void
  setQualityUi: (worktreeId: string, tab: QualityWorktreeState['uiTab']) => void

  /** Internal: apply a push event to quality state */
  applyQualityPushEvent: (
    event:
      | { event: 'qualityProgress'; worktreeId: string; runId: string; percent: number | null }
      | { event: 'qualityFinished'; worktreeId: string; runId: string; success: boolean; error: string | null }
      | { event: 'gateChanged'; worktreeId: string; gate?: QualityGate }
  ) => void

  /** Internal: prune quality state for removed worktrees */
  pruneCodeIntelQualityWorktrees: (liveWorktreeIds: string[]) => void
}

// ---------------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------------

/**
 * Create the quality store slice.
 * Pass the set function from Zustand's StateCreator.
 */
export function createCodeIntelQualitySlice(
  set: (fn: (prev: { codeIntelQualityByWorktree: Record<string, QualityWorktreeState> }) => Partial<{ codeIntelQualityByWorktree: Record<string, QualityWorktreeState> }>) => void
): CodeIntelQualitySlice {

  function getOrDefault(
    state: { codeIntelQualityByWorktree: Record<string, QualityWorktreeState> },
    worktreeId: string
  ): QualityWorktreeState {
    return state.codeIntelQualityByWorktree[worktreeId] ?? DEFAULT_WORKTREE_STATE
  }

  function patchWorktree(
    worktreeId: string,
    patch: Partial<QualityWorktreeState>
  ): void {
    set((prev) => {
      const current = getOrDefault(prev, worktreeId)
      return {
        codeIntelQualityByWorktree: {
          ...prev.codeIntelQualityByWorktree,
          [worktreeId]: { ...current, ...patch }
        }
      }
    })
  }

  return {
    codeIntelQualityByWorktree: {},

    startQualityRun(worktreeId) {
      set((prev) => {
        const current = getOrDefault(prev, worktreeId)
        // Idempotent: lock immediately to prevent double-start
        if (current.activeRun?.phase === 'starting' || current.activeRun?.phase === 'running') {
          return {}
        }
        return {
          codeIntelQualityByWorktree: {
            ...prev.codeIntelQualityByWorktree,
            [worktreeId]: {
              ...current,
              activeRun: { runId: '', phase: 'starting', percent: null, error: null, isOwned: true }
            }
          }
        }
      })
    },

    cancelQualityRun(worktreeId) {
      set((prev) => {
        const current = getOrDefault(prev, worktreeId)
        if (!current.activeRun) return {}
        return {
          codeIntelQualityByWorktree: {
            ...prev.codeIntelQualityByWorktree,
            [worktreeId]: {
              ...current,
              activeRun: { ...current.activeRun, phase: 'cancelling' }
            }
          }
        }
      })
    },

    attachQualityRun(worktreeId, runId) {
      set((prev) => {
        const current = getOrDefault(prev, worktreeId)
        // If starting and we now have runId, update it; otherwise create external run
        const existing = current.activeRun
        const isOwned = existing?.phase === 'starting'
        return {
          codeIntelQualityByWorktree: {
            ...prev.codeIntelQualityByWorktree,
            [worktreeId]: {
              ...current,
              activeRun: {
                runId,
                phase: 'running',
                percent: existing?.percent ?? null,
                error: null,
                isOwned: isOwned ?? false
              }
            }
          }
        }
      })
    },

    loadQualityGate(worktreeId, _opts) {
      // Async load handled by hook layer (useQualityGate hook, 087-03)
      patchWorktree(worktreeId, { isLoading: true })
    },

    loadQualityRuns(worktreeId, _opts) {
      patchWorktree(worktreeId, { isLoading: true })
    },

    loadQualityProfiles(worktreeId, _opts) {
      patchWorktree(worktreeId, { isLoading: true })
    },

    invalidateQuality(worktreeId) {
      patchWorktree(worktreeId, { gateStale: true })
    },

    setQualityUi(worktreeId, tab) {
      patchWorktree(worktreeId, { uiTab: tab })
    },

    applyQualityPushEvent(event) {
      const { worktreeId } = event
      set((prev) => {
        const current = prev.codeIntelQualityByWorktree[worktreeId]
        // Silently ignore events for unknown worktrees
        if (!current) return {}

        if (event.event === 'qualityProgress') {
          const run = current.activeRun
          if (!run) {
            // External run — attach it
            return {
              codeIntelQualityByWorktree: {
                ...prev.codeIntelQualityByWorktree,
                [worktreeId]: {
                  ...current,
                  activeRun: { runId: event.runId, phase: 'running', percent: event.percent, error: null, isOwned: false }
                }
              }
            }
          }
          return {
            codeIntelQualityByWorktree: {
              ...prev.codeIntelQualityByWorktree,
              [worktreeId]: {
                ...current,
                activeRun: { ...run, phase: 'running', percent: event.percent }
              }
            }
          }
        }

        if (event.event === 'qualityFinished') {
          const phase: QualityRunPhase = event.success
            ? 'completed'
            : (event.error?.includes('interrupted') ? 'interrupted' : 'failed')
          return {
            codeIntelQualityByWorktree: {
              ...prev.codeIntelQualityByWorktree,
              [worktreeId]: {
                ...current,
                activeRun: current.activeRun
                  ? { ...current.activeRun, phase, error: event.error }
                  : null,
                // Mark gate as stale so hooks reload it
                gateStale: true
              }
            }
          }
        }

        if (event.event === 'gateChanged') {
          return {
            codeIntelQualityByWorktree: {
              ...prev.codeIntelQualityByWorktree,
              [worktreeId]: {
                ...current,
                gate: event.gate ?? null,
                gateStale: false
              }
            }
          }
        }

        return {}
      })
    },

    pruneCodeIntelQualityWorktrees(liveWorktreeIds) {
      const liveSet = new Set(liveWorktreeIds)
      set((prev) => {
        const next: Record<string, QualityWorktreeState> = {}
        for (const [id, state] of Object.entries(prev.codeIntelQualityByWorktree)) {
          if (liveSet.has(id)) next[id] = state
        }
        return { codeIntelQualityByWorktree: next }
      })
    }
  }
}

/** Keys that should be pruned when a worktree is removed */
export const CODE_INTEL_QUALITY_WORKTREE_KEYS = ['codeIntelQualityByWorktree'] as const
