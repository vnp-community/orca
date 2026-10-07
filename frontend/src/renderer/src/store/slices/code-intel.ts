/**
 * code-intel.ts — FE-CV-TASK-050-10
 *
 * Zustand store slice for code-intel state.
 * Manages:
 * - Support/settings state (enabled/disabled/unknown/unsupported)
 * - Per-worktree result cache (LRU: 16 items/worktree, 6 worktrees max)
 * - Stale signal per worktree
 * - Worktree purge (both removeWorktree and buildWorktreePurgeState paths)
 *
 * @module store/slices/code-intel
 */

// ---------------------------------------------------------------------------
// Support state shape
// ---------------------------------------------------------------------------

export type CodeIntelSupportState = {
  state: 'unknown' | 'enabled' | 'disabled' | 'unsupported'
  effective?: {
    codeIntelEnabled?: boolean
    qualityGateEnabled?: boolean
    aiReviewEnabled?: boolean
  } | null
  lastPolledAt?: string | null
}

// ---------------------------------------------------------------------------
// LRU cache types
// ---------------------------------------------------------------------------

type LruEntry<T> = { key: string; value: T; ts: number }

/**
 * Simple LRU cache with a fixed capacity.
 * Evicts least-recently-used entries when capacity is exceeded.
 */
class LruCache<T> {
  private map = new Map<string, LruEntry<T>>()
  constructor(private capacity: number) {}

  get(key: string): T | undefined {
    const entry = this.map.get(key)
    if (!entry) return undefined
    // Move to end (most recently used)
    this.map.delete(key)
    this.map.set(key, { ...entry, ts: Date.now() })
    return entry.value
  }

  set(key: string, value: T): void {
    this.map.delete(key) // Remove if exists to update position
    if (this.map.size >= this.capacity) {
      // Evict oldest
      const firstKey = this.map.keys().next().value
      if (firstKey !== undefined) this.map.delete(firstKey)
    }
    this.map.set(key, { key, value, ts: Date.now() })
  }

  delete(key: string): void {
    this.map.delete(key)
  }

  clear(): void {
    this.map.clear()
  }

  size(): number {
    return this.map.size
  }

  keys(): string[] {
    return Array.from(this.map.keys())
  }
}

// ---------------------------------------------------------------------------
// Per-worktree state
// ---------------------------------------------------------------------------

export type CodeIntelWorktreeState = {
  /** LRU cache of RPC results keyed by method:params-hash */
  resultCache: LruCache<unknown>
  /** Stale signal: when set, results are considered stale */
  stale: boolean
  /** Last resync counter value when this worktree was synced */
  lastSyncCounter: number
}

// ---------------------------------------------------------------------------
// Slice state type
// ---------------------------------------------------------------------------

export type CodeIntelSlice = {
  /** Global support/settings state */
  codeIntelSupportState: CodeIntelSupportState

  /** Per-worktree state map */
  codeIntelWorktreeState: Record<string, CodeIntelWorktreeState>

  /** Resync counter: increment triggers a re-fetch */
  codeIntelResyncCounter: number

  // Actions
  setCodeIntelSupportState: (state: CodeIntelSupportState) => void
  invalidateCodeIntelWorktree: (worktreeId: string) => void
  setCacheResult: (worktreeId: string, cacheKey: string, value: unknown) => void
  getCacheResult: (worktreeId: string, cacheKey: string) => unknown
  pruneCodeIntelWorktrees: (liveWorktreeIds: string[]) => void
  triggerCodeIntelResync: () => void
}

/** Keys that should be pruned when a worktree is removed */
export const CODE_INTEL_WORKTREE_KEYED_STATE_KEYS = ['codeIntelWorktreeState'] as const

const MAX_RESULT_CACHE_PER_WORKTREE = 16
const MAX_CACHED_WORKTREES = 6

// Tracks worktree LRU order for the 6-worktree limit
const worktreeAccessOrder: string[] = []

function touchWorktree(worktreeId: string): void {
  const idx = worktreeAccessOrder.indexOf(worktreeId)
  if (idx !== -1) worktreeAccessOrder.splice(idx, 1)
  worktreeAccessOrder.push(worktreeId)
}

function getOrCreateWorktreeState(
  state: Record<string, CodeIntelWorktreeState>,
  worktreeId: string
): CodeIntelWorktreeState {
  return state[worktreeId] ?? {
    resultCache: new LruCache<unknown>(MAX_RESULT_CACHE_PER_WORKTREE),
    stale: false,
    lastSyncCounter: 0
  }
}

// ---------------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------------

export function createCodeIntelSlice(
  set: (fn: (prev: { codeIntelSupportState: CodeIntelSupportState; codeIntelWorktreeState: Record<string, CodeIntelWorktreeState>; codeIntelResyncCounter: number }) => Partial<{ codeIntelSupportState: CodeIntelSupportState; codeIntelWorktreeState: Record<string, CodeIntelWorktreeState>; codeIntelResyncCounter: number }>) => void,
  get: () => { codeIntelSupportState: CodeIntelSupportState; codeIntelWorktreeState: Record<string, CodeIntelWorktreeState>; codeIntelResyncCounter: number }
): CodeIntelSlice {
  return {
    codeIntelSupportState: { state: 'unknown' },
    codeIntelWorktreeState: {},
    codeIntelResyncCounter: 0,

    setCodeIntelSupportState(newState) {
      set(() => ({ codeIntelSupportState: newState }))
    },

    invalidateCodeIntelWorktree(worktreeId) {
      set((prev) => {
        const current = getOrCreateWorktreeState(prev.codeIntelWorktreeState, worktreeId)
        current.resultCache.clear()
        return {
          codeIntelWorktreeState: {
            ...prev.codeIntelWorktreeState,
            [worktreeId]: { ...current, stale: true }
          }
        }
      })
    },

    setCacheResult(worktreeId, cacheKey, value) {
      touchWorktree(worktreeId)

      // Evict oldest worktrees if over MAX_CACHED_WORKTREES
      set((prev) => {
        const state = { ...prev.codeIntelWorktreeState }
        // Prune excess worktrees (oldest first from worktreeAccessOrder)
        while (worktreeAccessOrder.length > MAX_CACHED_WORKTREES) {
          const oldest = worktreeAccessOrder.shift()
          if (oldest) delete state[oldest]
        }

        const worktreeState = getOrCreateWorktreeState(state, worktreeId)
        worktreeState.resultCache.set(cacheKey, value)
        return {
          codeIntelWorktreeState: {
            ...state,
            [worktreeId]: worktreeState
          }
        }
      })
    },

    getCacheResult(worktreeId, cacheKey) {
      const state = get()
      return state.codeIntelWorktreeState[worktreeId]?.resultCache.get(cacheKey)
    },

    pruneCodeIntelWorktrees(liveWorktreeIds) {
      const liveSet = new Set(liveWorktreeIds)
      set((prev) => {
        const next: Record<string, CodeIntelWorktreeState> = {}
        for (const [id, s] of Object.entries(prev.codeIntelWorktreeState)) {
          if (liveSet.has(id)) next[id] = s
        }
        // Clean up access order tracking
        for (let i = worktreeAccessOrder.length - 1; i >= 0; i--) {
          if (!liveSet.has(worktreeAccessOrder[i])) {
            worktreeAccessOrder.splice(i, 1)
          }
        }
        return { codeIntelWorktreeState: next }
      })
    },

    triggerCodeIntelResync() {
      set((prev) => ({ codeIntelResyncCounter: prev.codeIntelResyncCounter + 1 }))
    }
  }
}
