/**
 * code-intel-quality-wire-parsers.ts — FE-CV-TASK-087-01
 *
 * Safe parsers for §4.7 quality wire data.
 * Rules:
 * - Never throw (all paths have safe defaults)
 * - Unknown enum values → 'unknown' (U4)
 * - Negative/NaN numbers → 0 (except percent which stays null)
 * - Missing arrays → []
 * - Missing strings → ''
 *
 * @module shared/code-intel-quality-wire-parsers
 */

import type {
  GateResult, QualityGate, QualityGateReason,
  QualityRun, QualityRunStatus, QualityStep,
  QualityFinding, QualityFindingSeverity, QualityFindingCategory, QualityWaiver,
  RunnableProfile, QualityProfile,
  QualityTrendPoint,
  CoverageReport, CoverageFile, CoverageLine,
  CiComparison
} from './code-intel-quality-types'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function str(v: unknown, fallback = ''): string {
  return typeof v === 'string' ? v : fallback
}

function safeNum(v: unknown, fallback = 0): number {
  if (typeof v !== 'number' || isNaN(v)) return fallback
  return v < 0 ? 0 : v
}

function nullableNum(v: unknown): number | null {
  if (v === null || v === undefined) return null
  if (typeof v !== 'number' || isNaN(v)) return null
  return v
}

function boolVal(v: unknown): boolean {
  return v === true
}

function nullableStr(v: unknown): string | null {
  return typeof v === 'string' ? v : null
}

function arr(v: unknown): unknown[] {
  return Array.isArray(v) ? v : []
}

function obj(v: unknown): Record<string, unknown> {
  return typeof v === 'object' && v !== null && !Array.isArray(v)
    ? (v as Record<string, unknown>)
    : {}
}

// ---------------------------------------------------------------------------
// Enum guards
// ---------------------------------------------------------------------------

const GATE_RESULTS = new Set(['pass', 'warn', 'fail'])
const RUN_STATUSES = new Set(['queued', 'running', 'completed', 'failed', 'cancelled', 'interrupted'])
const FINDING_SEVERITIES = new Set(['critical', 'high', 'medium', 'low', 'info'])
const FINDING_CATEGORIES = new Set(['security', 'reliability', 'maintainability', 'coverage', 'duplication', 'other'])
const CI_VERDICTS = new Set(['better', 'same', 'worse'])
const GATE_MODES = new Set(['block', 'warn'])
const TRIGGERED_BY = new Set(['user', 'auto', 'ci'])

function gateResult(v: unknown): GateResult {
  return GATE_RESULTS.has(v as string) ? (v as GateResult) : 'unknown'
}

function runStatus(v: unknown): QualityRunStatus {
  return RUN_STATUSES.has(v as string) ? (v as QualityRunStatus) : 'unknown'
}

function findingSeverity(v: unknown): QualityFindingSeverity {
  return FINDING_SEVERITIES.has(v as string) ? (v as QualityFindingSeverity) : 'unknown'
}

function findingCategory(v: unknown): QualityFindingCategory {
  return FINDING_CATEGORIES.has(v as string) ? (v as QualityFindingCategory) : 'unknown'
}

// ---------------------------------------------------------------------------
// §4.7.1 QualityGate
// ---------------------------------------------------------------------------

function parseQualityGateReason(raw: unknown): QualityGateReason {
  const r = obj(raw)
  return {
    check: str(r.check),
    observed: safeNum(r.observed),
    threshold: safeNum(r.threshold),
    result: gateResult(r.result),
    code: typeof r.code === 'string' ? r.code : undefined,
    params: typeof r.params === 'object' && r.params !== null ? r.params as Record<string, unknown> : undefined
  }
}

export function parseQualityGate(raw: unknown): QualityGate {
  const r = obj(raw)
  return {
    result: gateResult(r.result),
    reasons: arr(r.reasons).map(parseQualityGateReason),
    mode: GATE_MODES.has(r.mode as string) ? (r.mode as 'block' | 'warn') : 'unknown',
    stale: boolVal(r.stale),
    unavailable: boolVal(r.unavailable),
    runId: nullableStr(r.runId)
  }
}

// ---------------------------------------------------------------------------
// §4.7.2 QualityRun
// ---------------------------------------------------------------------------

function parseQualityStep(raw: unknown): QualityStep {
  const r = obj(raw)
  return {
    id: str(r.id),
    name: str(r.name),
    status: runStatus(r.status),
    startedAt: nullableStr(r.startedAt),
    completedAt: nullableStr(r.completedAt),
    error: nullableStr(r.error),
    percent: nullableNum(r.percent)
  }
}

export function parseQualityRun(raw: unknown): QualityRun {
  const r = obj(raw)
  return {
    id: str(r.id),
    worktreeId: str(r.worktreeId),
    profileId: str(r.profileId),
    status: runStatus(r.status),
    startedAt: nullableStr(r.startedAt),
    completedAt: nullableStr(r.completedAt),
    error: nullableStr(r.error),
    percent: nullableNum(r.percent),
    steps: arr(r.steps).map(parseQualityStep),
    triggeredBy: TRIGGERED_BY.has(r.triggeredBy as string)
      ? (r.triggeredBy as 'user' | 'auto' | 'ci')
      : 'unknown'
  }
}

