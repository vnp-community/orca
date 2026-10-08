/**
 * quality-trend-series.ts — FE-CV-TASK-087-15
 *
 * QualityTrendPoint[] -> series for TrendLineChart. A metric key that is absent stays null
 * (the line breaks); it is never turned into 0 (PQ-33).
 *
 * @module components/review-map/quality/quality-trend-series
 */

import type { QualityTrendPoint } from '../../../../../shared/code-intel-quality-visualization-types'
import type { TrendSeries } from '../../quality-charts/TrendLineChart'
import type { QualityVerdictLevel } from '../../quality-charts/severity-encoding'
import { toPercent } from './coverage-percent-normalization'

export const TREND_MAX_POINTS = 50

export type TrendSourceKind = 'local' | 'ci' | 'unknown'

export type QualityTrendModel = {
  points: QualityTrendPoint[]
  series: TrendSeries[]
  xLabels: string[]
  /** Diff coverage per point as percent 0..100, null where the point has no number. */
  diffCoverageSpark: (number | null)[]
  verdicts: QualityVerdictLevel[]
  sources: TrendSourceKind[]
  verdictChanges: number
  /** Points before the TREND_MAX_POINTS cut that were dropped on this side. */
  droppedOlder: number
}

function countOf(value: number): number | null {
  return Number.isFinite(value) ? value : null
}

function verdictLevel(raw: string): QualityVerdictLevel {
  return raw === 'pass' || raw === 'warn' || raw === 'fail' ? raw : 'unknown'
}

export function sourceKind(raw: string): TrendSourceKind {
  return raw === 'local' || raw === 'ci' ? raw : 'unknown'
}

export function shortCommit(commit: string): string {
  return commit.slice(0, 7)
}

function pointLabel(
  point: QualityTrendPoint,
  turnLabels: ReadonlyMap<string, string> | undefined
): string {
  const matched = point.turnKey ? turnLabels?.get(point.turnKey) : undefined
  if (matched) {
    return matched
  }
  // Why: UTC keeps labels identical across machines and test runs; the format is "MM-DD HH:mm".
  const time = point.createdAt.length >= 16 ? point.createdAt.slice(5, 16).replace('T', ' ') : ''
  const commit = shortCommit(point.headCommit)
  return [commit, time].filter(Boolean).join(' ')
}

export function buildQualityTrendModel(
  input: readonly QualityTrendPoint[],
  options: { turnLabels?: ReadonlyMap<string, string>; maxPoints?: number } = {}
): QualityTrendModel {
  const max = options.maxPoints ?? TREND_MAX_POINTS
  // Why: the backend order is not part of the contract; the chart needs oldest first.
  const ordered = input
    .map((point, index) => ({ point, index }))
    .sort((a, b) =>
      a.point.createdAt < b.point.createdAt
        ? -1
        : a.point.createdAt > b.point.createdAt
          ? 1
          : a.index - b.index
    )
    .map((entry) => entry.point)
  const droppedOlder = Math.max(0, ordered.length - max)
  const points = ordered.slice(droppedOlder)
  const verdicts = points.map((p) => verdictLevel(p.verdict))
  const changed = verdicts.map((v, i) => i > 0 && v !== verdicts[i - 1])
  const severity = (id: 'error' | 'warning' | 'info'): TrendSeries => ({
    id,
    label: id,
    points: points.map((p, i) => ({
      label: pointLabel(p, options.turnLabels),
      value: countOf(p.counts[id]),
      // Why: one marker per change is enough; the chart merges markers across series by index.
      ...(id === 'error' && changed[i] ? { marker: verdicts[i] } : {})
    }))
  })
  return {
    points,
    series: [severity('error'), severity('warning'), severity('info')],
    xLabels: points.map((p) => pointLabel(p, options.turnLabels)),
    diffCoverageSpark: points.map((p) => toPercent(p.metrics.diffCoverage)),
    verdicts,
    sources: points.map((p) => sourceKind(p.source)),
    verdictChanges: changed.filter(Boolean).length,
    droppedOlder
  }
}
