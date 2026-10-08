import { parseReviewReportModel } from './review-report-model-parser'
import type { ReviewReportModel } from './review-report-model-parser'

/** Wire-shaped payload (CONTRACT ui-api 4.7) used by the report tests. */
export function reviewReportWire(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    schemaVersion: 1,
    subject: {
      branch: 'feature/login',
      baseRef: 'main',
      headCommit: 'abcdef1234567890',
      baseCommit: '1111111222222',
      indexCommit: 'abcdef1234567890',
      indexStale: false,
      provider: 'github',
      turnKey: null
    },
    reproducibility: { profileRef: 'go-test@3', toolVersions: { go: '1.23' }, runIds: ['r1'], modelDigest: 'deadbeefcafe0123456789' },
    summary: { files: 12, added: 340, removed: 21, symbols: 30, flows: 2, components: ['api-gateway', 'review-service'] },
    risk: { level: 'MEDIUM', reasons: [{ code: 'touches_auth', params: {} }] },
    gate: {
      verdict: 'fail',
      reasons: [{ check: 'lint', result: 'fail', code: 'errors_over_limit', params: { count: '3' } }],
      waivers: { count: 1, earliestExpiry: '2026-11-01T00:00:00Z' }
    },
    findings: {
      counts: { error: 3, warning: 2, info: 1, byCategory: { lint: 4 } },
      top: [{ ruleId: 'no-unused', severity: 'error', category: 'lint', file: 'src/a.ts', line: 10, message: 'unused var', origin: 'quality' }]
    },
    contracts: {
      protoRpc: [{ name: 'GetThing', change: 'altered', breaking: true }],
      wsChannels: [{ name: 'codeIntel.status', change: 'added', breaking: false }],
      tables: [{ table: 'agent_turns', service: 'code-intel', op: 'added', breaking: false }]
    },
    readingOrder: [{ n: 1, file: 'src/a.ts', reason: 'entry point', symbols: ['A'] }],
    diagrams: [{ kind: 'components', mermaid: 'flowchart TD\n  A --> B', alt: ['A calls B'], truncated: false }],
    limits: { truncated: { findings: false, readingOrder: false, diagrams: false }, totalCounts: { findings: 6 } },
    warnings: ['index_stale'],
    ...overrides
  }
}

export function reviewReportFixture(overrides: Record<string, unknown> = {}): ReviewReportModel {
  return parseReviewReportModel(reviewReportWire(overrides))
}
