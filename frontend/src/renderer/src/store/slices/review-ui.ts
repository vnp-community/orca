/**
 * review-ui.ts — FE-CV-TASK-051-01
 *
 * Zustand store slice for Review workspace UI state.
 * Manages active lens, scope, detail drawer state per worktree.
 *
 * @module store/slices/review-ui
 */

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type ReviewLensId =
  | 'overview'
  | 'reading-order'
  | 'finding'
  | 'symbol'
  | 'dependency'
  | 'quality'
  | 'data-flow'
  | 'c4'

export type ReviewUiWorktreeState = {
  activeLensId: ReviewLensId
  /** Selected scope (overlay path or null for whole review) */
  selectedPath: string | null
  /** Whether the detail drawer is open */
  drawerOpen: boolean
  /** Selected item ID in the drawer (symbol, finding, etc.) */
  drawerItemId: string | null
  /** Active chip filters (severity, category, etc.) */
  chipFilters: string[]
}

const DEFAULT_STATE: ReviewUiWorktreeState = {
  activeLensId: 'overview',
  selectedPath: null,
  drawerOpen: false,
  drawerItemId: null,
  chipFilters: []
}

export type ReviewUiSlice = {
  reviewUiByWorktree: Record<string, ReviewUiWorktreeState>

  setReviewLens: (worktreeId: string, lensId: ReviewLensId) => void
  setReviewSelectedPath: (worktreeId: string, path: string | null) => void
  openReviewDrawer: (worktreeId: string, itemId: string) => void
  closeReviewDrawer: (worktreeId: string) => void
  toggleReviewChipFilter: (worktreeId: string, filter: string) => void
  clearReviewChipFilters: (worktreeId: string) => void
  pruneReviewUiWorktrees: (liveWorktreeIds: string[]) => void
}

// ---------------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------------

export function createReviewUiSlice(
  set: (fn: (prev: { reviewUiByWorktree: Record<string, ReviewUiWorktreeState> }) => Partial<{ reviewUiByWorktree: Record<string, ReviewUiWorktreeState> }>) => void
): ReviewUiSlice {
  function patchWorktree(worktreeId: string, patch: Partial<ReviewUiWorktreeState>): void {
    set((prev) => ({
      reviewUiByWorktree: {
        ...prev.reviewUiByWorktree,
        [worktreeId]: { ...(prev.reviewUiByWorktree[worktreeId] ?? DEFAULT_STATE), ...patch }
      }
    }))
  }

  return {
    reviewUiByWorktree: {},

    setReviewLens(worktreeId, lensId) {
      patchWorktree(worktreeId, { activeLensId: lensId })
    },

    setReviewSelectedPath(worktreeId, path) {
      patchWorktree(worktreeId, { selectedPath: path })
    },

    openReviewDrawer(worktreeId, itemId) {
      patchWorktree(worktreeId, { drawerOpen: true, drawerItemId: itemId })
    },

    closeReviewDrawer(worktreeId) {
      patchWorktree(worktreeId, { drawerOpen: false, drawerItemId: null })
    },

    toggleReviewChipFilter(worktreeId, filter) {
      set((prev) => {
        const current = prev.reviewUiByWorktree[worktreeId] ?? DEFAULT_STATE
        const has = current.chipFilters.includes(filter)
        return {
          reviewUiByWorktree: {
            ...prev.reviewUiByWorktree,
            [worktreeId]: {
              ...current,
              chipFilters: has
                ? current.chipFilters.filter((f) => f !== filter)
                : [...current.chipFilters, filter]
            }
          }
        }
      })
    },

    clearReviewChipFilters(worktreeId) {
      patchWorktree(worktreeId, { chipFilters: [] })
    },

    pruneReviewUiWorktrees(liveWorktreeIds) {
      const liveSet = new Set(liveWorktreeIds)
      set((prev) => {
        const next: Record<string, ReviewUiWorktreeState> = {}
        for (const [id, s] of Object.entries(prev.reviewUiByWorktree)) {
          if (liveSet.has(id)) next[id] = s
        }
        return { reviewUiByWorktree: next }
      })
    }
  }
}
