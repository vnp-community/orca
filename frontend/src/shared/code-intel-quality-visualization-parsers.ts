/**
 * code-intel-quality-visualization-parsers.ts — FE-CV-TASK-087-15
 *
 * Never-throw parsers for trend points and coverage reports. A missing number stays missing
 * (PQ-33): it is never filled with zero.
 *
 * @module shared/code-intel-quality-visualization-parsers
 */

import {
  count,
  finiteOrNull,
  list,
  oneOf,
  optFinite,
  optStr,
  parseGateResult,
  rec,
  str,
  strList
} from './code-intel-quality-parse-primitives'
import type {
  CoverageDiff,
  CoverageFile,
  CoverageReport,
  QualityCoverageResponse,
  QualityTrendMetrics,
  QualityTrendPoint,
  QualityTrendResponse
} from './code-intel-quality-types'

const SOURCES = new Set(['local', 'ci'])
const COVERAGE_SOURCES = new Set(['measured', 'estimated'])
const LANGUAGES = new Set(['go', 'ts', 'mixed'])

// ---------------------------------------------------------------------------
// Trend and coverage
// ---------------------------------------------------------------------------

function parseTrendMetrics(raw: unknown): QualityTrendMetrics {
  const r = rec(raw)
  const out: QualityTrendMetrics = {}
  // Why: a missing key means "no number" (PQ-33); it must never be filled with 0.
  for (const key of ['diffCoverage', 'newLayerViolations', 'newCycles', 'testsFailed'] as const) {
    const v = optFinite(r[key])
    if (v !== undefined) {
      out[key] = v
    }
  }
  return out
}

export function parseQualityTrendPoint(raw: unknown): QualityTrendPoint {
  const r = rec(raw)
  const counts = rec(r.counts)
  const byCategory = Object.fromEntries(
    Object.entries(rec(counts.byCategory))
      .filter((e): e is [string, number] => typeof e[1] === 'number' && Number.isFinite(e[1]))
      .map(([k, v]) => [k, Math.max(0, v)])
  )
  return {
    turnKey: str(r.turnKey),
    headCommit: str(r.headCommit),
    baseCommit: str(r.baseCommit),
    profileRef: str(r.profileRef),
    verdict: parseGateResult(r.verdict),
    counts: {
      error: count(counts.error),
      warning: count(counts.warning),
      info: count(counts.info),
      byCategory
    },
    metrics: parseTrendMetrics(r.metrics),
    runIds: strList(r.runIds),
    indexCommit: str(r.indexCommit),
    source: oneOf(r.source, SOURCES),
    createdAt: str(r.createdAt)
  }
}

export function parseQualityTrendResponse(raw: unknown): QualityTrendResponse {
  const r = rec(raw)
  const points = list(r.points).map(parseQualityTrendPoint)
  return {
    points,
    truncated: r.truncated === true,
    totalCount: Math.max(count(r.totalCount), points.length)
  }
}

function parseCoverageFile(raw: unknown): CoverageFile {
  const r = rec(raw)
  const ranges = list(r.uncoveredRanges)
    .map((x): [number, number] | null =>
      Array.isArray(x) && typeof x[0] === 'number' && typeof x[1] === 'number' ? [x[0], x[1]] : null
    )
    .filter((x): x is [number, number] => x !== null)
  return {
    path: str(r.path),
    stmts: count(r.stmts),
    covered: count(r.covered),
    pct: count(r.pct),
    ...(ranges.length > 0 ? { uncoveredRanges: ranges } : {})
  }
}

function parseCoverageDiff(raw: unknown): CoverageDiff | null {
  if (typeof raw !== 'object' || raw === null || Array.isArray(raw)) {
    return null
  }
  const r = rec(raw)
  return {
    changedExecutable: count(r.changedExecutable),
    covered: count(r.covered),
    uncovered: count(r.uncovered),
    diffCoverage: finiteOrNull(r.diffCoverage),
    ...(optStr(r.reason) !== undefined ? { reason: optStr(r.reason) } : {}),
    partial: r.partial === true,
    excludedFiles: list(r.excludedFiles).map((x) => ({
      path: str(rec(x).path),
      reason: str(rec(x).reason)
    }))
  }
}

export function parseCoverageReport(raw: unknown): CoverageReport | null {
  if (typeof raw !== 'object' || raw === null || Array.isArray(raw)) {
    return null
  }
  const r = rec(raw)
  const totals = rec(r.totals)
  const files = list(r.files).map(parseCoverageFile)
  const toolVersions = Object.fromEntries(
    Object.entries(rec(r.toolVersions)).filter(
      (e): e is [string, string] => typeof e[1] === 'string'
    )
  )
  const totalsOut: CoverageReport['totals'] = {}
  for (const key of [
    'stmts',
    'covered',
    'pct',
    'changedSymbolsTested',
    'changedSymbolsUntested',
    'changedSymbolsUnknown'
  ] as const) {
    const v = optFinite(totals[key])
    if (v !== undefined) {
      totalsOut[key] = v
    }
  }
  return {
    source: oneOf(r.source, COVERAGE_SOURCES),
    language: oneOf(r.language, LANGUAGES),
    ...(optStr(r.mode) !== undefined ? { mode: optStr(r.mode) } : {}),
    headCommit: str(r.headCommit),
    baseCommit: str(r.baseCommit),
    dirty: r.dirty === true,
    totals: totalsOut,
    diff: parseCoverageDiff(r.diff),
    files,
    truncated: r.truncated === true,
    totalCount: Math.max(count(r.totalCount), files.length),
    toolVersions,
    ...(optStr(r.estimatedNote) !== undefined ? { estimatedNote: optStr(r.estimatedNote) } : {})
  }
}

export function parseQualityCoverageResponse(raw: unknown): QualityCoverageResponse {
  const r = rec(raw)
  return {
    report: parseCoverageReport(r.report),
    ...(optStr(r.reason) !== undefined ? { reason: optStr(r.reason) } : {})
  }
}
