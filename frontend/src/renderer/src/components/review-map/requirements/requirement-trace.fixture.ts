import { parseRequirementTrace } from './requirement-trace-view-model'
import type { RequirementTrace } from './requirement-trace-view-model'

type EvidenceWire = { kind: string; ref: string; label: string; confidence: string; matchedBy?: string }

export function evidenceWire(over: Partial<EvidenceWire> = {}): EvidenceWire {
  return { kind: 'change', ref: 'src/a.ts', label: 'a.ts', confidence: 'explicit', matchedBy: 'satisfies', ...over }
}

export function requirementWire(over: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    key: 'task:1#a',
    text: 'Filter by week',
    origin: 'structured',
    verifyHint: 'test',
    retired: false,
    state: 'has_evidence',
    evidence: [evidenceWire()],
    ...over
  }
}

export function traceWire(over: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    subject: { source: 'task', taskId: 't1', taskNumber: 42, worktreeRef: 'wt' },
    linkConfidence: 'explicit',
    requirements: [requirementWire()],
    unlinkedChanges: [{ file: 'src/other.ts', symbols: ['x'], reason: 'no_match' }],
    summary: { total: 1, hasEvidence: 1, partial: 0, noEvidence: 0, unknown: 0 },
    warnings: [],
    ...over
  }
}

export function traceFixture(over: Record<string, unknown> = {}): RequirementTrace {
  return parseRequirementTrace(traceWire(over))
}
