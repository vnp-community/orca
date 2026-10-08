/**
 * review-turn-overlay-store.ts — FE-CV-TASK-060-08
 *
 * The active "since previous turn" comparison per worktree, published by the workspace and read
 * by lens canvases to label nodes. A module store so lenses need no new registry props.
 *
 * @module components/review-map/turns/review-turn-overlay-store
 */

import { useCallback, useSyncExternalStore } from 'react'
import type { TurnCompareResult } from './turn-compare-model'

const byWorktree = new Map<string, TurnCompareResult>()
const listeners = new Set<() => void>()

export function setReviewTurnOverlay(worktreeId: string, compare: TurnCompareResult | null): void {
  if (compare) {
    byWorktree.set(worktreeId, compare)
  } else if (!byWorktree.delete(worktreeId)) {
    return
  }
  for (const listener of listeners) {
    listener()
  }
}

export function getReviewTurnOverlay(worktreeId: string | null): TurnCompareResult | null {
  return worktreeId ? (byWorktree.get(worktreeId) ?? null) : null
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

export function useReviewTurnOverlay(worktreeId: string | null): TurnCompareResult | null {
  const get = useCallback(() => getReviewTurnOverlay(worktreeId), [worktreeId])
  return useSyncExternalStore(subscribe, get)
}
