/**
 * quality-hotspot-rows.ts — FE-CV-TASK-087-15
 *
 * Findings of rule 'hotspot.file' -> heatmap rows. Columns are the union of the metric keys that
 * actually arrived (contract lists none); nothing is named in advance and no total is invented.
 *
 * @module components/review-map/quality/quality-hotspot-rows
 */

import type { Finding } from '../../../../../shared/code-intel-findings-types'
import type { HeatmapColumn, HeatmapRow } from '../../quality-charts/HotspotHeatmap'

export const HOTSPOT_RULE = 'hotspot.file'
export const HOTSPOT_MAX_COLUMNS = 6

export type HotspotModel = {
  rows: HeatmapRow[]
  /** At most HOTSPOT_MAX_COLUMNS, most frequent first. */
  columns: HeatmapColumn[]
  /** Every metric key seen, same order; the alternative table lists all of them. */
  allKeys: string[]
  owners: Record<string, string>
  /** Every metric value per row id (null when the key is missing or not finite). */
  allMetrics: Record<string, Record<string, number | null>>
}

function finiteMetric(finding: Finding, key: string): number | null {
  const value = finding.metrics?.[key]
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}

function ownerText(finding: Finding): string | null {
  const names = finding.owner?.names
  return Array.isArray(names) && names.length > 0 ? names.join(', ') : null
}

export function buildHotspotModel(
  findings: readonly Finding[],
  labelOf: (key: string) => string = (key) => key
): HotspotModel {
  const unique = new Map<string, Finding>()
  for (const finding of findings) {
    if (finding.rule !== HOTSPOT_RULE) {
      continue
    }
    const path = finding.evidence?.[0]?.path ?? finding.subject
    if (path && !unique.has(path)) {
      unique.set(path, finding)
    }
  }
  const frequency = new Map<string, number>()
  for (const finding of unique.values()) {
    for (const key of Object.keys(finding.metrics ?? {})) {
      if (finiteMetric(finding, key) !== null) {
        frequency.set(key, (frequency.get(key) ?? 0) + 1)
      }
    }
  }
  const allKeys = [...frequency.keys()].sort(
    (a, b) => frequency.get(b)! - frequency.get(a)! || (a < b ? -1 : 1)
  )
  const columnKeys = allKeys.slice(0, HOTSPOT_MAX_COLUMNS)
  const rows: HeatmapRow[] = [...unique.entries()].map(([path, finding]) => ({
    id: path,
    label: path,
    values: columnKeys.map((key) => finiteMetric(finding, key))
  }))
  // Why: order by the most common metric only to be stable; it is not a score.
  rows.sort((a, b) => {
    const av = a.values[0]
    const bv = b.values[0]
    if (av === bv) {
      return a.id < b.id ? -1 : 1
    }
    if (av === null || av === undefined) {
      return 1
    }
    if (bv === null || bv === undefined) {
      return -1
    }
    return bv - av
  })
  const owners: Record<string, string> = {}
  const allMetrics: HotspotModel['allMetrics'] = {}
  for (const [path, finding] of unique) {
    const owner = ownerText(finding)
    if (owner) {
      owners[path] = owner
    }
    allMetrics[path] = Object.fromEntries(allKeys.map((key) => [key, finiteMetric(finding, key)]))
  }
  return {
    rows,
    columns: columnKeys.map((key) => ({ key, label: labelOf(key) })),
    allKeys,
    owners,
    allMetrics
  }
}
