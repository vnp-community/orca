/**
 * reading-order-model.ts — FE-CV-TASK-052-01
 *
 * Pure model for the reading order lens.
 * Builds and filters reading order rows from ChangeOverlay + ComponentGroup data.
 *
 * @module components/review-map/reading-order-model
 */

import type { ChangeOverlay } from '../../../../shared/code-intel-types'

// ---------------------------------------------------------------------------
// Row types
// ---------------------------------------------------------------------------

export type ReadingOrderRowStatus = 'unread' | 'reading' | 'read' | 'skipped'

export type ReadingOrderRow = {
  id: string
  path: string
  changeType: string
  /** Recommended reading position (1-indexed) */
  position: number
  status: ReadingOrderRowStatus
  /** Optional component group name */
  groupName: string | null
  /** Size estimate in lines */
  estimatedLines: number | null
}

export type ReadingOrderGroup = {
  id: string
  name: string
  description: string | null
  rows: ReadingOrderRow[]
}

// ---------------------------------------------------------------------------
// Progress serialization
// ---------------------------------------------------------------------------

export type ReadingProgressEntry = {
  path: string
  status: ReadingOrderRowStatus
  updatedAt: string
}

export type SerializedReadingProgress = {
  entries: ReadingProgressEntry[]
  version: number
}

// ---------------------------------------------------------------------------
// Builder
// ---------------------------------------------------------------------------

/**
 * Build reading order rows from ChangeOverlay.
 * Position is the overlay index + 1.
 * Status is 'unread' by default; caller should apply progress.
 */
export function buildReadingOrderRows(
  overlay: ChangeOverlay[],
  progress: ReadingProgressEntry[]
): ReadingOrderRow[] {
  const progressMap = new Map<string, ReadingProgressEntry>()
  for (const entry of progress) {
    progressMap.set(entry.path, entry)
  }

  return overlay
    .filter((o) => o.path && o.path !== '...')
    .map((o, i) => {
      const progressEntry = progressMap.get(o.path)
      return {
        id: o.path,
        path: o.path,
        changeType: o.changeType,
        position: i + 1,
        status: (progressEntry?.status ?? 'unread') as ReadingOrderRowStatus,
        groupName: null,
        estimatedLines: null
      }
    })
}

/**
 * Calculate reading completion percentage.
 */
export function getReadingCompletion(rows: ReadingOrderRow[]): number {
  if (!rows.length) return 0
  const done = rows.filter((r) => r.status === 'read' || r.status === 'skipped').length
  return Math.round((done / rows.length) * 100)
}
