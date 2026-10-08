/**
 * code-intel-quality-wire-parsers.ts — FE-CV-TASK-087-01
 *
 * Tolerant parsers for the §4.7 quality wire data. They never throw: enum values outside the
 * known set become 'unknown' (U4), negative/NaN counts become 0, missing arrays become [],
 * and ratios/percent that cannot be read stay null (a missing number is never zero).
 *
 * @module shared/code-intel-quality-wire-parsers
 */

import { parseIndexBasis } from './code-intel-index-status-parser'
import {
  count,
  list,
  oneOf,
  optFinite,
  optStr,
  parseGateResult,
  rec,
  str,
  strList
} from './code-intel-quality-parse-primitives'
import type { IndexBasis } from './code-intel-index-types'
import type {
  CiComparison,
  CiRelation,
  QualityCategory,
  QualityFinding,
  QualityFindingsResponse,
  QualityFindingWaiver,
  QualityGate,
  QualityGateReason,
  QualityGateResponse,
  QualityRun,
  QualityRunScope,
  QualityRunsResponse,
  QualityRunStatus,
  QualityRunSummary,
  QualitySeverity,
  QualityStep,
  QualityStepFailureKind,
  QualityStepStatus,
  QualityWaiver,
  QualityWaiverSubjectKind,
} from './code-intel-quality-types'

// ---------------------------------------------------------------------------
// Enum tables
// ---------------------------------------------------------------------------

const SEVERITIES = new Set(['error', 'warning', 'info'])
const CATEGORIES = new Set([
  'lint', 'typecheck', 'test', 'coverage', 'complexity', 'security', 'dependency', 'convention', 'architecture', 'ai'
])
const STEP_STATUSES = new Set(['passed', 'findings', 'failed', 'timeout', 'cancelled', 'skipped', 'env_not_ready'])
const FAILURE_KINDS = new Set(['', 'format_drift', 'output_too_large', 'exit_unexpected', 'parser_error', 'env'])
const RUN_SCOPES = new Set(['worktree', 'changed', 'commitRange'])
const RUN_STATUSES = new Set(['queued', 'running', 'succeeded', 'failed', 'cancelled', 'interrupted'])
const SOURCES = new Set(['local', 'ci'])
const MODES = new Set(['inform', 'block'])
const SUBJECT_KINDS = new Set(['finding', 'structure_finding', 'check'])
const RELATIONS = new Set([
  'agree_pass', 'agree_fail', 'local_pass_ci_fail', 'local_fail_ci_pass', 'local_only', 'ci_only',
  'ci_pending', 'sha_mismatch', 'not_comparable'
])

// ---------------------------------------------------------------------------
// Findings
// ---------------------------------------------------------------------------

function parseFindingWaiver(raw: unknown): QualityFindingWaiver | undefined {
  const r = rec(raw)
  if (typeof r.by !== 'string' && typeof r.reason !== 'string') {
    return undefined
  }
  return { by: str(r.by), reason: str(r.reason), expiresAt: str(r.expiresAt) }
}

export function parseQualityFinding(raw: unknown): QualityFinding {
  const r = rec(raw)
  const fixHint = optStr(r.fixHint)
  const waiver = parseFindingWaiver(r.waiver)
  const line = typeof r.line === 'number' && Number.isFinite(r.line) ? r.line : 0
  return {
    fingerprint: str(r.fingerprint),
    fpVersion: count(r.fpVersion),
    ruleId: str(r.ruleId),
    severity: oneOf<QualitySeverity>(r.severity, SEVERITIES),
    category: oneOf<QualityCategory>(r.category, CATEGORIES),
    file: str(r.file),
    line,
    endLine: typeof r.endLine === 'number' && Number.isFinite(r.endLine) ? r.endLine : line,
    column: count(r.column),
    endColumn: count(r.endColumn),
    message: str(r.message),
    tool: str(r.tool),
    toolVersion: str(r.toolVersion),
    ...(fixHint !== undefined ? { fixHint } : {}),
    stepId: str(r.stepId),
    inScope: r.inScope === true,
    ...(waiver ? { waiver } : {})
  }
}

// ---------------------------------------------------------------------------
// Run and steps
// ---------------------------------------------------------------------------

