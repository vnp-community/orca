/**
 * Which rows of a review the viewer marked seen — FE-OR-T203
 *
 * Kept in this browser only, by review id. It is a convenience for one person reading one review, not a record: it is lost on
 * another machine, and storage that is blocked or full is ignored rather than reported.
 *
 * @module components/request/outcome-review/diff/outcome-review-seen
 */

import { useCallback, useState } from 'react'

const PREFIX = 'orca.outcomeReview.seen.'
/** A review has far fewer rows than this; the cap only stops a runaway list from filling storage. */
const MAX_KEYS = 2000

export function readSeen(reviewId: string): Set<string> {
  try {
    const raw = window.localStorage.getItem(PREFIX + reviewId)
    const parsed: unknown = raw ? JSON.parse(raw) : []
    return new Set(
      Array.isArray(parsed) ? parsed.filter((k): k is string => typeof k === 'string') : []
    )
  } catch {
    return new Set()
  }
}

export function writeSeen(reviewId: string, seen: ReadonlySet<string>): void {
  try {
    window.localStorage.setItem(PREFIX + reviewId, JSON.stringify([...seen].slice(-MAX_KEYS)))
  } catch {
    // Storage may be blocked (private window) or full; marking rows seen then lasts until the page is closed.
  }
}

export function useSeenRows(reviewId: string): {
  seen: ReadonlySet<string>
  setSeen: (key: string, value: boolean) => void
} {
  const [state, setState] = useState<{ id: string; seen: Set<string> }>(() => ({
    id: reviewId,
    seen: readSeen(reviewId)
  }))
  // Why: another review has its own list; reading it when the id changes keeps a stale list from marking the wrong rows.
  const current = state.id === reviewId ? state.seen : readSeen(reviewId)
  const setSeen = useCallback(
    (key: string, value: boolean) => {
      const next = new Set(current)
      if (value) {
        next.add(key)
      } else {
        next.delete(key)
      }
      writeSeen(reviewId, next)
      setState({ id: reviewId, seen: next })
    },
    [current, reviewId]
  )
  return { seen: current, setSeen }
}
