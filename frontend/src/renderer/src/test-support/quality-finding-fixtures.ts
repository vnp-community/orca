import type { QualityFinding } from '../../../shared/code-intel-quality-types'

export function makeQualityFinding(patch: Partial<QualityFinding> = {}): QualityFinding {
  return {
    fingerprint: 'fp-1',
    fpVersion: 1,
    ruleId: 'no-unused-vars',
    severity: 'error',
    category: 'lint',
    file: 'src/a.ts',
    line: 3,
    endLine: 3,
    column: 0,
    endColumn: 0,
    message: 'x is unused',
    tool: 'oxlint',
    toolVersion: '1.0.0',
    stepId: 's1',
    inScope: true,
    ...patch
  }
}

export function makeQualityFindings(count: number, prefix = 'fp'): QualityFinding[] {
  return Array.from({ length: count }, (_, i) =>
    makeQualityFinding({
      fingerprint: `${prefix}-${String(i).padStart(5, '0')}`,
      line: i + 1,
      endLine: i + 1
    })
  )
}
