/**
 * review-chip-filter.ts — FE-CV-TASK-051-04
 *
 * One summary chip = one filter. files/symbols/untested/violations filter by key set;
 * flows/tables/contracts only switch lens (no filter) because their items are not file-keyed.
 */

import type { ChangeOverlayView } from './review-wire-types'

export const REVIEW_CHIP_IDS = [
  'files',
  'symbols',
  'flows',
  'tables',
  'contracts',
  'untested',
  'violations'
] as const

export type ReviewChipId = (typeof REVIEW_CHIP_IDS)[number]

const FILTERING_CHIPS: ReadonlySet<ReviewChipId> = new Set([
  'files',
  'symbols',
  'untested',
  'violations'
])

/** Chips that navigate to a lens instead of filtering. Lens ids are registry ids. */
export const REVIEW_CHIP_TARGET_LENS: Partial<Record<ReviewChipId, string>> = {
  flows: 'dataflow',
  tables: 'erd',
  contracts: 'contract'
}

export function isFilteringChip(chip: ReviewChipId): boolean {
  return FILTERING_CHIPS.has(chip)
}

const CHIP_COUNT_KEY: Record<ReviewChipId, string> = {
  files: 'changedFiles',
  symbols: 'changedSymbols',
  flows: 'affectedFlows',
  tables: 'touchedTables',
  contracts: 'touchedContracts',
  untested: 'uncoveredSymbols',
  violations: 'violations'
}

const CHIP_ARRAY: Record<ReviewChipId, (o: ChangeOverlayView) => unknown[]> = {
  files: (o) => o.changedFiles,
  symbols: (o) => o.changedSymbols,
  flows: (o) => o.affectedFlows,
  tables: (o) => o.touchedTables,
  contracts: (o) => o.touchedContracts,
  untested: (o) => o.uncoveredSymbols,
  violations: (o) => o.violations
}

/** Count shown on a chip: the full total when the array was truncated, else its length. */
export function reviewChipCount(overlay: ChangeOverlayView, chip: ReviewChipId): number {
  const key = CHIP_COUNT_KEY[chip]
  const total = overlay.limits.totalCounts[key]
  const length = CHIP_ARRAY[chip](overlay).length
  const truncatedFlag = overlay.limits.truncated[chipTruncationKey(chip)]
  if (typeof total === 'number' && (truncatedFlag || total > length)) {
    return total
  }
  return length
}

function chipTruncationKey(chip: ReviewChipId): string {
  switch (chip) {
    case 'files':
      return 'files'
    case 'symbols':
    case 'untested':
      return 'symbols'
    case 'flows':
      return 'flows'
    default:
      return chip
  }
}

export type ReviewChipPredicate = {
  /** File paths kept by the filter. */
  files: ReadonlySet<string>
  /** Symbol keys kept, when the chip is symbol-keyed (null = not symbol-keyed). */
  symbolKeys: ReadonlySet<string> | null
}

/** Pure; returns null for navigation-only chips. */
export function reviewChipPredicate(
  overlay: ChangeOverlayView,
  chip: ReviewChipId
): ReviewChipPredicate | null {
  switch (chip) {
    case 'files':
      return { files: new Set(overlay.changedFiles.map((f) => f.path)), symbolKeys: null }
    case 'symbols':
      return {
        files: new Set(overlay.changedSymbols.map((s) => s.symbol.filePath)),
        symbolKeys: new Set(overlay.changedSymbols.map((s) => s.symbol.key))
      }
    case 'untested':
      return {
        files: new Set(overlay.uncoveredSymbols.map((s) => s.filePath)),
        symbolKeys: new Set(overlay.uncoveredSymbols.map((s) => s.key))
      }
    case 'violations':
      return { files: new Set(overlay.violations.map((v) => v.file)), symbolKeys: null }
    default:
      return null
  }
}
