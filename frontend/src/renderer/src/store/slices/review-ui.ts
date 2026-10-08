/**
 * review-ui.ts — FE-CV-TASK-051-01
 *
 * Per-worktree UI state of the Review workspace (scope, lens, chip filter, selection).
 * Why not persisted: it is cheap to rebuild and only layout geometry is stored locally
 * (review-layout-storage.ts).
 */

import type { StateCreator } from 'zustand'
import type { AppState } from '../types'
import type { ReviewScope } from '../../components/review-map/review-scope-model'
import type { ReviewChipId } from '../../components/review-map/review-chip-filter'

export type ReviewUiState = {
  /** null until the default scope is resolved from the branch compare summary. */
  scope: ReviewScope | null
  /** Registry lens id; null = first available lens. */
  lens: string | null
  chipFilter: ReviewChipId | null
  selectedSymbolKey: string | null
  scopePickerOpen: boolean
  drawerOpen: boolean
  /** Architecture lens: selected container; null = pick the default from the change set. */
  c4ContainerId?: string | null
  /** Architecture lens: unsaved c4.yaml drafts by container (<= C4_DRAFT_MAX_CONTAINERS). */
  c4Drafts?: Record<string, string>
  /** Data-flow lens: selected flow id. */
  dataFlowId?: string | null
  /** Impact lens: symbol key at the centre; null/absent = follow the selected changed symbol. */
  impactFocusKey?: string | null
  /** Who made the current selection; 'diff' highlights without opening the drawer. */
  selectedSymbolSource?: 'user' | 'diff'
  /** ERD lens: selected service (null = default), back-stack for ghost-table hops, selected table key. */
  erdService?: string | null
  erdServiceHistory?: string[]
  selectedErdTable?: string | null
  /** Storage lens: environment toggle and selected node id. */
  storageEnv?: 'dev' | 'prod'
  selectedStorageNodeId?: string | null
}

/** Back-stack bound for ghost-table hops between ERD services. */
export const ERD_SERVICE_HISTORY_MAX = 16

/** Drafts are bounded so a forgotten editor can't grow the store without limit. */
export const C4_DRAFT_MAX_CONTAINERS = 8
export const C4_DRAFT_MAX_BYTES = 64 * 1024

export const DEFAULT_REVIEW_UI_STATE: ReviewUiState = {
  scope: null,
  lens: null,
  chipFilter: null,
  selectedSymbolKey: null,
  scopePickerOpen: false,
  drawerOpen: false
}

export type ReviewUiSlice = {
  reviewUiByWorktree: Record<string, ReviewUiState>
  /** Changing scope clears chip filter and selection (they refer to the old scope). */
  setReviewScope: (worktreeId: string, scope: ReviewScope) => void
  setReviewLens: (worktreeId: string, lens: string) => void
  /** Toggles: same chip again clears it. One filter at a time. */
  toggleReviewChipFilter: (worktreeId: string, chip: ReviewChipId) => void
  clearReviewChipFilter: (worktreeId: string) => void
  selectReviewSymbol: (worktreeId: string, symbolKey: string | null) => void
  setReviewDrawerOpen: (worktreeId: string, open: boolean) => void
  setReviewScopePickerOpen: (worktreeId: string, open: boolean) => void
  setReviewC4Container: (worktreeId: string, containerId: string | null) => void
  /** null clears the draft; oversize text is ignored; the oldest draft is evicted at the cap. */
  setReviewC4Draft: (worktreeId: string, containerId: string, text: string | null) => void
  setReviewDataFlowId: (worktreeId: string, flowId: string | null) => void
  setReviewImpactFocus: (worktreeId: string, symbolKey: string | null) => void
  /** Diff cursor -> graph: highlight only; never opens the drawer. */
  selectReviewSymbolFromDiff: (worktreeId: string, symbolKey: string) => void
  /** Pushes the previous service onto the back-stack; also clears the table selection. */
  setErdService: (worktreeId: string, service: string | null) => void
  goBackErdService: (worktreeId: string) => void
  selectErdTable: (worktreeId: string, tableKey: string | null) => void
  setStorageEnv: (worktreeId: string, env: 'dev' | 'prod') => void
  selectStorageNode: (worktreeId: string, nodeId: string | null) => void
}

