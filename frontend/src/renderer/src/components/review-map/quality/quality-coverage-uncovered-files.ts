/**
 * quality-coverage-uncovered-files.ts — FE-CV-TASK-087-16
 *
 * Files with uncovered line ranges, most uncovered lines first, for the "Lines not covered" list.
 *
 * @module components/review-map/quality/quality-coverage-uncovered-files
 */

import type { CoverageFile } from '../../../../../shared/code-intel-quality-visualization-types'

export const UNCOVERED_FILES_LIMIT = 20

export type UncoveredFile = { path: string; lines: number; ranges: [number, number][] }

export function listUncoveredFiles(
  files: readonly CoverageFile[],
  limit: number = UNCOVERED_FILES_LIMIT
): { shown: UncoveredFile[]; total: number } {
  const withRanges: UncoveredFile[] = []
  for (const file of files) {
    const ranges = (file.uncoveredRanges ?? []).filter(
      ([from, to]) => Number.isInteger(from) && Number.isInteger(to) && from >= 1 && to >= from
    )
    if (ranges.length > 0) {
      const lines = ranges.reduce((sum, [from, to]) => sum + (to - from + 1), 0)
      withRanges.push({ path: file.path, lines, ranges })
    }
  }
  withRanges.sort((a, b) => b.lines - a.lines || (a.path < b.path ? -1 : 1))
  return { shown: withRanges.slice(0, limit), total: withRanges.length }
}
