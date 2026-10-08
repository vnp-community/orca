/**
 * review-summary-model.ts — FE-CV-TASK-061-06
 *
 * Pure view model of the right-sidebar Review summary. Count keys follow the proposed
 * `totalCounts` names (open question 2); a missing key hides its chip instead of showing 0.
 */

import type { ChangeOverlayView, ReadingProgress, RiskAssessmentView } from '../review-wire-types'

export type ReviewSummaryCountId = 'files' | 'symbols' | 'flows' | 'tables' | 'contracts' | 'uncovered'

export type ReviewSummaryModel = {
  baseRef: string | null
  risk: RiskAssessmentView | null
  freshnessState: string | null
  counts: { id: ReviewSummaryCountId; value: number }[]
  findings: { error: number; warning: number; info: number }
  /** null when there is no progress data or no file total. */
  progress: { seen: number; total: number } | null
}

const COUNT_IDS: ReviewSummaryCountId[] = [
  'files',
  'symbols',
  'flows',
  'tables',
  'contracts',
  'uncovered'
]

const FALLBACK_LENGTH: Partial<Record<ReviewSummaryCountId, keyof ChangeOverlayView>> = {
  files: 'changedFiles',
  symbols: 'changedSymbols',
  flows: 'affectedFlows',
  tables: 'touchedTables',
  contracts: 'touchedContracts',
  uncovered: 'uncoveredSymbols'
}

export function buildReviewSummaryModel(
  overlay: ChangeOverlayView,
  progress: ReadingProgress | null
): ReviewSummaryModel {
  const totals = overlay.limits?.totalCounts ?? {}
  const counts: ReviewSummaryModel['counts'] = []
  for (const id of COUNT_IDS) {
    const total = totals[id]
    if (typeof total === 'number') {
      counts.push({ id, value: total })
      continue
    }
    const key = FALLBACK_LENGTH[id]
    const list = key ? (overlay[key] as unknown) : null
    // Why: 'summary' detail returns empty arrays, so an empty fallback is "unknown", not 0.
    if (Array.isArray(list) && list.length > 0) {
      counts.push({ id, value: list.length })
    }
  }

  const findings = { error: 0, warning: 0, info: 0 }
  for (const v of overlay.violations ?? []) {
    if (v.severity === 'error' || v.severity === 'warning' || v.severity === 'info') {
      findings[v.severity] += 1
    }
  }

  const fileTotal = counts.find((c) => c.id === 'files')?.value ?? 0
  const seen = progress
    ? Object.values(progress.entries).filter((entry) => entry.state === 'seen').length
    : 0
  return {
    baseRef: overlay.scope?.baseRef ?? null,
    risk: overlay.risk,
    freshnessState: overlay.indexFreshness?.state ?? null,
    counts,
    findings,
    progress: progress && fileTotal > 0 ? { seen: Math.min(seen, fileTotal), total: fileTotal } : null
  }
}
