/**
 * review-progress.ts — FE-CV-TASK-052-03
 *
 * Reading progress of a review, persisted in the backend (codeIntel.reviewState.*), never in
 * localStorage (cleared on logout). Holds the whole ReviewState so notes/turnMarkers are
 * re-sent verbatim and a second writer (SOL-060) can reuse patchReviewState + the same queue.
 */

import type { StateCreator } from 'zustand'
import type { AppState } from '../types'
import type { ReviewDataApi } from '../../components/review-map/review-shell-data'
import { defaultReviewDataApi } from '../../components/review-map/review-data-api-default'
import {
  mergeReadingProgress,
  pruneReadingProgress
} from '../../components/review-map/reading-progress-merge'
import type { ReviewProgressEntry, ReviewProgressSaveStatus } from './review-progress-entry'
import {
  emptyReadingProgress,
  type ReadingProgress,
  type ReviewStateView
} from '../../components/review-map/review-wire-types'

export const REVIEW_PROGRESS_DEBOUNCE_MS = 800
export const REVIEW_PROGRESS_RETRY_MS = 15_000
const MAX_CONFLICT_ROUNDS = 3

export type ReviewProgressSlice = {
  reviewProgressByWorktree: Record<string, ReviewProgressEntry>
  loadReviewProgress: (worktreeId: string, baseCommit: string, headCommit: string) => Promise<void>
  setReadingItemSeen: (worktreeId: string, stepKey: string, seen: boolean) => void
  setReadingGroupSeen: (worktreeId: string, stepKeys: readonly string[], seen: boolean) => void
  setReadingLastFocused: (worktreeId: string, stepKey: string | null) => void
  /** Second-writer hook (notes, turnMarkers, status) sharing the same save queue. */
  patchReviewState: (worktreeId: string, patch: Partial<ReviewStateView>) => void
  flushReviewProgress: (worktreeId: string) => Promise<void>
  retryReviewProgressSave: (worktreeId: string) => Promise<void>
  dropReviewProgress: (worktreeId: string) => void
}

export const REVIEW_PROGRESS_WORKTREE_KEYED_STATE_KEYS = ['reviewProgressByWorktree'] as const

export type { ReviewProgressEntry, ReviewProgressSaveStatus }

// Timers and in-flight flags live outside the store: they are not render state.
const debounceTimers = new Map<string, ReturnType<typeof setTimeout>>()
const retryTimers = new Map<string, ReturnType<typeof setTimeout>>()
const inFlight = new Map<string, Promise<void>>()
let api: ReviewDataApi = defaultReviewDataApi

/** Test seam. */
export function setReviewProgressApi(next: ReviewDataApi | null): void {
  api = next ?? defaultReviewDataApi
}

function clearTimers(worktreeId: string): void {
  const d = debounceTimers.get(worktreeId)
  if (d) {
    clearTimeout(d)
  }
  debounceTimers.delete(worktreeId)
  const r = retryTimers.get(worktreeId)
  if (r) {
    clearTimeout(r)
  }
  retryTimers.delete(worktreeId)
}

