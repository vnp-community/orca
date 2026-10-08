/**
 * quality-finding-filter.ts — FE-CV-TASK-087-12
 *
 * Client-side filters applied after the server filters (severity, category, scope, file):
 * free-text search and the "show waived" switch. Plain case-insensitive substring matching.
 *
 * @module components/review-map/quality/findings/quality-finding-filter
 */

import type { QualityFinding } from '../../../../../../shared/code-intel-quality-types'

export type QualityFindingLocalFilter = { search: string; showWaived: boolean }

export function matchesQualityFindingSearch(finding: QualityFinding, needle: string): boolean {
  const q = needle.trim().toLowerCase()
  if (q === '') {
    return true
  }
  return (
    finding.message.toLowerCase().includes(q) ||
    finding.ruleId.toLowerCase().includes(q) ||
    finding.file.toLowerCase().includes(q)
  )
}

export function filterQualityFindings(
  findings: readonly QualityFinding[],
  filter: QualityFindingLocalFilter
): QualityFinding[] {
  return findings.filter(
    (f) => (filter.showWaived || !f.waiver) && matchesQualityFindingSearch(f, filter.search)
  )
}

export function countWaivedFindings(findings: readonly QualityFinding[]): number {
  return findings.reduce((n, f) => (f.waiver ? n + 1 : n), 0)
}