/** Keys purged when a worktree is removed (both removal paths in worktrees.ts). */
export const REVIEW_UI_WORKTREE_KEYED_STATE_KEYS = ['reviewUiByWorktree'] as const

export const createReviewUiSlice: StateCreator<AppState, [], [], ReviewUiSlice> = (set) => {
  const patch = (worktreeId: string, fn: (prev: ReviewUiState) => Partial<ReviewUiState>): void =>
    set((s) => {
      const prev = s.reviewUiByWorktree[worktreeId] ?? DEFAULT_REVIEW_UI_STATE
      return {
        reviewUiByWorktree: { ...s.reviewUiByWorktree, [worktreeId]: { ...prev, ...fn(prev) } }
      }
    })

  return {
    reviewUiByWorktree: {},
    setReviewScope: (worktreeId, scope) =>
      patch(worktreeId, () => ({
        scope,
        chipFilter: null,
        selectedSymbolKey: null,
        drawerOpen: false
      })),
    setReviewLens: (worktreeId, lens) => patch(worktreeId, () => ({ lens })),
    toggleReviewChipFilter: (worktreeId, chip) =>
      patch(worktreeId, (prev) => ({ chipFilter: prev.chipFilter === chip ? null : chip })),
    clearReviewChipFilter: (worktreeId) => patch(worktreeId, () => ({ chipFilter: null })),
    selectReviewSymbol: (worktreeId, symbolKey) =>
      patch(worktreeId, () => ({
        selectedSymbolKey: symbolKey,
        selectedSymbolSource: 'user',
        drawerOpen: symbolKey !== null
      })),
    setReviewDrawerOpen: (worktreeId, open) => patch(worktreeId, () => ({ drawerOpen: open })),
    setReviewScopePickerOpen: (worktreeId, open) =>
      patch(worktreeId, () => ({ scopePickerOpen: open })),
    setReviewC4Container: (worktreeId, containerId) =>
      patch(worktreeId, () => ({ c4ContainerId: containerId })),
    setReviewC4Draft: (worktreeId, containerId, text) =>
      patch(worktreeId, (prev) => {
        const drafts = { ...prev.c4Drafts }
        if (text === null) {
          delete drafts[containerId]
          return { c4Drafts: drafts }
        }
        if (new TextEncoder().encode(text).length > C4_DRAFT_MAX_BYTES) {
          return {}
        }
        // Re-insert so the most recently edited draft is last (eviction takes the first key).
        delete drafts[containerId]
        drafts[containerId] = text
        for (const key of Object.keys(drafts)) {
          if (Object.keys(drafts).length <= C4_DRAFT_MAX_CONTAINERS) {
            break
          }
          delete drafts[key]
        }
        return { c4Drafts: drafts }
      }),
    setReviewDataFlowId: (worktreeId, flowId) => patch(worktreeId, () => ({ dataFlowId: flowId })),
    setReviewImpactFocus: (worktreeId, symbolKey) =>
      patch(worktreeId, () => ({ impactFocusKey: symbolKey })),
    selectReviewSymbolFromDiff: (worktreeId, symbolKey) =>
      patch(worktreeId, () => ({ selectedSymbolKey: symbolKey, selectedSymbolSource: 'diff' })),
    setErdService: (worktreeId, service) =>
      patch(worktreeId, (prev) => {
        if ((prev.erdService ?? null) === service) {
          return {}
        }
        const history = [...(prev.erdServiceHistory ?? [])]
        if (prev.erdService) {
          history.push(prev.erdService)
        }
        return {
          erdService: service,
          erdServiceHistory: history.slice(-ERD_SERVICE_HISTORY_MAX),
          selectedErdTable: null
        }
      }),
    goBackErdService: (worktreeId) =>
      patch(worktreeId, (prev) => {
        const history = [...(prev.erdServiceHistory ?? [])]
        const previous = history.pop()
        return previous === undefined
          ? {}
          : { erdService: previous, erdServiceHistory: history, selectedErdTable: null }
      }),
    selectErdTable: (worktreeId, tableKey) =>
      patch(worktreeId, () => ({ selectedErdTable: tableKey })),
    setStorageEnv: (worktreeId, env) =>
      patch(worktreeId, () => ({ storageEnv: env, selectedStorageNodeId: null })),
    selectStorageNode: (worktreeId, nodeId) =>
      patch(worktreeId, () => ({ selectedStorageNodeId: nodeId }))
  }
}
