/**
 * quality-finding-location.ts — FE-CV-TASK-087-12
 *
 * `file:line[-endLine]` text of a finding; file-level findings (line <= 0) show the path only.
 *
 * @module components/review-map/quality/findings/quality-finding-location
 */

import type { QualityFinding } from '../../../../../../shared/code-intel-quality-types'

export function formatQualityFindingLocation(
  finding: Pick<QualityFinding, 'file' | 'line' | 'endLine'>
): string {
  if (finding.line <= 0) {
    return finding.file
  }
  const range = finding.endLine > finding.line ? `-${finding.endLine}` : ''
  return `${finding.file}:${finding.line}${range}`
}
