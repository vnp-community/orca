/**
 * Parsers of the test-spec read models. Tolerant like every request parser: a missing field gets a neutral value and an
 * unknown status is kept as 'unknown'; nothing throws.
 *
 * @module shared/request-testspec-parsers
 */

import type {
  TestSpecDocumentPage,
  TestSpecGap,
  TestSpecRun,
  TestSpecs,
  TestSpecStatus,
  TestSpecSummary
} from './request-testspec-types'

const STATUSES = new Set<TestSpecStatus>([
  'pending',
  'starting',
  'running',
  'completed',
  'blocked',
  'failed',
  'skipped',
  'cancelled'
])

function obj(raw: unknown): Record<string, unknown> {
  return raw && typeof raw === 'object' && !Array.isArray(raw)
    ? (raw as Record<string, unknown>)
    : {}
}
function str(v: unknown): string {
  return typeof v === 'string' ? v : ''
}
function opt(v: unknown): string | undefined {
  return typeof v === 'string' && v !== '' ? v : undefined
}
function num(v: unknown): number {
  return typeof v === 'number' && Number.isFinite(v) ? v : 0
}
function strings(v: unknown): string[] {
  return Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : []
}

function parseGap(raw: unknown): TestSpecGap {
  const g = obj(raw)
  return {
    code: str(g.code),
    missing: str(g.missing),
    question: str(g.question),
    owner: str(g.owner),
    blocks: num(g.blocks)
  }
}

export function parseTestSpecSummary(raw: unknown): TestSpecSummary {
  const s = obj(raw)
  const counts = obj(s.counts)
  return {
    feature: str(s.feature),
    version: num(s.version),
    status: str(s.status),
    counts: {
      requirements: num(counts.TestRequirement),
      cases: num(counts.TestCase),
      gaps: num(counts.TestGap)
    },
    files: strings(s.files),
    gaps: Array.isArray(s.gaps) ? s.gaps.map(parseGap) : [],
    notes: strings(s.notes)
  }
}

export function parseTestSpecRun(raw: unknown): TestSpecRun {
  const r = obj(raw)
  const status = str(r.status) as TestSpecStatus
  return {
    id: str(r.id),
    attempt: num(r.attempt),
    status: STATUSES.has(status) ? status : 'unknown',
    reasonCode: opt(r.reasonCode),
    reasonMessage: opt(r.reasonMessage),
    contextMode: opt(r.contextMode),
    contextRef: opt(r.contextRef),
    tddRunId: opt(r.tddRunId),
    feature: opt(r.feature),
    summary: parseTestSpecSummary(r.summary),
    startedBy: opt(r.startedBy),
    startedAt: opt(r.startedAt),
    finishedAt: opt(r.finishedAt),
    updatedAt: opt(r.updatedAt)
  }
}

export function parseTestSpecs(raw: unknown): TestSpecs {
  const r = obj(raw)
  return {
    requestId: str(r.requestId),
    enabled: r.enabled === true,
    current: r.current ? parseTestSpecRun(r.current) : null,
    history: Array.isArray(r.history) ? r.history.map(parseTestSpecRun) : []
  }
}

export function parseTestSpecDocument(raw: unknown): TestSpecDocumentPage {
  const d = obj(raw)
  return {
    document: str(d.document),
    version: num(d.version),
    status: str(d.status),
    length: num(d.length),
    text: str(d.text),
    hasNext: d.hasNext === true,
    next: num(d.next)
  }
}
