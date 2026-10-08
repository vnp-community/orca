/**
 * quality-finding-sort.ts — FE-CV-TASK-087-12
 *
 * Stable ordering of check findings: severity (worst first), category, file, line, fingerprint.
 * The fingerprint tie-break keeps the order identical across reloads and pages.
 *
 * @module components/review-map/quality/findings/quality-finding-sort
 */

import type { QualityFinding } from '../../../../../../shared/code-intel-quality-types'

const SEVERITY_ORDER: Record<string, number> = { error: 0, warning: 1, info: 2 }

function severityRank(severity: string): number {
  return SEVERITY_ORDER[severity] ?? 3
}

function compareText(a: string, b: string): number {
  return a < b ? -1 : a > b ? 1 : 0
}

export function compareQualityFindings(a: QualityFinding, b: QualityFinding): number {
  return (
    severityRank(a.severity) - severityRank(b.severity) ||
    compareText(a.category, b.category) ||
    compareText(a.file, b.file) ||
    a.line - b.line ||
    compareText(a.fingerprint, b.fingerprint)
  )
}

export function sortQualityFindings(findings: readonly QualityFinding[]): QualityFinding[] {
  return [...findings].sort(compareQualityFindings)
}