export function parseQualityStep(raw: unknown): QualityStep {
  const r = rec(raw)
  const envReason = optStr(r.envReason)
  return {
    id: str(r.id),
    profileId: str(r.profileId),
    status: oneOf<QualityStepStatus>(r.status, STEP_STATUSES),
    failureKind: oneOf<QualityStepFailureKind>(r.failureKind ?? '', FAILURE_KINDS),
    ...(envReason !== undefined ? { envReason } : {}),
    exitCode: typeof r.exitCode === 'number' && Number.isFinite(r.exitCode) ? r.exitCode : 0,
    durationMs: count(r.durationMs),
    tool: str(r.tool),
    toolVersion: str(r.toolVersion),
    errorCount: count(r.errorCount),
    warningCount: count(r.warningCount),
    infoCount: count(r.infoCount),
    totalCount: count(r.totalCount),
    truncated: r.truncated === true,
    outsideScopeCount: count(r.outsideScopeCount)
  }
}

export function parseQualityRunSummary(raw: unknown): QualityRunSummary {
  const r = rec(raw)
  return {
    error: count(r.error),
    warning: count(r.warning),
    info: count(r.info),
    stepsTotal: count(r.stepsTotal),
    stepsWithFindings: count(r.stepsWithFindings),
    stepsFailed: count(r.stepsFailed),
    stepsEnvNotReady: count(r.stepsEnvNotReady),
    outsideScope: count(r.outsideScope),
    truncated: r.truncated === true
  }
}

export function parseQualityRun(raw: unknown): QualityRun {
  const r = rec(raw)
  const ci = rec(r.ci)
  const baseCommit = optStr(r.baseCommit)
  const errorCode = optStr(r.errorCode)
  const treeFingerprint = optStr(r.treeFingerprint)
  return {
    id: str(r.id),
    worktreeId: str(r.worktreeId),
    headCommit: str(r.headCommit),
    indexCommit: str(r.indexCommit),
    indexBasis: list(r.indexBasis)
      .map(parseIndexBasis)
      .filter((b): b is IndexBasis => b !== null),
    scope: oneOf<QualityRunScope>(r.scope, RUN_SCOPES),
    ...(baseCommit !== undefined ? { baseCommit } : {}),
    profile: str(r.profile),
    status: oneOf<QualityRunStatus>(r.status, RUN_STATUSES),
    source: oneOf(r.source, SOURCES),
    startedAt: typeof r.startedAt === 'string' ? r.startedAt : null,
    finishedAt: typeof r.finishedAt === 'string' ? r.finishedAt : null,
    summary: parseQualityRunSummary(r.summary),
    steps: list(r.steps).map(parseQualityStep),
    ...(errorCode !== undefined ? { errorCode } : {}),
    ...(treeFingerprint !== undefined ? { treeFingerprint } : {}),
    ...(typeof r.dirty === 'boolean' ? { dirty: r.dirty } : {}),
    workTreeChangedDuringRun: r.workTreeChangedDuringRun === true,
    scopeWidened: r.scopeWidened === true,
    ...(typeof r.ci === 'object' && r.ci !== null
      ? {
          ci: {
            provider: str(ci.provider),
            headSha: str(ci.headSha),
            ...(optStr(ci.url) !== undefined ? { url: optStr(ci.url) } : {}),
            fetchedAt: str(ci.fetchedAt),
            ...(optStr(ci.staleAfter) !== undefined ? { staleAfter: optStr(ci.staleAfter) } : {})
          }
        }
      : {})
  }
}

// ---------------------------------------------------------------------------
// Gate, waiver, CI comparison
// ---------------------------------------------------------------------------

/** observed/threshold are strings by contract; tolerate numbers from older backends. */
function textOrNumber(v: unknown): string {
  if (typeof v === 'string') {
    return v
  }
  return typeof v === 'number' && Number.isFinite(v) ? String(v) : ''
}

function parseGateReason(raw: unknown): QualityGateReason {
  const r = rec(raw)
  const params = rec(r.params)
  const paramEntries = Object.entries(params).filter(
    (e): e is [string, string] => typeof e[1] === 'string'
  )
  return {
    check: str(r.check),
    observed: textOrNumber(r.observed),
    threshold: textOrNumber(r.threshold),
    result: parseGateResult(r.result),
    ...(optStr(r.code) !== undefined ? { code: optStr(r.code) } : {}),
    ...(paramEntries.length > 0 ? { params: Object.fromEntries(paramEntries) } : {}),
    ...(optStr(r.runId) !== undefined ? { runId: optStr(r.runId) } : {}),
    ...(typeof r.waivedCount === 'number' && r.waivedCount > 0 ? { waivedCount: count(r.waivedCount) } : {}),
    ...(optStr(r.category) !== undefined ? { category: optStr(r.category) } : {}),
    ...(optStr(r.tool) !== undefined ? { tool: optStr(r.tool) } : {})
  }
}

