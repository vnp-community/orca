/**
 * open-findings-by-symbol.ts — FE-CV-TASK-059-03
 *
 * Per-symbol roll-up of open (not dismissed) structural findings, for other lenses to draw a
 * warning icon on graph nodes. Pure; lenses feed it the result of useCodeIntelFindings.
 *
 * @module components/review-map/findings/open-findings-by-symbol
 */

import type { Finding } from '../../../../../shared/code-intel-types'
import { normalizeFindingSeverity } from './finding-view-model'
import type { FindingSeverityState } from './finding-view-model'

export type SymbolFindingSummary = { count: number; topSeverity: FindingSeverityState }

const RANK: Record<FindingSeverityState, number> = { error: 0, warning: 1, info: 2, unknown: 3 }

export function selectOpenFindingsBySymbolKey(
  findings: readonly Finding[]
): Record<string, SymbolFindingSummary> {
  const out: Record<string, SymbolFindingSummary> = {}
  for (const finding of findings) {
    if (finding.dismissed) {
      continue
    }
    const severity = normalizeFindingSeverity(finding.severity)
    const keys = new Set(
      (finding.evidence ?? []).map((e) => e.symbol?.key).filter((k): k is string => Boolean(k))
    )
    for (const key of keys) {
      const prev = out[key]
      out[key] = prev
        ? {
            count: prev.count + 1,
            topSeverity: RANK[severity] < RANK[prev.topSeverity] ? severity : prev.topSeverity
          }
        : { count: 1, topSeverity: severity }
    }
  }
  return out
}
