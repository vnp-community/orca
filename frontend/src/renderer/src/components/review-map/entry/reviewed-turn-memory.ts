/**
 * reviewed-turn-memory.ts — FE-CV-TASK-061-06
 *
 * Session-local memory of the last agent turn the user opened in Review, per worktree. Drives the
 * "finished, not reviewed" dot; deliberately not persisted (it is only a nudge).
 */

import { useSyncExternalStore } from 'react'

const reviewedByWorktree = new Map<string, string>()
const listeners = new Set<() => void>()

export function markTurnReviewed(worktreeId: string, completionId: string): void {
  if (reviewedByWorktree.get(worktreeId) === completionId) {
    return
  }
  reviewedByWorktree.set(worktreeId, completionId)
  listeners.forEach((l) => l())
}

export function getReviewedTurnId(worktreeId: string | null): string | null {
  return worktreeId ? (reviewedByWorktree.get(worktreeId) ?? null) : null
}

export function useReviewedTurnId(worktreeId: string | null): string | null {
  return useSyncExternalStore(
    (cb) => {
      listeners.add(cb)
      return () => listeners.delete(cb)
    },
    () => getReviewedTurnId(worktreeId),
    () => null
  )
}

export function resetReviewedTurnMemoryForTests(): void {
  reviewedByWorktree.clear()
}
