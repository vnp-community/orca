/**
 * Parsers of the test-script read models. Tolerant like every request parser: a missing field gets a neutral value and an unknown
 * status is kept as 'unknown'; nothing throws.
 *
 * @module shared/request-testscript-parsers
 */

import type {
  TestScriptFile,
  TestScriptManifest,
  TestScriptManifestRow,
  TestScriptReportPage,
  TestScriptRun,
  TestScripts,
  TestScriptStatus,
  TestScriptSummary
} from './request-testscript-types'

const STATUSES = new Set<TestScriptStatus>([
  'pending',
  'running',
  'completed',
  'needs_review',
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

function parseFile(raw: unknown): TestScriptFile {
  const f = obj(raw)
  return {
    path: str(f.path),
    status: str(f.status),
    attempts: num(f.attempts),
    cases: strings(f.cases),
    errors: num(f.errors)
  }
}

/** Reads the job as tsa_get reported it (snake_case keys, the agent's "needs-review" spelling). */
export function parseTestScriptSummary(raw: unknown): TestScriptSummary {
  const s = obj(raw)
  const counts = obj(s.counts)
  return {
    counts: {
      files: num(counts.files),
      generated: num(counts.generated),
      needsReview: num(counts['needs-review']),
      failed: num(counts.failed),
      skipped: num(counts.skipped)
    },
    files: Array.isArray(s.files) ? s.files.map(parseFile) : [],
    report: str(s.report),
    manifest: str(s.manifest),
    generator: str(s.generator),
    costUsd: num(s.cost_usd),
    seconds: num(s.seconds)
  }
}

export function parseTestScriptRun(raw: unknown): TestScriptRun {
  const r = obj(raw)
  const status = str(r.status) as TestScriptStatus
  return {
    id: str(r.id),
    attempt: num(r.attempt),
    status: STATUSES.has(status) ? status : 'unknown',
    reasonCode: opt(r.reasonCode),
    reasonMessage: opt(r.reasonMessage),
    feature: opt(r.feature),
    specVersion: num(r.specVersion),
    tsaJobId: opt(r.tsaJobId),
    branch: opt(r.branch),
    baseRef: opt(r.baseRef),
    worktreePath: opt(r.worktreePath),
    commitSha: opt(r.commitSha),
    summary: parseTestScriptSummary(r.summary),
    startedAt: opt(r.startedAt),
    finishedAt: opt(r.finishedAt),
    updatedAt: opt(r.updatedAt)
  }
}

export function parseTestScripts(raw: unknown): TestScripts {
  const r = obj(raw)
  return {
    requestId: str(r.requestId),
    enabled: r.enabled === true,
    current: r.current ? parseTestScriptRun(r.current) : null,
    history: Array.isArray(r.history) ? r.history.map(parseTestScriptRun) : []
  }
}

export function parseTestScriptReport(raw: unknown): TestScriptReportPage {
  const d = obj(raw)
  return {
    path: str(d.path),
    length: num(d.length),
    text: str(d.text),
    hasNext: d.hasNext === true,
    next: num(d.next)
  }
}

export const parseTestScriptFilePage = parseTestScriptReport

function parseManifestRow(raw: unknown): TestScriptManifestRow {
  const r = obj(raw)
  const line = num(r.line)
  return {
    exp: str(r.exp),
    case: str(r.case),
    kind: str(r.kind),
    op: str(r.op),
    value: typeof r.value === 'string' ? r.value : r.value == null ? '' : JSON.stringify(r.value),
    status: str(r.status),
    reason: opt(r.reason),
    file: opt(r.file),
    line: line > 0 ? line : undefined,
    fn: opt(r.function),
    assertion: opt(r.assertion)
  }
}

/** Reads assertion-manifest.json (tsa-manifest/v1) from its text; invalid JSON gives an empty manifest, never a throw. */
export function parseTestScriptManifest(text: string): TestScriptManifest {
  let raw: unknown = null
  try {
    raw = JSON.parse(text)
  } catch {
    raw = null
  }
  const m = obj(raw)
  return {
    feature: str(m.feature),
    specVersion: num(m.specVersion),
    branch: str(m.branch),
    rows: Array.isArray(m.assertions) ? m.assertions.map(parseManifestRow) : []
  }
}
