/**
 * Version history of a Request's analyses (CR-REQ-041): ordering, the number shown for each version, the one-line note
 * and the status badge. Pure so the list stays cheap to test with a hundred versions.
 *
 * @module components/request/solution/solution-version-history
 */

import type { Solution } from '../../../../../shared/request-types'

/** The note under a version is cut here; the full text is one click away. */
export const VERSION_NOTE_MAX_LENGTH = 80
/** Versions per page of the history list. */
export const VERSION_PAGE_SIZE = 20

export type VersionBadge = 'current' | 'superseded' | 'rejected' | 'approved'

/** Newest first: by `seq` when both carry one, else by the older `version`, else by generation time. */
export function orderSolutionsNewestFirst(solutions: Solution[]): Solution[] {
  return [...solutions].sort((a, b) => {
    if (a.seq !== undefined && b.seq !== undefined && a.seq !== b.seq) {
      return b.seq - a.seq
    }
    const byVersion = (b.version ?? 0) - (a.version ?? 0)
    if (byVersion !== 0) {
      return byVersion
    }
    return (b.generatedAt ?? '').localeCompare(a.generatedAt ?? '')
  })
}

/** The version the reviewer is working on: the newest one that has not been replaced. */
export function currentSolutionOf(newestFirst: Solution[]): Solution | undefined {
  return newestFirst.find((s) => s.status !== 'superseded') ?? newestFirst[0]
}

/** "Version n": the backend's `seq`, or the place in the order when an older gateway sent none. */
export function solutionVersionNumbers(newestFirst: Solution[]): Map<string, number> {
  const numbers = new Map<string, number>()
  newestFirst.forEach((s, index) => {
    numbers.set(s.id, s.seq ?? newestFirst.length - index)
  })
  return numbers
}

/** One line of at most `max` characters (counted as characters, not UTF-16 units), or null for the first proposal. */
export function versionNote(
  feedback: string | undefined,
  max = VERSION_NOTE_MAX_LENGTH
): string | null {
  const flat = (feedback ?? '').replace(/\s+/g, ' ').trim()
  if (flat === '') {
    return null
  }
  const chars = [...flat]
  return chars.length <= max
    ? flat
    : `${chars
        .slice(0, max - 1)
        .join('')
        .trimEnd()}…`
}

export function versionBadge(solution: Solution, isCurrent: boolean): VersionBadge | null {
  switch (solution.status) {
    case 'superseded':
      return 'superseded'
    case 'rejected':
      return 'rejected'
    case 'chosen':
      return 'approved'
    default:
      return isCurrent ? 'current' : null
  }
}

export function versionPageCount(total: number): number {
  return Math.max(1, Math.ceil(total / VERSION_PAGE_SIZE))
}

export function versionPageOf(newestFirst: Solution[], id: string): number {
  const index = newestFirst.findIndex((s) => s.id === id)
  return index < 0 ? 0 : Math.floor(index / VERSION_PAGE_SIZE)
}