export function parseQualityGate(raw: unknown): QualityGate {
  const r = rec(raw)
  const based = rec(r.basedOn)
  return {
    verdict: parseGateResult(r.verdict),
    reasons: list(r.reasons).map(parseGateReason),
    mode: oneOf(r.mode, MODES),
    profile: str(r.profile),
    basedOn: {
      runIds: strList(based.runIds),
      indexCommit: str(based.indexCommit),
      stale: based.stale === true,
      ...(optStr(based.headCommit) !== undefined ? { headCommit: optStr(based.headCommit) } : {}),
      ...(optStr(based.baseCommit) !== undefined ? { baseCommit: optStr(based.baseCommit) } : {}),
      ...(optStr(based.evaluatedAt) !== undefined ? { evaluatedAt: optStr(based.evaluatedAt) } : {}),
      ...(optFinite(based.profileVersion) !== undefined ? { profileVersion: optFinite(based.profileVersion) } : {})
    }
  }
}

export function parseQualityWaiver(raw: unknown): QualityWaiver {
  const r = rec(raw)
  return {
    id: str(r.id),
    subjectKind: oneOf<QualityWaiverSubjectKind>(r.subjectKind, SUBJECT_KINDS),
    subjectKey: str(r.subjectKey),
    scope: str(r.scope),
    reason: str(r.reason),
    createdBy: str(r.createdBy),
    createdAt: str(r.createdAt),
    expiresAt: str(r.expiresAt),
    ...(optStr(r.revokedAt) !== undefined ? { revokedAt: optStr(r.revokedAt) } : {})
  }
}

export function parseCiComparison(raw: unknown): CiComparison {
  const r = rec(raw)
  const local = rec(r.local)
  const ci = rec(r.ci)
  const hints = strList(r.reasonsHint)
  const pick = (src: Record<string, unknown>, keys: string[]): Record<string, string> =>
    Object.fromEntries(keys.filter((k) => typeof src[k] === 'string').map((k) => [k, src[k] as string]))
  return {
    profile: str(r.profile),
    headCommit: str(r.headCommit),
    local: {
      ...pick(local, ['runId', 'status', 'finishedAt']),
      ...(typeof local.dirty === 'boolean' ? { dirty: local.dirty } : {})
    },
    ci: pick(ci, ['runId', 'status', 'url', 'fetchedAt', 'sha']),
    relation: oneOf<CiRelation>(r.relation, RELATIONS),
    ...(hints.length > 0 ? { reasonsHint: hints } : {})
  }
}

export function parseQualityGateResponse(raw: unknown): QualityGateResponse {
  const r = rec(raw)
  return {
    gate: parseQualityGate(r.gate),
    waivers: list(r.waivers).map(parseQualityWaiver),
    evaluatedAt: str(r.evaluatedAt),
    profileDefinitionDigest: str(r.profileDefinitionDigest),
    comparison: list(r.comparison).map(parseCiComparison)
  }
}

// ---------------------------------------------------------------------------
// Responses of list-like channels
// ---------------------------------------------------------------------------

export function parseQualityRunsResponse(raw: unknown): QualityRunsResponse {
  const r = rec(raw)
  const next = optStr(r.nextPageToken)
  return { runs: list(r.runs).map(parseQualityRun), ...(next !== undefined ? { nextPageToken: next } : {}) }
}

export function parseQualityFindingsResponse(raw: unknown): QualityFindingsResponse {
  const r = rec(raw)
  const findings = list(r.findings).map(parseQualityFinding)
  const next = optStr(r.nextPageToken)
  return {
    findings,
    totalCount: Math.max(count(r.totalCount), 0),
    truncated: r.truncated === true,
    outsideScopeCount: count(r.outsideScopeCount),
    ...(next !== undefined ? { nextPageToken: next } : {})
  }
}

export * from './code-intel-quality-profile-parsers'
export * from './code-intel-quality-visualization-parsers'
export { parseGateResult } from './code-intel-quality-parse-primitives'
