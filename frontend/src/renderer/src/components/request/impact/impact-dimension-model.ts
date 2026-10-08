/**
 * Impact dimension model — FE-REQ-TASK-036-05
 *
 * Pure view-model helpers for the risk card and the Option x dimension table.
 *
 * @module components/request/impact/impact-dimension-model
 */

import { GRAPH_RISK_ORDER } from '../../../../../shared/graph-types'
import type { GraphRisk } from '../../../../../shared/graph-types'
import type { ImpactComparison, ImpactSummary } from '../../../../../shared/request-artifact-types'

// Why: backend dimension key names are not final; unknown extra keys are appended after these.
export const IMPACT_DIMENSIONS = [
  'architecture',
  'contract',
  'data',
  'blast_radius',
  'security',
  'operations',
  'quality',
  'uncertainty',
  'size'
] as const

export type ComparisonCell = { level: GraphRisk; score: number | null; note?: string }
export type ComparisonRow = {
  dimension: string
  cells: Record<string, ComparisonCell | null>
  differs: boolean
}

export function buildComparisonRows(
  optionIds: readonly string[],
  comparison: readonly ImpactComparison[] | null
): ComparisonRow[] {
  if (!comparison || comparison.length === 0) {
    return []
  }
  const byOption = new Map(comparison.map((c) => [c.optionId, c]))
  const extra = new Set<string>()
  for (const c of comparison) {
    for (const k of Object.keys(c.dimensions)) {
      if (!(IMPACT_DIMENSIONS as readonly string[]).includes(k)) {
        extra.add(k)
      }
    }
  }
  return [...IMPACT_DIMENSIONS, ...[...extra].sort()].map((dimension) => {
    const cells: Record<string, ComparisonCell | null> = {}
    for (const id of optionIds) {
      cells[id] = byOption.get(id)?.dimensions[dimension] ?? null
    }
    return { dimension, cells, differs: cellsDiffer(Object.values(cells)) }
  })
}

export type Sol020Row = {
  key: 'effort' | 'rollback'
  cells: Record<string, string | null>
  differs: boolean
}

/** Effort / Rollback (SOL-020) rows under the impact dimensions; rows nobody filled are dropped. */
export function buildSol020Rows(
  options: readonly { id: string; effort?: string; rollback?: string }[]
): Sol020Row[] {
  const rows: Sol020Row[] = []
  for (const key of ['effort', 'rollback'] as const) {
    const cells: Record<string, string | null> = {}
    for (const o of options) {
      cells[o.id] = o[key]?.trim() || null
    }
    const values = Object.values(cells)
    if (values.every((v) => v === null)) {
      continue
    }
    rows.push({ key, cells, differs: new Set(values).size > 1 })
  }
  return rows
}

function cellsDiffer(cells: (ComparisonCell | null)[]): boolean {
  const seen = new Set(cells.map((c) => (c ? `${c.level}:${c.score ?? ''}` : 'none')))
  return seen.size > 1
}

export type CardSummary = {
  level: GraphRisk
  scoreText: string | null
  reasons: string[]
  caption: {
    tool: string | null
    assessedAt: string | null
    confidence: 'low' | 'medium' | 'high' | null
  }
  stale: boolean
  advisory: boolean
  hardRules: string[]
}

/** Never produces a level for a missing summary: null in, null out (UI shows "Not assessed"). */
export function summarizeCard(summary: ImpactSummary | null): CardSummary | null {
  if (!summary) {
    return null
  }
  return {
    level: summary.level,
    scoreText: summary.score !== null ? `${Math.round(summary.score)}/100` : null,
    reasons: summary.topReasons.slice(0, 3),
    caption: { tool: summary.tool, assessedAt: summary.assessedAt, confidence: summary.confidence },
    stale: summary.stale,
    advisory: summary.mode === 'shadow',
    hardRules: summary.hardRules
  }
}

export function sortByRiskDesc<T extends { level: GraphRisk }>(items: readonly T[]): T[] {
  return [...items].sort((a, b) => GRAPH_RISK_ORDER[b.level] - GRAPH_RISK_ORDER[a.level])
}
