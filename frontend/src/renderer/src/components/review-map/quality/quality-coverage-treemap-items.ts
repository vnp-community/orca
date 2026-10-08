/**
 * quality-coverage-treemap-items.ts — FE-CV-TASK-087-15
 *
 * CoverageReport.files -> MetricTreemap items: size = statements, shade = share not covered (%).
 * Files without statements cannot be drawn; they are counted, not hidden.
 *
 * @module components/review-map/quality/quality-coverage-treemap-items
 */

import type { CoverageFile } from '../../../../../shared/code-intel-quality-visualization-types'
import type { TreemapItem } from '../../quality-charts/MetricTreemap'

export const COVERAGE_TREEMAP_MAX_TILES = 400

export type CoverageTreemapModel = {
  items: TreemapItem[]
  /** Files with stmts <= 0 or non-finite numbers. */
  withoutStatements: number
  /** Drawable files cut by the tile cap (smallest first). */
  hidden: number
  /** Drawable files before the cap. */
  drawable: number
}

export function uncoveredPercent(file: Pick<CoverageFile, 'stmts' | 'covered'>): number {
  const share = 1 - Math.min(Math.max(file.covered, 0), file.stmts) / file.stmts
  return Math.round(share * 1000) / 10
}

export function buildCoverageTreemapItems(
  files: readonly CoverageFile[],
  maxTiles: number = COVERAGE_TREEMAP_MAX_TILES
): CoverageTreemapModel {
  const valid = files.filter(
    (f) => Number.isFinite(f.stmts) && f.stmts > 0 && Number.isFinite(f.covered)
  )
  const sorted = [...valid].sort((a, b) => b.stmts - a.stmts || (a.path < b.path ? -1 : 1))
  const items = sorted.slice(0, maxTiles).map((f) => ({
    id: f.path,
    label: f.path,
    size: f.stmts,
    intensity: uncoveredPercent(f)
  }))
  return {
    items,
    withoutStatements: files.length - valid.length,
    hidden: sorted.length - items.length,
    drawable: sorted.length
  }
}