export const createReviewProgressSlice: StateCreator<AppState, [], [], ReviewProgressSlice> = (
  set,
  get
) => {
  const read = (id: string): ReviewProgressEntry | undefined => get().reviewProgressByWorktree[id]
  const write = (id: string, patch: Partial<ReviewProgressEntry>): void =>
    set((s) => {
      const prev = s.reviewProgressByWorktree[id]
      if (!prev) {
        return {}
      }
      return {
        reviewProgressByWorktree: { ...s.reviewProgressByWorktree, [id]: { ...prev, ...patch } }
      }
    })

  function scheduleSave(id: string): void {
    const existing = debounceTimers.get(id)
    if (existing) {
      clearTimeout(existing)
    }
    debounceTimers.set(
      id,
      setTimeout(() => {
        debounceTimers.delete(id)
        void runSave(id)
      }, REVIEW_PROGRESS_DEBOUNCE_MS)
    )
  }

  function scheduleRetry(id: string): void {
    if (retryTimers.has(id)) {
      return
    }
    retryTimers.set(
      id,
      setTimeout(() => {
        retryTimers.delete(id)
        // Hidden windows retry on the next visibility flush instead of spinning.
        if (typeof document !== 'undefined' && document.visibilityState === 'hidden') {
          return
        }
        void runSave(id)
      }, REVIEW_PROGRESS_RETRY_MS)
    )
  }

  // One save in flight per worktree; edits made meanwhile are picked up by the loop (one waiting).
  function runSave(id: string): Promise<void> {
    const running = inFlight.get(id)
    if (running) {
      return running
    }
    const job = (async () => {
      let conflicts = 0
      for (;;) {
        const entry = read(id)
        if (!entry || !entry.serverState || entry.localOnly) {
          return
        }
        if (entry.saveStatus !== 'dirty' && entry.saveStatus !== 'error') {
          return
        }
        const snapshot = entry.progress
        const pruned = pruneReadingProgress(snapshot)
        write(id, { saveStatus: 'saving', oversize: pruned.stillTooLarge })
        const toSend: ReviewStateView = {
          ...entry.serverState,
          baseCommit: entry.baseCommit,
          headCommit: entry.headCommit,
          readingProgress: pruned.progress
        }
        const res = await api.saveReviewState(id, toSend, entry.serverState.version)
        const after = read(id)
        if (!after) {
          return
        }
        if (res.ok) {
          const changedMeanwhile = after.progress !== snapshot
          write(id, {
            serverState: { ...res.value, readingProgress: pruned.progress },
            saveStatus: changedMeanwhile ? 'dirty' : 'saved',
            lastErrorKind: null
          })
          if (!changedMeanwhile) {
            return
          }
          continue
        }
        if (res.error.kind === 'conflict' && conflicts < MAX_CONFLICT_ROUNDS) {
          conflicts += 1
          const fresh = await api.getReviewState(id, {
            baseCommit: entry.baseCommit,
            headCommit: entry.headCommit
          })
          const latest = read(id)
          if (!latest) {
            return
          }
          if (fresh.ok) {
            write(id, {
              serverState: {
                ...latest.serverState!,
                ...fresh.value,
                readingProgress: fresh.value.readingProgress
              },
              progress: mergeReadingProgress(latest.progress, fresh.value.readingProgress),
              saveStatus: 'dirty'
            })
            continue
          }
        }
        write(id, {
          saveStatus: 'error',
          lastErrorKind: res.error.kind,
          localOnly: res.error.kind === 'forbidden'
        })
        if (res.error.kind !== 'forbidden') {
          scheduleRetry(id)
        }
        return
      }
    })().finally(() => inFlight.delete(id))
    inFlight.set(id, job)
    return job
  }

  const markDirty = (id: string, progress: ReadingProgress): void => {
    const entry = read(id)
    if (!entry) {
      return
    }
    write(id, { progress, saveStatus: entry.localOnly ? 'error' : 'dirty' })
    if (!entry.localOnly) {
      scheduleSave(id)
    }
  }

  return {
    reviewProgressByWorktree: {},

    async loadReviewProgress(worktreeId, baseCommit, headCommit) {
      const prev = read(worktreeId)
      if (prev && (prev.baseCommit !== baseCommit || prev.headCommit !== headCommit)) {
        await get().flushReviewProgress(worktreeId)
      }
      set((s) => ({
        reviewProgressByWorktree: {
          ...s.reviewProgressByWorktree,
          [worktreeId]: {
            baseCommit,
            headCommit,
            loadStatus: 'loading',
            loadErrorKind: null,
            serverState: null,
            // New head => empty progress; the old row stays on the backend.
            progress: emptyReadingProgress(),
            saveStatus: 'saved',
            lastErrorKind: null,
            localOnly: false,
            oversize: false
          }
        }
      }))
      const res = await api.getReviewState(worktreeId, { baseCommit, headCommit })
      const cur = read(worktreeId)
      if (!cur || cur.baseCommit !== baseCommit || cur.headCommit !== headCommit) {
        return
      }
      if (!res.ok) {
        write(worktreeId, { loadStatus: 'error', loadErrorKind: res.error.kind })
        return
      }
      const dirtyLocal = cur.saveStatus !== 'saved'
      write(worktreeId, {
        loadStatus: 'ready',
        serverState: res.value,
        progress: dirtyLocal
          ? mergeReadingProgress(cur.progress, res.value.readingProgress)
          : res.value.readingProgress
      })
    },

    setReadingItemSeen(worktreeId, stepKey, seen) {
      const entry = read(worktreeId)
      if (!entry || entry.loadStatus !== 'ready') {
        return
      }
      markDirty(worktreeId, {
        ...entry.progress,
        entries: {
          ...entry.progress.entries,
          [stepKey]: { state: seen ? 'seen' : 'unseen', at: Date.now() }
        }
      })
    },

    setReadingGroupSeen(worktreeId, stepKeys, seen) {
      const entry = read(worktreeId)
      if (!entry || entry.loadStatus !== 'ready' || stepKeys.length === 0) {
        return
      }
      const at = Date.now()
      const entries = { ...entry.progress.entries }
      for (const k of stepKeys) {
        entries[k] = { state: seen ? 'seen' : 'unseen', at }
      }
      markDirty(worktreeId, { ...entry.progress, entries })
    },

    setReadingLastFocused(worktreeId, stepKey) {
      const entry = read(worktreeId)
      if (!entry || entry.loadStatus !== 'ready' || entry.progress.lastFocusedKey === stepKey) {
        return
      }
      markDirty(worktreeId, { ...entry.progress, lastFocusedKey: stepKey })
    },

    patchReviewState(worktreeId, patch) {
      const entry = read(worktreeId)
      if (!entry?.serverState) {
        return
      }
      // Keep readingProgress owned by this slice.
      const { readingProgress: _ignored, ...rest } = patch
      write(worktreeId, {
        serverState: { ...entry.serverState, ...rest },
        saveStatus: entry.localOnly ? 'error' : 'dirty',
        progress: { ...entry.progress }
      })
      if (!entry.localOnly) {
        scheduleSave(worktreeId)
      }
    },

    async flushReviewProgress(worktreeId) {
      const d = debounceTimers.get(worktreeId)
      if (d) {
        clearTimeout(d)
      }
      debounceTimers.delete(worktreeId)
      await runSave(worktreeId)
    },

    async retryReviewProgressSave(worktreeId) {
      const entry = read(worktreeId)
      if (!entry || entry.saveStatus !== 'error' || entry.localOnly) {
        return
      }
      write(worktreeId, { saveStatus: 'dirty' })
      await runSave(worktreeId)
    },

    dropReviewProgress(worktreeId) {
      clearTimers(worktreeId)
      set((s) => {
        if (!(worktreeId in s.reviewProgressByWorktree)) {
          return {}
        }
        const next = { ...s.reviewProgressByWorktree }
        delete next[worktreeId]
        return { reviewProgressByWorktree: next }
      })
    }
  }
}