// ---------------------------------------------------------------------------
// §4.7.3 Findings
// ---------------------------------------------------------------------------

function parseQualityWaiver(raw: unknown): QualityWaiver | null {
  if (!raw || typeof raw !== 'object') return null
  const r = raw as Record<string, unknown>
  return {
    id: str(r.id),
    findingId: str(r.findingId),
    reason: str(r.reason),
    waivedBy: nullableStr(r.waivedBy),
    waivedAt: str(r.waivedAt),
    expiresAt: nullableStr(r.expiresAt)
  }
}

export function parseQualityFinding(raw: unknown): QualityFinding {
  const r = obj(raw)
  return {
    id: str(r.id),
    ruleId: str(r.ruleId),
    ruleName: str(r.ruleName),
    severity: findingSeverity(r.severity),
    category: findingCategory(r.category),
    path: str(r.path),
    startLine: safeNum(r.startLine),
    endLine: safeNum(r.endLine),
    startColumn: nullableNum(r.startColumn),
    endColumn: nullableNum(r.endColumn),
    message: str(r.message),
    snippet: nullableStr(r.snippet),
    waived: boolVal(r.waived),
    waiver: parseQualityWaiver(r.waiver),
    effortMinutes: safeNum(r.effortMinutes),
    isNew: boolVal(r.isNew),
    autoFixAvailable: boolVal(r.autoFixAvailable)
  }
}

// ---------------------------------------------------------------------------
// §4.7.4 Profile
// ---------------------------------------------------------------------------

function parseRunnableProfile(raw: unknown): RunnableProfile {
  const r = obj(raw)
  return {
    id: str(r.id),
    name: str(r.name),
    description: nullableStr(r.description),
    isDefault: boolVal(r.isDefault),
    checks: arr(r.checks).map((c) => str(c)),
    estimatedMinutes: nullableNum(r.estimatedMinutes)
  }
}

export function parseQualityProfile(raw: unknown): QualityProfile {
  const r = obj(raw)
  return {
    activeProfileId: nullableStr(r.activeProfileId),
    profiles: arr(r.profiles).map(parseRunnableProfile)
  }
}

// Re-export for 050 compatibility
export { parseRunnableProfile }

// ---------------------------------------------------------------------------
// §4.7.5 Trend
// ---------------------------------------------------------------------------

export function parseQualityTrendPoint(raw: unknown): QualityTrendPoint {
  const r = obj(raw)
  return {
    date: str(r.date),
    score: nullableNum(r.score),
    coverage: nullableNum(r.coverage),
    violations: safeNum(r.violations),
    runId: nullableStr(r.runId)
  }
}

// ---------------------------------------------------------------------------
// §4.7.6 Coverage
// ---------------------------------------------------------------------------

function parseCoverageLine(raw: unknown): CoverageLine {
  const r = obj(raw)
  const coveredRaw = r.covered
  return {
    line: safeNum(r.line),
    covered: coveredRaw === true ? true : coveredRaw === false ? false : null,
    branchCoverage: nullableNum(r.branchCoverage)
  }
}

function parseCoverageFile(raw: unknown): CoverageFile {
  const r = obj(raw)
  return {
    path: str(r.path),
    lineCoverage: nullableNum(r.lineCoverage),
    lines: arr(r.lines).map(parseCoverageLine)
  }
}

export function parseCoverageReport(raw: unknown): CoverageReport {
  const r = obj(raw)
  return {
    worktreeId: str(r.worktreeId),
    headCommit: nullableStr(r.headCommit),
    lineCoverage: nullableNum(r.lineCoverage),
    branchCoverage: nullableNum(r.branchCoverage),
    statementCoverage: nullableNum(r.statementCoverage),
    files: arr(r.files).map(parseCoverageFile),
    ciCoverage: nullableNum(r.ciCoverage),
    generatedAt: nullableStr(r.generatedAt)
  }
}

// ---------------------------------------------------------------------------
// §4.7.7 CiComparison
// ---------------------------------------------------------------------------

export function parseCiComparison(raw: unknown): CiComparison {
  const r = obj(raw)
  return {
    runId: nullableStr(r.runId),
    ciRunId: nullableStr(r.ciRunId),
    verdict: CI_VERDICTS.has(r.verdict as string) ? (r.verdict as 'better' | 'same' | 'worse') : 'unknown',
    scoreDelta: nullableNum(r.scoreDelta),
    coverageDelta: nullableNum(r.coverageDelta),
    newViolations: safeNum(r.newViolations),
    resolvedViolations: safeNum(r.resolvedViolations),
    generatedAt: nullableStr(r.generatedAt)
  }
}
