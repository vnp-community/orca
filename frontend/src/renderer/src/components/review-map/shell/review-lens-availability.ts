/**
 * review-lens-availability.ts — FE-CV-TASK-058-03
 *
 * Lenses whose backend channel answered `unsupported`/`disabled` report themselves here so the
 * tab strip can hide them (no dead-end tab, no retry). Per worktree, in memory only: a new
 * session probes again.
 *
 * @module components/review-map/shell/review-lens-availability
 */

import { useSyncExternalStore } from 'react'

const EMPTY: ReadonlySet<string> = new Set()
let byWorktree = new Map<string, ReadonlySet<string>>()
const listeners = new Set<() => void>()

function emit(): void {
  for (const l of listeners) {
    l()
  }
}

export function setReviewLensUnavailable(
  worktreeId: string,
  lensId: string,
  unavailable: boolean
): void {
  const current = byWorktree.get(worktreeId) ?? EMPTY
  if (current.has(lensId) === unavailable) {
    return
  }
  const next = new Set(current)
  if (unavailable) {
    next.add(lensId)
  } else {
    next.delete(lensId)
  }
  // New Map so the snapshot identity changes only when something really changed.
  byWorktree = new Map(byWorktree).set(worktreeId, next)
  emit()
}

export function resetReviewLensAvailability(): void {
  byWorktree = new Map()
  emit()
}

export function useUnavailableReviewLenses(worktreeId: string): ReadonlySet<string> {
  return useSyncExternalStore(
    (cb) => {
      listeners.add(cb)
      return () => listeners.delete(cb)
    },
    () => byWorktree.get(worktreeId) ?? EMPTY,
    () => EMPTY
  )
}
